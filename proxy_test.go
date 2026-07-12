package eveses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestProxyPurchase_Residential(t *testing.T) {
	var gotPath, gotMethod, gotIdempotency string
	var gotBody map[string]any

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotIdempotency = r.Header.Get("Idempotency-Key")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"uuid":"px_abc","type":"residential","kind":"metered","gb":10,
			"status":"active","price_cents":900,"currency":"USD",
			"auto_extend":false,"extendable":false
		}`))
	})

	order, err := client.Proxy.Buy(context.Background(), &ProxyPurchaseParams{
		Type:           ProxyTypeResidential,
		GB:             10,
		Subscription:   true,
		IdempotencyKey: "idem-px",
	})
	if err != nil {
		t.Fatalf("Purchase error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/proxy/orders" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotIdempotency != "idem-px" {
		t.Errorf("Idempotency-Key = %q", gotIdempotency)
	}
	if gotBody["type"] != "residential" || gotBody["gb"] != float64(10) || gotBody["subscription"] != true {
		t.Errorf("body = %#v", gotBody)
	}
	if order.UUID != "px_abc" || order.Status != "active" || order.PriceCents != 900 {
		t.Errorf("order = %#v", order)
	}
	if order.GB == nil || *order.GB != 10 {
		t.Errorf("GB = %v, want 10", order.GB)
	}
	if order.Raw["kind"] != "metered" {
		t.Errorf("Raw didn't carry payload: %#v", order.Raw)
	}
}

func TestProxyPurchase_StaticSendsSelection(t *testing.T) {
	var gotBody map[string]any
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"px_isp","type":"isp","status":"active","price_cents":300}`))
	})

	order, err := client.Proxy.Buy(context.Background(), &ProxyPurchaseParams{
		Type: ProxyTypeISP,
		Selection: &ProxyStaticSelection{
			ProductID: 9, PlanID: 4, LocationID: 51, LocationName: "Australia", Quantity: 3,
		},
	})
	if err != nil {
		t.Fatalf("Purchase error: %v", err)
	}
	if gotBody["type"] != "isp" || gotBody["product_id"] != float64(9) ||
		gotBody["plan_id"] != float64(4) || gotBody["location_id"] != float64(51) ||
		gotBody["quantity"] != float64(3) || gotBody["location_name"] != "Australia" {
		t.Errorf("body = %#v", gotBody)
	}
	if order.UUID != "px_isp" || order.Currency != "USD" {
		t.Errorf("order = %#v", order)
	}
}

func TestProxyQuote_ResidentialQueryParams(t *testing.T) {
	var gotPath, gotType, gotGB, gotSub string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotType = r.URL.Query().Get("type")
		gotGB = r.URL.Query().Get("gb")
		gotSub = r.URL.Query().Get("subscription")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"price_cents":900,"gb":10,"currency":"USD"}`))
	})

	q, err := client.Proxy.Quote(context.Background(), &ProxyQuoteParams{
		Type: ProxyTypeResidential, GB: 10, Subscription: true,
	})
	if err != nil {
		t.Fatalf("Quote error: %v", err)
	}
	if gotPath != "/api/v1/proxy/quote" {
		t.Errorf("path = %q", gotPath)
	}
	if gotType != "residential" || gotGB != "10" || gotSub != "true" {
		t.Errorf("query type=%q gb=%q subscription=%q", gotType, gotGB, gotSub)
	}
	if q.Raw["price_cents"] != float64(900) {
		t.Errorf("quote raw = %#v", q.Raw)
	}
}

func TestProxyList_ParsesResidentialAndOrders(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"residential":{"host":"proxy.eveses.com","username":"u","password":"p","traffic_gb_available":5,"traffic_gb_used":1},
			"subscription":{"status":"active","gb":10,"discount_pct":15},
			"orders":[{"uuid":"px_1","type":"isp","status":"active","price_cents":300}]
		}`))
	})

	list, err := client.Proxy.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if gotPath != "/api/v1/proxy/orders" {
		t.Errorf("path = %q", gotPath)
	}
	if list.Residential == nil || list.Residential.Username != "u" || list.Residential.TrafficGBAvailable != 5 {
		t.Errorf("residential = %#v", list.Residential)
	}
	if list.Subscription == nil || list.Subscription.Status != "active" || list.Subscription.DiscountPct != 15 {
		t.Errorf("subscription = %#v", list.Subscription)
	}
	if len(list.Orders) != 1 || list.Orders[0].UUID != "px_1" {
		t.Errorf("orders = %#v", list.Orders)
	}
}

func TestProxyExtendAndAutoRenew(t *testing.T) {
	var extendBody, autoBody map[string]any
	var extendPath, autoPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/extend"):
			extendPath = r.URL.Path
			_ = json.Unmarshal(body, &extendBody)
		case strings.HasSuffix(r.URL.Path, "/auto-renew"):
			autoPath = r.URL.Path
			_ = json.Unmarshal(body, &autoBody)
		}
		_, _ = w.Write([]byte(`{"uuid":"px_1","type":"isp","status":"active","price_cents":300,"auto_extend":true}`))
	})

	if _, err := client.Proxy.Extend(context.Background(), "px_1", 30); err != nil {
		t.Fatalf("Extend error: %v", err)
	}
	if extendPath != "/api/v1/proxy/orders/px_1/extend" || extendBody["days"] != float64(30) {
		t.Errorf("extend path=%q body=%#v", extendPath, extendBody)
	}

	order, err := client.Proxy.AutoRenew(context.Background(), "px_1", true)
	if err != nil {
		t.Fatalf("AutoRenew error: %v", err)
	}
	if autoPath != "/api/v1/proxy/orders/px_1/auto-renew" || autoBody["enabled"] != true {
		t.Errorf("auto-renew path=%q body=%#v", autoPath, autoBody)
	}
	if !order.AutoExtend {
		t.Errorf("AutoExtend = false, want true")
	}
}

func TestProxyResetSessionsAndSubscription(t *testing.T) {
	var resetPath, subPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sessions/reset"):
			resetPath = r.URL.Path
			_, _ = w.Write([]byte(`{"reset":true}`))
		case strings.HasSuffix(r.URL.Path, "/subscription/pause"):
			subPath = r.URL.Path
			_, _ = w.Write([]byte(`{"status":"paused","gb":10,"discount_pct":15}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	if err := client.Proxy.ResetSessions(context.Background()); err != nil {
		t.Fatalf("ResetSessions error: %v", err)
	}
	if resetPath != "/api/v1/proxy/sessions/reset" {
		t.Errorf("reset path = %q", resetPath)
	}

	sub, err := client.Proxy.SubscriptionPause(context.Background())
	if err != nil {
		t.Fatalf("SubscriptionPause error: %v", err)
	}
	if subPath != "/api/v1/proxy/subscription/pause" || sub.Status != "paused" {
		t.Errorf("subscription path=%q status=%q", subPath, sub.Status)
	}
}
