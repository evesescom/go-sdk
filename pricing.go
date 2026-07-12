package eveses

import (
	"context"
	"encoding/json"
)

// PricingResponse is the response of Pricing.All — every product's prices in
// one call (GET /api/v1/pricing). The per-service blocks vary in shape, so
// each is kept as a raw map; Raw retains the whole payload for forward-compat.
type PricingResponse struct {
	Currency     string         `json:"currency,omitempty"`
	Numbers      map[string]any `json:"numbers,omitempty"`
	Proxy        map[string]any `json:"proxy,omitempty"`
	WebUnblocker map[string]any `json:"webunblocker,omitempty"`
	Emails       map[string]any `json:"emails,omitempty"`
	Captcha      map[string]any `json:"captcha,omitempty"`
	Raw          map[string]any `json:"-"`
}

// PricingService wraps /api/v1/pricing — the aggregate price list.
type PricingService struct {
	client *Client
}

// All returns every product's prices in one call.
func (s *PricingService) All(ctx context.Context) (*PricingResponse, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/pricing",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var resp PricingResponse
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &resp); err != nil {
			return nil, &Error{Message: "decode pricing: " + err.Error()}
		}
		var rawMap map[string]any
		if err := json.Unmarshal(inner, &rawMap); err == nil {
			resp.Raw = rawMap
		}
	}
	return &resp, nil
}
