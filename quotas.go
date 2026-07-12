package eveses

import (
	"context"
	"encoding/json"
)

// Quota is a single remaining-balance entry inside QuotasResponse. Not every
// field is populated for every product — trial rows carry service/total/used/
// expires_at, proxy/webunblocker rows carry provider.
type Quota struct {
	Service   string   `json:"service,omitempty"`
	Provider  string   `json:"provider,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	Remaining float64  `json:"remaining"`
	Total     *float64 `json:"total,omitempty"`
	Used      *float64 `json:"used,omitempty"`
	ExpiresAt string   `json:"expires_at,omitempty"`
}

// QuotasResponse is the response of Quotas.Get — remaining prepaid balances.
// A key is omitted (nil slice) when the user has no quota for that product.
type QuotasResponse struct {
	Trial        []Quota `json:"trial,omitempty"`
	Proxy        []Quota `json:"proxy,omitempty"`
	WebUnblocker []Quota `json:"webunblocker,omitempty"`
}

// QuotasService wraps /api/v1/quotas — remaining prepaid balances. Only
// products with a decrementing counter appear. Numbers/emails/captcha never
// have quotas.
type QuotasService struct {
	client *Client
}

// Get returns the account's remaining prepaid balances.
func (s *QuotasService) Get(ctx context.Context) (*QuotasResponse, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/quotas",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var resp QuotasResponse
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &resp); err != nil {
			return nil, &Error{Message: "decode quotas: " + err.Error()}
		}
	}
	return &resp, nil
}
