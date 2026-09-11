package eveses

import (
	"context"
	"encoding/json"
	"net/url"
)

// BillingProfile is who we bill: the legal entity, its registered address and
// its tax number. Every document is issued against a frozen copy of this, so
// editing it later never rewrites an invoice already in somebody's books.
type BillingProfile struct {
	Kind             string `json:"kind,omitempty"`
	LegalName        string `json:"legal_name,omitempty"`
	Country          string `json:"country,omitempty"`
	AddressLine1     string `json:"address_line1,omitempty"`
	AddressLine2     string `json:"address_line2,omitempty"`
	City             string `json:"city,omitempty"`
	PostalCode       string `json:"postal_code,omitempty"`
	TaxID            string `json:"tax_id,omitempty"`
	VATNumber        string `json:"vat_number,omitempty"`
	EmailForInvoices string `json:"email_for_invoices,omitempty"`
}

// InvoiceLine is one billed item.
type InvoiceLine struct {
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
	AmountCents    int    `json:"amount_cents"`
}

// Invoice is a document a customer pays, or a receipt for one already paid.
type Invoice struct {
	UUID        string `json:"uuid"`
	Number      string `json:"number"`
	Status      string `json:"status"`
	AmountCents int    `json:"amount_cents"`
	TotalCents  int    `json:"total_cents"`
	Currency    string `json:"currency"`
	IssuedAt    string `json:"issued_at,omitempty"`
	// Empty on a receipt: nothing is due on money already received.
	DueAt            string            `json:"due_at,omitempty"`
	PaidAt           string            `json:"paid_at,omitempty"`
	PaymentReference string            `json:"payment_reference,omitempty"`
	BillTo           map[string]any    `json:"bill_to,omitempty"`
	Seller           map[string]any    `json:"seller,omitempty"`
	Lines            []InvoiceLine     `json:"lines,omitempty"`
	Payment          map[string]string `json:"payment,omitempty"`
}

// IsPaid reports whether the document records money already received.
func (i *Invoice) IsPaid() bool { return i.Status == "paid" }

// BillingService wraps /api/v1/billing.
//
// Two documents share the word "invoice" and read in opposite directions, so
// they are two methods rather than one with a flag:
//
//   - IssueInvoice asks the customer to pay. It carries a payment reference
//     and a due date.
//   - InvoiceForDeposit records a top-up that already settled. It comes back
//     marked paid, with no due date and nothing to act on.
//
// Handing somebody the first when they wanted the second reads as an attempt
// to charge them twice.
type BillingService struct {
	client *Client
}

// Profile returns the invoicing details, or nil when none have been set.
func (s *BillingService) Profile(ctx context.Context) (*BillingProfile, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/billing/profile",
	}, &raw); err != nil {
		return nil, err
	}

	var p BillingProfile
	if inner := unwrapData(raw); len(inner) > 0 {
		if err := json.Unmarshal(inner, &p); err != nil {
			return nil, &Error{Message: "decode billing profile: " + err.Error()}
		}
	}
	if p.LegalName == "" {
		return nil, nil
	}
	return &p, nil
}

// SaveProfile sets or updates the invoicing details. Do it once.
//
// Only non-empty fields are sent, so correcting a postcode does not blank a
// tax number that was simply left unset on the struct.
func (s *BillingService) SaveProfile(ctx context.Context, profile BillingProfile) (*BillingProfile, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "PUT",
		path:   "/api/v1/billing/profile",
		body:   profile,
	}, &raw); err != nil {
		return nil, err
	}

	var out BillingProfile
	if inner := unwrapData(raw); len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode billing profile: " + err.Error()}
		}
	}
	return &out, nil
}

// Invoices lists invoices issued to this account, newest first.
func (s *BillingService) Invoices(ctx context.Context) ([]Invoice, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/billing/invoices",
	}, &raw); err != nil {
		return nil, err
	}

	var wrapper struct {
		Invoices []Invoice `json:"invoices"`
	}
	if inner := unwrapData(raw); len(inner) > 0 {
		if err := json.Unmarshal(inner, &wrapper); err != nil {
			return nil, &Error{Message: "decode invoices: " + err.Error()}
		}
	}
	return wrapper.Invoices, nil
}

// Invoice fetches one invoice in full, including the frozen seller and bill-to.
func (s *BillingService) Invoice(ctx context.Context, uuid string) (*Invoice, error) {
	return s.decodeInvoice(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/billing/invoices/" + url.PathEscape(uuid),
	})
}

// IssueInvoice raises an invoice to PAY, for money about to be sent.
// Accepted range is $10 to $10,000, in cents.
//
// You get back a payment reference; quote it when paying and the amount lands
// on the balance.
func (s *BillingService) IssueInvoice(ctx context.Context, amountCents int, currency, note string) (*Invoice, error) {
	if currency == "" {
		currency = "USD"
	}
	body := map[string]any{
		"amount_cents": amountCents,
		"currency":     currency,
		"purpose":      "wallet_topup",
	}
	if note != "" {
		body["note"] = note
	}

	return s.decodeInvoice(ctx, requestOptions{
		method: "POST",
		path:   "/api/v1/billing/invoices",
		body:   body,
	})
}

// InvoiceForDeposit issues a receipt for a top-up that already settled.
//
// Comes back marked paid, with no due date. Shows what actually left the
// customer's account — a $10.00 top-up charged at $10.20 is invoiced as $10.20
// with the card fee itemised, so it reconciles against a card statement rather
// than against the balance.
//
// Idempotent: asking twice returns the same document, not a second invoice
// number for one payment.
func (s *BillingService) InvoiceForDeposit(ctx context.Context, depositUUID string) (*Invoice, error) {
	return s.decodeInvoice(ctx, requestOptions{
		method: "POST",
		path:   "/api/v1/billing/deposits/" + url.PathEscape(depositUUID) + "/invoice",
	})
}

// CancelInvoice cancels an unpaid invoice.
//
// The number stays with it — freeing one for reuse is the gap an audit asks
// about.
func (s *BillingService) CancelInvoice(ctx context.Context, uuid string) (*Invoice, error) {
	return s.decodeInvoice(ctx, requestOptions{
		method: "POST",
		path:   "/api/v1/billing/invoices/" + url.PathEscape(uuid) + "/cancel",
	})
}

// InvoicePDF returns the invoice as PDF bytes, ready to write to a file.
func (s *BillingService) InvoicePDF(ctx context.Context, uuid string) ([]byte, error) {
	var out []byte
	if err := s.client.do(ctx, requestOptions{
		method:  "GET",
		path:    "/api/v1/billing/invoices/" + url.PathEscape(uuid) + "/pdf",
		headers: map[string]string{"Accept": "application/pdf"},
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *BillingService) decodeInvoice(ctx context.Context, opts requestOptions) (*Invoice, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, opts, &raw); err != nil {
		return nil, err
	}

	var inv Invoice
	if inner := unwrapData(raw); len(inner) > 0 {
		if err := json.Unmarshal(inner, &inv); err != nil {
			return nil, &Error{Message: "decode invoice: " + err.Error()}
		}
	}
	if inv.Currency == "" {
		inv.Currency = "USD"
	}
	if inv.PaymentReference == "" {
		inv.PaymentReference = inv.Payment["reference"]
	}
	if inv.TotalCents == 0 {
		inv.TotalCents = inv.AmountCents
	}
	return &inv, nil
}
