package eveses

import (
	"context"
	"net/url"
	"strconv"
)

// WebUnblockerPackage describes a Web Unblocker request-count package returned
// by WebUnblocker.Pricing.
type WebUnblockerPackage struct {
	Requests     int    `json:"requests"`
	PriceCents   int    `json:"price_cents"`
	Currency     string `json:"currency,omitempty"`
	Subscription bool   `json:"subscription"`
}

// WebUnblockerQuote is a pre-purchase price estimate returned by
// WebUnblocker.Quote.
type WebUnblockerQuote struct {
	Requests     int            `json:"requests"`
	Subscription bool           `json:"subscription"`
	PriceCents   int            `json:"price_cents"`
	Currency     string         `json:"currency,omitempty"`
	Raw          map[string]any `json:"-"`
}

// WebUnblockerSubscription is the subscription block returned inside the
// access response.
type WebUnblockerSubscription struct {
	Status       string `json:"status"`
	Requests     int    `json:"requests"`
	DiscountPct  int    `json:"discount_pct"`
	NextRenewsAt string `json:"next_renews_at,omitempty"`
}

// WebUnblockerOrder is a single Web Unblocker purchase order.
type WebUnblockerOrder struct {
	UUID         string `json:"uuid"`
	Requests     int    `json:"requests"`
	Used         int    `json:"used"`
	Status       string `json:"status"`
	PriceCents   int    `json:"price_cents"`
	Currency     string `json:"currency,omitempty"`
	Subscription bool   `json:"subscription"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// WebUnblockerAccess is the response of WebUnblocker.Access — the user's
// credentials, quota, subscription, and order history.
type WebUnblockerAccess struct {
	Host         string                    `json:"host,omitempty"`
	Port         int                       `json:"port,omitempty"`
	Username     string                    `json:"username,omitempty"`
	Password     string                    `json:"password,omitempty"`
	RequestsUsed int                       `json:"requests_used"`
	RequestsLeft int                       `json:"requests_left"`
	Subscription *WebUnblockerSubscription `json:"subscription,omitempty"`
	Orders       []WebUnblockerOrder       `json:"orders"`
}

const webUnblockerBase = "/api/v1/webunblocker"

// WebUnblockerService wraps /api/v1/webunblocker.
//
// It covers pricing, quoting, purchasing, trial, access/quota checks, and
// subscription lifecycle management (cancel / pause / resume).
type WebUnblockerService struct {
	client *Client
}

// Pricing returns the available Web Unblocker request-count packages.
// Replaces the old `packages` verb.
func (s *WebUnblockerService) Pricing(ctx context.Context) (map[string]any, error) {
	return s.getMap(ctx, webUnblockerBase+"/pricing", nil)
}

// Quote estimates the cost of a Web Unblocker purchase before committing.
// Set subscription to true for the recurring-subscription price.
func (s *WebUnblockerService) Quote(ctx context.Context, requests int, subscription bool) (*WebUnblockerQuote, error) {
	q := url.Values{}
	q.Set("requests", strconv.Itoa(requests))
	if subscription {
		q.Set("subscription", "1")
	}

	raw, err := s.getMap(ctx, webUnblockerBase+"/quote", q)
	if err != nil {
		return nil, err
	}
	quote := &WebUnblockerQuote{Raw: raw}
	// Populate typed fields from the raw map so callers don't have to type-assert.
	if v, ok := raw["requests"]; ok {
		if n, ok := toInt(v); ok {
			quote.Requests = n
		}
	}
	if v, ok := raw["subscription"]; ok {
		if b, ok := v.(bool); ok {
			quote.Subscription = b
		}
	}
	if v, ok := raw["price_cents"]; ok {
		if n, ok := toInt(v); ok {
			quote.PriceCents = n
		}
	}
	if v, ok := raw["currency"]; ok {
		if s, ok := v.(string); ok {
			quote.Currency = s
		}
	}
	return quote, nil
}

// Buy buys a Web Unblocker allocation via POST /api/v1/webunblocker/orders.
// Set subscription to start a recurring auto-renewing subscription.
// idempotencyKey, when non-empty, is forwarded as an Idempotency-Key header so
// replays return the same order.
func (s *WebUnblockerService) Buy(ctx context.Context, requests int, subscription bool, idempotencyKey string) (*WebUnblockerOrder, error) {
	body := map[string]any{
		"requests":     requests,
		"subscription": subscription,
	}

	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}

	var order WebUnblockerOrder
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    webUnblockerBase + "/orders",
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

// List returns the user's Web Unblocker orders via
// GET /api/v1/webunblocker/orders.
func (s *WebUnblockerService) List(ctx context.Context) ([]WebUnblockerOrder, error) {
	var out []WebUnblockerOrder
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   webUnblockerBase + "/orders",
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []WebUnblockerOrder{}
	}
	return out, nil
}

// Trial activates the free Web Unblocker trial allocation (one-time).
func (s *WebUnblockerService) Trial(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   webUnblockerBase + "/trial",
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// Access returns the user's Web Unblocker credentials, quota usage,
// subscription status, and order list.
func (s *WebUnblockerService) Access(ctx context.Context) (*WebUnblockerAccess, error) {
	var out WebUnblockerAccess
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   webUnblockerBase,
	}, &out); err != nil {
		return nil, err
	}
	if out.Orders == nil {
		out.Orders = []WebUnblockerOrder{}
	}
	return &out, nil
}

// SubscriptionCancel stops the Web Unblocker subscription's auto-renewal.
func (s *WebUnblockerService) SubscriptionCancel(ctx context.Context) (map[string]any, error) {
	return s.subscriptionAction(ctx, "cancel")
}

// SubscriptionPause skips Web Unblocker renewals until resumed.
func (s *WebUnblockerService) SubscriptionPause(ctx context.Context) (map[string]any, error) {
	return s.subscriptionAction(ctx, "pause")
}

// SubscriptionResume resumes the Web Unblocker subscription.
func (s *WebUnblockerService) SubscriptionResume(ctx context.Context) (map[string]any, error) {
	return s.subscriptionAction(ctx, "resume")
}

func (s *WebUnblockerService) subscriptionAction(ctx context.Context, action string) (map[string]any, error) {
	return s.postMap(ctx, webUnblockerBase+"/subscription/"+action, nil)
}

// getMap issues a GET and returns the decoded JSON object as a map.
func (s *WebUnblockerService) getMap(ctx context.Context, path string, query url.Values) (map[string]any, error) {
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

// postMap issues a POST with an optional body and returns the decoded JSON
// object as a map.
func (s *WebUnblockerService) postMap(ctx context.Context, path string, body any) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   path,
		body:   body,
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// toInt coerces a JSON-decoded value (float64 or int) to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
