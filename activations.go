package eveses

import (
	"context"
	"encoding/json"
	"net/url"
)

// OrderMode is "activation" (default) or "rent".
type OrderMode string

const (
	OrderModeActivation OrderMode = "activation"
	OrderModeRent       OrderMode = "rent"
)

// Order is the on-the-wire shape of a single order returned by
// /api/account/orders/*. Snake_case wire fields are translated to idiomatic
// Go names; Raw retains the original JSON for forward-compat.
type Order struct {
	OrderID    string         `json:"order_id"`
	Status     string         `json:"status"`
	Phone      string         `json:"phone,omitempty"`
	Country    string         `json:"country,omitempty"`
	Service    string         `json:"service,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	PriceCents *int           `json:"price_cents,omitempty"`
	ExpiresAt  string         `json:"expires_at,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	Raw        map[string]any `json:"-"`
}

// OrderSms is a single SMS message attached to an order.
type OrderSms struct {
	ID         int    `json:"id"`
	Text       string `json:"text"`
	Sender     string `json:"sender,omitempty"`
	ReceivedAt string `json:"received_at,omitempty"`
}

// OrderSmsBundle combines the webhook-stored and on-demand-fresh SMS lists
// for an order.
type OrderSmsBundle struct {
	OrderID string     `json:"order_id"`
	Stored  []OrderSms `json:"stored"`
	Fresh   []OrderSms `json:"fresh"`
}

// CreateActivationParams is the input to ActivationsService.Create.
//
// Country and Service are required. When IdempotencyKey is set, it is sent
// both in the JSON body (for legacy server-side dedup) and as the
// `Idempotency-Key` HTTP header.
type CreateActivationParams struct {
	// Country is an ISO 3166-1 alpha-2 code, lowercased ("ua", "pl"). Required.
	Country string
	// Service is the upstream service code ("telegram", "wa"). Required.
	Service string
	// Mode defaults to "activation" when empty.
	Mode OrderMode
	// DurationMinutes is required for rent mode (>= 1).
	DurationMinutes *int
	// IdempotencyKey is sent both in body and as the Idempotency-Key header.
	IdempotencyKey string
	// MaxPriceCents bounds the highest acceptable price in cents.
	MaxPriceCents *int
}

// ActivationsService wraps /api/account/orders/*.
type ActivationsService struct {
	client *Client
}

// Create provisions a number for a country/service. Returns the created order.
func (s *ActivationsService) Create(ctx context.Context, p *CreateActivationParams) (*Order, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	if p.Country == "" {
		return nil, &Error{Message: "Country is required"}
	}
	if p.Service == "" {
		return nil, &Error{Message: "Service is required"}
	}

	mode := string(p.Mode)
	if mode == "" {
		mode = string(OrderModeActivation)
	}

	body := map[string]any{
		"mode":    mode,
		"country": p.Country,
		"service": p.Service,
	}
	if p.DurationMinutes != nil {
		body["duration_minutes"] = *p.DurationMinutes
	}
	if p.IdempotencyKey != "" {
		body["idempotency_key"] = p.IdempotencyKey
	}
	if p.MaxPriceCents != nil {
		body["max_price_cents"] = *p.MaxPriceCents
	}

	headers := map[string]string{}
	if p.IdempotencyKey != "" {
		headers["Idempotency-Key"] = p.IdempotencyKey
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    "/api/account/orders",
		body:    body,
		headers: headers,
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// Get fetches an order by its UUID.
func (s *ActivationsService) Get(ctx context.Context, orderID string) (*Order, error) {
	if orderID == "" {
		return nil, &Error{Message: "orderID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/orders/" + url.PathEscape(orderID),
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// Cancel releases the number and refunds the user (where supported).
func (s *ActivationsService) Cancel(ctx context.Context, orderID string) (*Order, error) {
	if orderID == "" {
		return nil, &Error{Message: "orderID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/account/orders/" + url.PathEscape(orderID) + "/cancel",
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// Finish marks the order completed once the SMS has been consumed.
func (s *ActivationsService) Finish(ctx context.Context, orderID string) (*Order, error) {
	if orderID == "" {
		return nil, &Error{Message: "orderID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/account/orders/" + url.PathEscape(orderID) + "/finish",
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// Sms returns all SMS messages for an order, combining `stored` (delivered
// to us via webhook) with `fresh` (pulled from the upstream provider on demand).
func (s *ActivationsService) Sms(ctx context.Context, orderID string) (*OrderSmsBundle, error) {
	if orderID == "" {
		return nil, &Error{Message: "orderID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/orders/" + url.PathEscape(orderID) + "/sms",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var bundle OrderSmsBundle
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &bundle); err != nil {
			return nil, &Error{Message: "decode sms bundle: " + err.Error()}
		}
	}
	if bundle.OrderID == "" {
		bundle.OrderID = orderID
	}
	if bundle.Stored == nil {
		bundle.Stored = []OrderSms{}
	}
	if bundle.Fresh == nil {
		bundle.Fresh = []OrderSms{}
	}
	return &bundle, nil
}

// decodeOrder unwraps {"data": {...}} envelopes and decodes the order. The
// raw JSON is also retained on Order.Raw for callers that need fields the
// SDK doesn't model yet.
func decodeOrder(raw json.RawMessage) (*Order, error) {
	inner := unwrapData(raw)
	if len(inner) == 0 {
		return &Order{}, nil
	}

	var order Order
	if err := json.Unmarshal(inner, &order); err != nil {
		return nil, &Error{Message: "decode order: " + err.Error()}
	}

	var rawMap map[string]any
	if err := json.Unmarshal(inner, &rawMap); err == nil {
		order.Raw = rawMap
	}
	return &order, nil
}
