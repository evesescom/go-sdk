package eveses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestProxiesList_HappyPath(t *testing.T) {
	var gotPath, gotMethod string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"residential": {
					"host": "proxy.eveses.com",
					"ports": {"http": 12321, "socks5": 32325},
					"username": "u1",
					"password": "p1",
					"example": "proxy.eveses.com:12321:u1:p1",
					"curl": "curl -x http://u1:p1@proxy.eveses.com:12321 https://api.ipify.org",
					"traffic_gb_available": 4.5,
					"traffic_gb_used": 1.5
				},
				"subscription": {"status": "active", "gb": 10, "discount_pct": 5, "next_renews_at": "2026-07-01T00:00:00+00:00", "renew_failures": 0},
				"orders": [
					{"uuid": "ord_1", "type": "residential", "kind": "metered", "gb": 5, "quantity": 1, "location": null, "status": "active", "price_cents": 500, "currency": "USD", "proxies": null, "auto_extend": false, "extendable": false, "expires_at": null, "created_at": "2026-06-01T00:00:00+00:00"}
				]
			}
		}`))
	})

	out, err := client.Proxies.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/account/proxies" {
		t.Errorf("path = %q, want /api/account/proxies", gotPath)
	}
	if out.Residential == nil || out.Residential.Ports.HTTP != 12321 {
		t.Fatalf("residential = %#v", out.Residential)
	}
	if out.Residential.TrafficGBAvailable != 4.5 {
		t.Errorf("traffic_gb_available = %v, want 4.5", out.Residential.TrafficGBAvailable)
	}
	if out.Subscription == nil || out.Subscription.Status != "active" {
		t.Errorf("subscription = %#v", out.Subscription)
	}
	if len(out.Orders) != 1 || out.Orders[0].UUID != "ord_1" {
		t.Fatalf("orders = %#v", out.Orders)
	}
}

func TestProxiesQuote_StaticSelection(t *testing.T) {
	var gotType, gotProductID, gotQuantity string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotType = r.URL.Query().Get("type")
		gotProductID = r.URL.Query().Get("product_id")
		gotQuantity = r.URL.Query().Get("quantity")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"type": "isp", "quantity": 2, "price_cents": 800, "currency": "USD"}}`))
	})

	productID, planID, locID, qty := 7, 3, 11, 2
	quote, err := client.Proxies.Quote(context.Background(), &ProxyQuoteParams{
		Type:       "isp",
		ProductID:  &productID,
		PlanID:     &planID,
		LocationID: &locID,
		Quantity:   &qty,
	})
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if gotType != "isp" {
		t.Errorf("type query = %q, want isp", gotType)
	}
	if gotProductID != "7" {
		t.Errorf("product_id query = %q, want 7", gotProductID)
	}
	if gotQuantity != "2" {
		t.Errorf("quantity query = %q, want 2", gotQuantity)
	}
	if quote.PriceCents != 800 {
		t.Errorf("price_cents = %d, want 800", quote.PriceCents)
	}
	if quote.Raw["type"] != "isp" {
		t.Errorf("Raw didn't carry payload: %#v", quote.Raw)
	}
}

func TestProxiesPurchase_ResidentialWithIdempotency(t *testing.T) {
	var gotPath, gotIdempotency string
	var gotBody map[string]any
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIdempotency = r.Header.Get("Idempotency-Key")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data": {"uuid": "ord_new", "type": "residential", "kind": "metered", "gb": 5, "quantity": 1, "status": "active", "price_cents": 500, "currency": "USD", "auto_extend": false, "extendable": false}}`))
	})

	gb := 5.0
	order, err := client.Proxies.Purchase(context.Background(), &PurchaseProxyParams{
		Type:           "residential",
		GB:             &gb,
		Subscription:   true,
		IdempotencyKey: "idem_proxy_1",
	})
	if err != nil {
		t.Fatalf("Purchase returned error: %v", err)
	}
	if gotPath != "/api/account/proxies/purchase" {
		t.Errorf("path = %q", gotPath)
	}
	if gotIdempotency != "idem_proxy_1" {
		t.Errorf("Idempotency-Key = %q", gotIdempotency)
	}
	if gotBody["type"] != "residential" || gotBody["gb"] != float64(5) || gotBody["subscription"] != true {
		t.Errorf("body = %#v", gotBody)
	}
	if order.UUID != "ord_new" || order.PriceCents != 500 {
		t.Errorf("order = %#v", order)
	}
}

func TestProxiesAutoRenew(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"uuid": "ord_1", "type": "isp", "kind": "static", "quantity": 1, "status": "active", "price_cents": 400, "currency": "USD", "auto_extend": true, "extendable": true}}`))
	})

	order, err := client.Proxies.AutoRenew(context.Background(), "ord_1", true)
	if err != nil {
		t.Fatalf("AutoRenew returned error: %v", err)
	}
	if gotPath != "/api/account/proxies/ord_1/auto-renew" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["enabled"] != true {
		t.Errorf("body = %#v, want enabled=true", gotBody)
	}
	if !order.AutoExtend {
		t.Errorf("AutoExtend = %v, want true", order.AutoExtend)
	}
}

func TestProxiesCancelSubscription(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"status": "cancelled", "gb": 10, "discount_pct": 0, "renew_failures": 0}}`))
	})

	sub, err := client.Proxies.CancelSubscription(context.Background())
	if err != nil {
		t.Fatalf("CancelSubscription returned error: %v", err)
	}
	if gotPath != "/api/account/proxies/subscription/cancel" {
		t.Errorf("path = %q", gotPath)
	}
	if sub.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", sub.Status)
	}
}
