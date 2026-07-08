package eveses

import (
	"context"
	"encoding/json"
	"net/url"
)

// EmailAddress is a rented inbox address / order.
type EmailAddress struct {
	UUID         string         `json:"uuid"`
	Address      string         `json:"address"`
	Domain       string         `json:"domain"`
	Site         string         `json:"site,omitempty"`
	Status       string         `json:"status"`
	PriceCents   int            `json:"price_cents"`
	Currency     string         `json:"currency"`
	MessageCount int            `json:"message_count"`
	ExpiresAt    string         `json:"expires_at,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
	Raw          map[string]any `json:"-"`
}

// EmailMessage is a single received message. Body may be plain text or HTML;
// there is no id or read flag.
type EmailMessage struct {
	From       string `json:"from"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	ReceivedAt string `json:"received_at,omitempty"`
}

// EmailOrder is an inbox address together with its received messages, as
// returned by EmailsService.Get. It embeds EmailAddress.
type EmailOrder struct {
	EmailAddress
	Messages []EmailMessage `json:"messages"`
}

// EmailDomain is a rentable domain with its user price and availability.
type EmailDomain struct {
	Provider   string `json:"provider"`
	Domain     string `json:"domain"`
	PriceCents int    `json:"price_cents"`
	Available  bool   `json:"available"`
}

// EmailDomainsResponse is the response of EmailsService.Domains.
type EmailDomainsResponse struct {
	Domains  []EmailDomain `json:"domains"`
	Currency string        `json:"currency"`
}

// EmailQuote is the response of EmailsService.Quote.
type EmailQuote struct {
	Domain     string `json:"domain"`
	Provider   string `json:"provider,omitempty"`
	PriceCents int    `json:"price_cents"`
	Currency   string `json:"currency"`
}

// EmailsService wraps /api/account/emails.
type EmailsService struct {
	client *Client
}

// List returns the user's rented email addresses.
func (s *EmailsService) List(ctx context.Context) ([]EmailAddress, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/emails",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out struct {
		Emails []EmailAddress `json:"emails"`
	}
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode emails: " + err.Error()}
		}
	}
	if out.Emails == nil {
		out.Emails = []EmailAddress{}
	}
	for i := range out.Emails {
		if out.Emails[i].Currency == "" {
			out.Emails[i].Currency = "USD"
		}
	}
	return out.Emails, nil
}

// Domains lists rentable domains with user prices. site scopes reseller
// providers; pass "" for our own catch-all domains.
func (s *EmailsService) Domains(ctx context.Context, site string) (*EmailDomainsResponse, error) {
	q := url.Values{}
	if site != "" {
		q.Set("site", site)
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/emails/domains",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out EmailDomainsResponse
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode email domains: " + err.Error()}
		}
	}
	if out.Domains == nil {
		out.Domains = []EmailDomain{}
	}
	out.Currency = defaultCurrency(out.Currency)
	return &out, nil
}

// EmailQuoteParams configures EmailsService.Quote. Domain is required; Site and
// Provider are optional (needed for reseller providers).
type EmailQuoteParams struct {
	Domain   string
	Site     string
	Provider string
}

// Quote prices a concrete domain pick before renting.
func (s *EmailsService) Quote(ctx context.Context, p *EmailQuoteParams) (*EmailQuote, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	if p.Domain == "" {
		return nil, &Error{Message: "Domain is required"}
	}
	q := url.Values{}
	q.Set("domain", p.Domain)
	if p.Site != "" {
		q.Set("site", p.Site)
	}
	if p.Provider != "" {
		q.Set("provider", p.Provider)
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/emails/quote",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var quote EmailQuote
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &quote); err != nil {
			return nil, &Error{Message: "decode email quote: " + err.Error()}
		}
	}
	quote.Currency = defaultCurrency(quote.Currency)
	return &quote, nil
}

// PurchaseEmailParams configures EmailsService.Purchase. Domain is required;
// Site and Provider are optional. When IdempotencyKey is set it is sent as the
// Idempotency-Key HTTP header.
type PurchaseEmailParams struct {
	Domain         string
	Site           string
	Provider       string
	IdempotencyKey string
}

// Purchase rents an inbox address and returns the created address (HTTP 201).
func (s *EmailsService) Purchase(ctx context.Context, p *PurchaseEmailParams) (*EmailAddress, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	if p.Domain == "" {
		return nil, &Error{Message: "Domain is required"}
	}

	body := map[string]any{"domain": p.Domain}
	if p.Site != "" {
		body["site"] = p.Site
	}
	if p.Provider != "" {
		body["provider"] = p.Provider
	}

	headers := map[string]string{}
	if p.IdempotencyKey != "" {
		headers["Idempotency-Key"] = p.IdempotencyKey
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    "/api/account/emails/purchase",
		body:    body,
		headers: headers,
	}, &raw); err != nil {
		return nil, err
	}
	return decodeEmailAddress(raw)
}

// Get fetches one address and its received messages. This call also live-syncs
// reseller inboxes from the provider, so it is the inbox refresh mechanism —
// poll it to check for new mail.
func (s *EmailsService) Get(ctx context.Context, uuid string) (*EmailOrder, error) {
	if uuid == "" {
		return nil, &Error{Message: "uuid is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/emails/" + url.PathEscape(uuid),
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var order EmailOrder
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &order); err != nil {
			return nil, &Error{Message: "decode email order: " + err.Error()}
		}
	}
	if order.Currency == "" {
		order.Currency = "USD"
	}
	if order.Messages == nil {
		order.Messages = []EmailMessage{}
	}
	var rawMap map[string]any
	if err := json.Unmarshal(inner, &rawMap); err == nil {
		order.Raw = rawMap
	}
	return &order, nil
}

// Delete releases an address (soft cancel, no refund). The returned address
// has Status = "cancelled".
func (s *EmailsService) Delete(ctx context.Context, uuid string) (*EmailAddress, error) {
	if uuid == "" {
		return nil, &Error{Message: "uuid is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "DELETE",
		path:   "/api/account/emails/" + url.PathEscape(uuid),
	}, &raw); err != nil {
		return nil, err
	}
	return decodeEmailAddress(raw)
}

func decodeEmailAddress(raw json.RawMessage) (*EmailAddress, error) {
	inner := unwrapData(raw)
	if len(inner) == 0 {
		return &EmailAddress{Currency: "USD"}, nil
	}
	var addr EmailAddress
	if err := json.Unmarshal(inner, &addr); err != nil {
		return nil, &Error{Message: "decode email address: " + err.Error()}
	}
	if addr.Currency == "" {
		addr.Currency = "USD"
	}
	var rawMap map[string]any
	if err := json.Unmarshal(inner, &rawMap); err == nil {
		addr.Raw = rawMap
	}
	return &addr, nil
}
