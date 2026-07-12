package eveses

import (
	"context"
	"net/url"
	"strconv"
)

// EmailDomain is a single email domain entry returned by Emails.Pricing.
type EmailDomain struct {
	Domain   string `json:"domain"`
	Site     string `json:"site,omitempty"`
	Provider string `json:"provider,omitempty"`
	Price    any    `json:"price,omitempty"`
}

// EmailQuote is a pre-purchase price estimate returned by Emails.Quote.
type EmailQuote struct {
	Domain     string         `json:"domain"`
	Site       string         `json:"site,omitempty"`
	Provider   string         `json:"provider,omitempty"`
	PriceCents int            `json:"price_cents"`
	Currency   string         `json:"currency,omitempty"`
	Raw        map[string]any `json:"-"`
}

// EmailMessage is a single inbound message on a rented email.
type EmailMessage struct {
	ID         string `json:"id"`
	From       string `json:"from,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Body       string `json:"body,omitempty"`
	ReceivedAt string `json:"received_at,omitempty"`
	Read       bool   `json:"read"`
}

// EmailOrder is a rented email mailbox returned by Emails.Buy / Get / List.
type EmailOrder struct {
	UUID      string `json:"uuid"`
	Email     string `json:"email,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Site      string `json:"site,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Status    string `json:"status"`
	Released  bool   `json:"released"`
	Price     int    `json:"price_cents,omitempty"`
	Currency  string `json:"currency,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

const emailsBase = "/api/v1/emails"

// EmailsService wraps /api/v1/emails.
//
// It covers pricing/domain discovery, quoting, purchasing, listing, reading
// individual mailboxes and their messages, marking messages read, and
// releasing (deleting) a mailbox.
type EmailsService struct {
	client *Client
}

// Pricing returns available email domains and prices (domains under the
// `domains` key). Replaces the old `domains` verb. site is optional; it is
// omitted from the query when empty.
func (s *EmailsService) Pricing(ctx context.Context, site string) (map[string]any, error) {
	q := url.Values{}
	if site != "" {
		q.Set("site", site)
	}
	return s.getMap(ctx, emailsBase+"/pricing", q)
}

// Quote estimates the cost of renting an email address before committing.
func (s *EmailsService) Quote(ctx context.Context, domain, site, provider string) (*EmailQuote, error) {
	q := url.Values{}
	q.Set("domain", domain)
	q.Set("site", site)
	q.Set("provider", provider)

	raw, err := s.getMap(ctx, emailsBase+"/quote", q)
	if err != nil {
		return nil, err
	}
	quote := &EmailQuote{Raw: raw}
	if v, ok := raw["domain"]; ok {
		if str, ok := v.(string); ok {
			quote.Domain = str
		}
	}
	if v, ok := raw["site"]; ok {
		if str, ok := v.(string); ok {
			quote.Site = str
		}
	}
	if v, ok := raw["provider"]; ok {
		if str, ok := v.(string); ok {
			quote.Provider = str
		}
	}
	if v, ok := raw["price_cents"]; ok {
		if n, ok := toInt(v); ok {
			quote.PriceCents = n
		}
	}
	if v, ok := raw["currency"]; ok {
		if str, ok := v.(string); ok {
			quote.Currency = str
		}
	}
	return quote, nil
}

// Buy rents an email address via POST /api/v1/emails/orders. idempotencyKey,
// when non-empty, is forwarded as an Idempotency-Key header so replays return
// the same order.
func (s *EmailsService) Buy(ctx context.Context, domain, site, provider, idempotencyKey string) (*EmailOrder, error) {
	body := map[string]any{
		"domain":   domain,
		"site":     site,
		"provider": provider,
	}

	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}

	var order EmailOrder
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    emailsBase + "/orders",
		body:    body,
		headers: headers,
	}, &order); err != nil {
		return nil, err
	}
	if order.Currency == "" {
		order.Currency = "USD"
	}
	return &order, nil
}

// List returns the user's rented email orders via GET /api/v1/emails/orders.
// Set includeReleased to true to also return released (expired) mailboxes.
func (s *EmailsService) List(ctx context.Context, includeReleased bool) ([]EmailOrder, error) {
	q := url.Values{}
	if includeReleased {
		q.Set("include_released", "1")
	}

	var out []EmailOrder
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   emailsBase + "/orders",
		query:  q,
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []EmailOrder{}
	}
	return out, nil
}

// Get returns a single rented email mailbox, keyed on its email address
// via GET /api/v1/emails/{email}.
func (s *EmailsService) Get(ctx context.Context, email string) (*EmailOrder, error) {
	var order EmailOrder
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   emailsBase + "/" + url.PathEscape(email),
	}, &order); err != nil {
		return nil, err
	}
	if order.Currency == "" {
		order.Currency = "USD"
	}
	return &order, nil
}

// Messages returns the paginated inbox for a rented email mailbox, keyed on
// its email address via GET /api/v1/emails/{email}/messages. page and perPage
// are optional (pass 0 to use API defaults).
func (s *EmailsService) Messages(ctx context.Context, email string, page, perPage int) ([]EmailMessage, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		q.Set("per_page", strconv.Itoa(perPage))
	}

	var out []EmailMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   emailsBase + "/" + url.PathEscape(email) + "/messages",
		query:  q,
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []EmailMessage{}
	}
	return out, nil
}

// MarkRead marks a specific message as read on a rented email mailbox.
func (s *EmailsService) MarkRead(ctx context.Context, email, messageID string) error {
	return s.client.do(ctx, requestOptions{
		method: "POST",
		path:   emailsBase + "/" + url.PathEscape(email) + "/messages/" + url.PathEscape(messageID) + "/read",
	}, nil)
}

// Release deletes / releases a rented email mailbox. The mailbox is
// deactivated and can no longer receive messages.
func (s *EmailsService) Release(ctx context.Context, email string) error {
	return s.client.do(ctx, requestOptions{
		method: "DELETE",
		path:   emailsBase + "/" + url.PathEscape(email),
	}, nil)
}

// getMap issues a GET and returns the decoded JSON object as a map.
func (s *EmailsService) getMap(ctx context.Context, path string, query url.Values) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   path,
		query:  query,
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
