package eveses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestWebUnblockerList_HappyPath(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"access": {"host": "unblock.eveses.com", "port": 12323, "username": "u", "password": "p", "example": "e", "curl": "c", "requests_purchased": 10000, "requests_used": 1000, "requests_remaining": 9000},
				"subscription": {"status": "active", "requests": 10000, "discount_pct": 10, "renew_failures": 0},
				"orders": [{"uuid": "ord_wu", "product": "web_unblocker", "requests": 10000, "status": "active", "price_cents": 900, "currency": "USD", "created_at": "2026-06-01T00:00:00+00:00"}]
			}
		}`))
	})

	out, err := client.WebUnblocker.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if gotPath != "/api/account/web-unblocker" {
		t.Errorf("path = %q", gotPath)
	}
	if out.Access == nil || out.Access.RequestsRemaining != 9000 {
		t.Fatalf("access = %#v", out.Access)
	}
	if out.Subscription == nil || out.Subscription.Requests != 10000 {
		t.Errorf("subscription = %#v", out.Subscription)
	}
	if len(out.Orders) != 1 || out.Orders[0].Product != "web_unblocker" {
		t.Errorf("orders = %#v", out.Orders)
	}
}

func TestWebUnblockerQuote(t *testing.T) {
	var gotRequests string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRequests = r.URL.Query().Get("requests")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"product": "web_unblocker", "requests": 25000, "unit": "request", "price_cents": 2000, "per_1k_cents": 80, "currency": "USD"}}`))
	})

	quote, err := client.WebUnblocker.Quote(context.Background(), 25000, false)
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if gotRequests != "25000" {
		t.Errorf("requests query = %q, want 25000", gotRequests)
	}
	if quote.PriceCents != 2000 || quote.Per1kCents != 80 {
		t.Errorf("quote = %#v", quote)
	}
}

func TestWebUnblockerPurchase_WithIdempotency(t *testing.T) {
	var gotPath, gotIdempotency string
	var gotBody map[string]any
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIdempotency = r.Header.Get("Idempotency-Key")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data": {"uuid": "ord_wu", "product": "web_unblocker", "requests": 10000, "status": "active", "price_cents": 900, "currency": "USD"}}`))
	})

	order, err := client.WebUnblocker.Purchase(context.Background(), &PurchaseWebUnblockerParams{
		Requests:       10000,
		Subscription:   true,
		IdempotencyKey: "idem_wu_1",
	})
	if err != nil {
		t.Fatalf("Purchase returned error: %v", err)
	}
	if gotPath != "/api/account/web-unblocker/purchase" {
		t.Errorf("path = %q", gotPath)
	}
	if gotIdempotency != "idem_wu_1" {
		t.Errorf("Idempotency-Key = %q", gotIdempotency)
	}
	if gotBody["requests"] != float64(10000) || gotBody["subscription"] != true {
		t.Errorf("body = %#v", gotBody)
	}
	if order.UUID != "ord_wu" || order.Requests != 10000 {
		t.Errorf("order = %#v", order)
	}
}

func TestWebUnblockerResumeSubscription(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"status": "active", "requests": 10000, "discount_pct": 10, "next_renews_at": "2026-08-01T00:00:00+00:00", "renew_failures": 0}}`))
	})

	sub, err := client.WebUnblocker.ResumeSubscription(context.Background())
	if err != nil {
		t.Fatalf("ResumeSubscription returned error: %v", err)
	}
	if gotPath != "/api/account/web-unblocker/subscription/resume" {
		t.Errorf("path = %q", gotPath)
	}
	if sub.Status != "active" {
		t.Errorf("status = %q, want active", sub.Status)
	}
}
