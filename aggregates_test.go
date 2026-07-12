package eveses

import (
	"context"
	"net/http"
	"testing"
)

func TestOrdersList_GlobalFeed(t *testing.T) {
	var gotPath, gotService, gotLimit string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotService = r.URL.Query().Get("service")
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data":[{"source":"proxy","id":"b1f2","status":"active","amount_cents":450,"currency":"USD","title":"Residential · 5 GB","detail_url":"/api/v1/proxy/orders/b1f2"}],
			"meta":{"next_cursor":"MjA","has_more":true}
		}`))
	})

	page, err := client.Orders.List(context.Background(), &OrdersListParams{
		Service: []string{"proxy", "numbers"}, Limit: 20,
	})
	if err != nil {
		t.Fatalf("Orders.List error: %v", err)
	}
	if gotPath != "/api/v1/orders" {
		t.Errorf("path = %q", gotPath)
	}
	if gotService != "proxy,numbers" || gotLimit != "20" {
		t.Errorf("query service=%q limit=%q", gotService, gotLimit)
	}
	if len(page.Data) != 1 || page.Data[0].Source != "proxy" || page.Data[0].AmountCents != 450 {
		t.Errorf("data = %#v", page.Data)
	}
	if !page.Meta.HasMore || page.Meta.NextCursor != "MjA" {
		t.Errorf("meta = %#v", page.Meta)
	}
	if page.Data[0].Raw["detail_url"] != "/api/v1/proxy/orders/b1f2" {
		t.Errorf("raw not attached: %#v", page.Data[0].Raw)
	}
}

func TestOrdersGet_Single(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"source":"numbers","id":"ord_1","status":"completed","amount_cents":1500}}`))
	})

	v, err := client.Orders.Get(context.Background(), "ord_1")
	if err != nil {
		t.Fatalf("Orders.Get error: %v", err)
	}
	if gotPath != "/api/v1/orders/ord_1" {
		t.Errorf("path = %q", gotPath)
	}
	if v.Source != "numbers" || v.Status != "completed" || v.AmountCents != 1500 {
		t.Errorf("view = %#v", v)
	}
}

func TestPricingAll(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"currency":"USD","numbers":{"from_cents":12},"captcha":{"unit":"per_solve"}}`))
	})

	p, err := client.Pricing.All(context.Background())
	if err != nil {
		t.Fatalf("Pricing.All error: %v", err)
	}
	if gotPath != "/api/v1/pricing" {
		t.Errorf("path = %q", gotPath)
	}
	if p.Currency != "USD" || p.Numbers["from_cents"] != float64(12) {
		t.Errorf("pricing = %#v", p)
	}
}

func TestQuotasGet(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"proxy":[{"provider":"iproyal","unit":"gb","remaining":3.2}]}`))
	})

	q, err := client.Quotas.Get(context.Background())
	if err != nil {
		t.Fatalf("Quotas.Get error: %v", err)
	}
	if gotPath != "/api/v1/quotas" {
		t.Errorf("path = %q", gotPath)
	}
	if len(q.Proxy) != 1 || q.Proxy[0].Provider != "iproyal" || q.Proxy[0].Remaining != 3.2 {
		t.Errorf("quotas = %#v", q.Proxy)
	}
}

func TestMeGet_FeaturesAndAbilities(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"email":"a@b.c","abilities":["*"],"features":{"proxy":true,"captcha":false}}`))
	})

	me, err := client.Me.Get(context.Background())
	if err != nil {
		t.Fatalf("Me.Get error: %v", err)
	}
	if gotPath != "/api/v1/me" {
		t.Errorf("path = %q", gotPath)
	}
	if len(me.Abilities) != 1 || me.Abilities[0] != "*" {
		t.Errorf("abilities = %#v", me.Abilities)
	}
	if !me.Features["proxy"] || me.Features["captcha"] {
		t.Errorf("features = %#v", me.Features)
	}
}

func TestCaptchaUsage(t *testing.T) {
	var gotPath, gotStatus string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotStatus = r.URL.Query().Get("status")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data":[{"id":84213,"type":"recaptcha_v2","status":"ready","cost_micro_usd":3400}],
			"meta":{"has_more":false,"unbilled_micro_usd":7200}
		}`))
	})

	u, err := client.Captcha.Usage(context.Background(), &CaptchaUsageParams{Status: "ready"})
	if err != nil {
		t.Fatalf("Captcha.Usage error: %v", err)
	}
	if gotPath != "/api/v1/captcha/usage" || gotStatus != "ready" {
		t.Errorf("path=%q status=%q", gotPath, gotStatus)
	}
	if len(u.Data) != 1 || u.Data[0].ID != 84213 || u.Data[0].CostMicroUSD != 3400 {
		t.Errorf("data = %#v", u.Data)
	}
	if u.Meta.UnbilledMicroUSD != 7200 {
		t.Errorf("meta = %#v", u.Meta)
	}
}

func TestNumbersBatchAndActions(t *testing.T) {
	var batchPath, retryPath, autoPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/numbers/orders/batch":
			batchPath = r.URL.Path
			_, _ = w.Write([]byte(`{"data":[{"order_id":"o1","status":"waiting_sms"},{"order_id":"o2","status":"waiting_sms"}]}`))
		case r.URL.Path == "/api/v1/numbers/orders/o1/retry":
			retryPath = r.URL.Path
			_, _ = w.Write([]byte(`{"order_id":"o1","status":"waiting_sms"}`))
		case r.URL.Path == "/api/v1/numbers/orders/o1/auto-renew":
			autoPath = r.URL.Path
			_, _ = w.Write([]byte(`{"order_id":"o1","status":"active"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	orders, err := client.Numbers.CreateBatch(context.Background(), []*CreateNumberParams{
		{Country: "ua", Service: "telegram"},
		{Country: "pl", Service: "wa"},
	})
	if err != nil {
		t.Fatalf("CreateBatch error: %v", err)
	}
	if batchPath != "/api/v1/numbers/orders/batch" || len(orders) != 2 || orders[1].OrderID != "o2" {
		t.Errorf("batch path=%q orders=%#v", batchPath, orders)
	}

	if _, err := client.Numbers.Retry(context.Background(), "o1"); err != nil {
		t.Fatalf("Retry error: %v", err)
	}
	if retryPath != "/api/v1/numbers/orders/o1/retry" {
		t.Errorf("retry path = %q", retryPath)
	}

	if _, err := client.Numbers.AutoRenew(context.Background(), "o1", true); err != nil {
		t.Fatalf("AutoRenew error: %v", err)
	}
	if autoPath != "/api/v1/numbers/orders/o1/auto-renew" {
		t.Errorf("auto path = %q", autoPath)
	}
}
