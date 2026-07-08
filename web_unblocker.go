package eveses

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// WebUnblockerAccess is the white-label connection detail + quota for a user's
// Web Unblocker pool.
type WebUnblockerAccess struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	Example           string `json:"example"`
	Curl              string `json:"curl"`
	RequestsPurchased int    `json:"requests_purchased"`
	RequestsUsed      int    `json:"requests_used"`
	RequestsRemaining int    `json:"requests_remaining"`
}

// WebUnblockerOrder is a single Web Unblocker request-bundle order.
type WebUnblockerOrder struct {
	UUID       string         `json:"uuid"`
	Product    string         `json:"product"`
	Requests   int            `json:"requests"`
	Status     string         `json:"status"`
	PriceCents int            `json:"price_cents"`
	Currency   string         `json:"currency"`
	CreatedAt  string         `json:"created_at,omitempty"`
	Raw        map[string]any `json:"-"`
}

// WebUnblockerSubscription is the monthly Web Unblocker subscription snapshot.
type WebUnblockerSubscription struct {
	Status        string `json:"status"`
	Requests      int    `json:"requests"`
	DiscountPct   int    `json:"discount_pct"`
	NextRenewsAt  string `json:"next_renews_at,omitempty"`
	RenewFailures int    `json:"renew_failures"`
}

// WebUnblockerOverview is the response of WebUnblockerService.List.
type WebUnblockerOverview struct {
	Access       *WebUnblockerAccess       `json:"access"`
	Subscription *WebUnblockerSubscription `json:"subscription"`
	Orders       []WebUnblockerOrder       `json:"orders"`
}

// WebUnblockerPackage is one rung of the request-bundle price ladder.
type WebUnblockerPackage struct {
	Requests       int    `json:"requests"`
	Per1kCents     int    `json:"per_1k_cents"`
	TotalCents     int    `json:"total_cents"`
	BasePer1kCents int    `json:"base_per_1k_cents"`
	DiscountPct    int    `json:"discount_pct"`
	Recommended    bool   `json:"recommended,omitempty"`
	Currency       string `json:"currency"`
}

// WebUnblockerPackagesResponse is the response of WebUnblockerService.Packages.
type WebUnblockerPackagesResponse struct {
	Packages []WebUnblockerPackage `json:"packages"`
	Currency string                `json:"currency"`
}

// WebUnblockerQuote is the response of WebUnblockerService.Quote.
type WebUnblockerQuote struct {
	Product    string `json:"product"`
	Requests   int    `json:"requests"`
	Unit       string `json:"unit"`
	PriceCents int    `json:"price_cents"`
	Per1kCents int    `json:"per_1k_cents"`
	Currency   string `json:"currency"`
}

// WebUnblockerService wraps /api/account/web-unblocker.
type WebUnblockerService struct {
	client *Client
}

// List returns the user's Web Unblocker access, subscription and order history.
func (s *WebUnblockerService) List(ctx context.Context) (*WebUnblockerOverview, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/web-unblocker",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out WebUnblockerOverview
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode web-unblocker: " + err.Error()}
		}
	}
	if out.Orders == nil {
		out.Orders = []WebUnblockerOrder{}
	}
	return &out, nil
}

// Packages returns the request-bundle price ladder.
func (s *WebUnblockerService) Packages(ctx context.Context) (*WebUnblockerPackagesResponse, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/web-unblocker/packages",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out WebUnblockerPackagesResponse
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode web-unblocker packages: " + err.Error()}
		}
	}
	if out.Packages == nil {
		out.Packages = []WebUnblockerPackage{}
	}
	out.Currency = defaultCurrency(out.Currency)
	return &out, nil
}

// Quote prices a custom request bundle before buying.
func (s *WebUnblockerService) Quote(ctx context.Context, requests int, subscription bool) (*WebUnblockerQuote, error) {
	q := url.Values{}
	q.Set("requests", strconv.Itoa(requests))
	if subscription {
		q.Set("subscription", "1")
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/web-unblocker/quote",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var quote WebUnblockerQuote
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &quote); err != nil {
			return nil, &Error{Message: "decode web-unblocker quote: " + err.Error()}
		}
	}
	quote.Currency = defaultCurrency(quote.Currency)
	return &quote, nil
}

// PurchaseWebUnblockerParams configures WebUnblockerService.Purchase. When
// IdempotencyKey is set it is sent as the Idempotency-Key HTTP header.
type PurchaseWebUnblockerParams struct {
	// Requests is the number of requests to buy. Required (>= the smallest
	// package server-side).
	Requests int
	// Subscription requests a monthly auto-renewing bundle.
	Subscription bool
	// IdempotencyKey is sent as the Idempotency-Key header.
	IdempotencyKey string
}

// Purchase buys a request bundle (tops up the user's pool) and returns the
// created order (HTTP 201).
func (s *WebUnblockerService) Purchase(ctx context.Context, p *PurchaseWebUnblockerParams) (*WebUnblockerOrder, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	if p.Requests <= 0 {
		return nil, &Error{Message: "Requests is required"}
	}

	body := map[string]any{"requests": p.Requests}
	if p.Subscription {
		body["subscription"] = true
	}

	headers := map[string]string{}
	if p.IdempotencyKey != "" {
		headers["Idempotency-Key"] = p.IdempotencyKey
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    "/api/account/web-unblocker/purchase",
		body:    body,
		headers: headers,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var order WebUnblockerOrder
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &order); err != nil {
			return nil, &Error{Message: "decode web-unblocker order: " + err.Error()}
		}
	}
	if order.Currency == "" {
		order.Currency = "USD"
	}
	var rawMap map[string]any
	if err := json.Unmarshal(inner, &rawMap); err == nil {
		order.Raw = rawMap
	}
	return &order, nil
}

// CancelSubscription stops the Web Unblocker subscription's auto-renewal.
func (s *WebUnblockerService) CancelSubscription(ctx context.Context) (*WebUnblockerSubscription, error) {
	return s.subscriptionAction(ctx, "cancel")
}

// PauseSubscription skips Web Unblocker renewals until resumed.
func (s *WebUnblockerService) PauseSubscription(ctx context.Context) (*WebUnblockerSubscription, error) {
	return s.subscriptionAction(ctx, "pause")
}

// ResumeSubscription resumes the Web Unblocker subscription (next renewal a
// month out).
func (s *WebUnblockerService) ResumeSubscription(ctx context.Context) (*WebUnblockerSubscription, error) {
	return s.subscriptionAction(ctx, "resume")
}

func (s *WebUnblockerService) subscriptionAction(ctx context.Context, action string) (*WebUnblockerSubscription, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/account/web-unblocker/subscription/" + action,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var sub WebUnblockerSubscription
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &sub); err != nil {
			return nil, &Error{Message: "decode web-unblocker subscription: " + err.Error()}
		}
	}
	return &sub, nil
}
