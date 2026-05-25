package eveses

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient spins up an httptest server pointed at handler, builds a
// Client with a tiny timeout, and returns both for assertions/teardown.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := New(Config{
		APIKey:  "sk_test_12345",
		BaseURL: srv.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client, srv
}

func TestActivationsCreate_HappyPath(t *testing.T) {
	var gotPath, gotAuth, gotIdempotency, gotMethod string
	var gotBody map[string]any

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotIdempotency = r.Header.Get("Idempotency-Key")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": {
				"order_id": "ord_abc",
				"status": "waiting_sms",
				"phone": "+380501112233",
				"country": "ua",
				"service": "telegram",
				"mode": "activation",
				"price_cents": 1500,
				"created_at": "2026-05-08T12:00:00Z"
			}
		}`))
	})

	maxPrice := 2000
	order, err := client.Activations.Create(context.Background(), &CreateActivationParams{
		Country:        "ua",
		Service:        "telegram",
		IdempotencyKey: "idem_test_1",
		MaxPriceCents:  &maxPrice,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/account/orders" {
		t.Errorf("path = %q, want /api/account/orders", gotPath)
	}
	if gotAuth != "Bearer sk_test_12345" {
		t.Errorf("Authorization = %q, want Bearer sk_test_12345", gotAuth)
	}
	if gotIdempotency != "idem_test_1" {
		t.Errorf("Idempotency-Key header = %q", gotIdempotency)
	}
	if gotBody["country"] != "ua" || gotBody["service"] != "telegram" {
		t.Errorf("body = %#v, missing required fields", gotBody)
	}
	if gotBody["mode"] != "activation" {
		t.Errorf("mode = %v, want activation (default)", gotBody["mode"])
	}
	if gotBody["max_price_cents"] != float64(2000) {
		t.Errorf("max_price_cents = %v, want 2000", gotBody["max_price_cents"])
	}

	if order.OrderID != "ord_abc" {
		t.Errorf("OrderID = %q, want ord_abc", order.OrderID)
	}
	if order.Status != "waiting_sms" {
		t.Errorf("Status = %q, want waiting_sms", order.Status)
	}
	if order.PriceCents == nil || *order.PriceCents != 1500 {
		t.Errorf("PriceCents = %v, want 1500", order.PriceCents)
	}
	if order.Raw["phone"] != "+380501112233" {
		t.Errorf("Raw didn't carry the original payload: %#v", order.Raw)
	}
}

func TestUnauthorized_MapsToAuthError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad token"}`))
	})

	_, err := client.Wallet.Balance(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if authErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", authErr.StatusCode)
	}
	if authErr.Message != "Bad token" {
		t.Errorf("Message = %q, want 'Bad token'", authErr.Message)
	}
}

func TestRateLimit_RetriesOnceAndSucceeds(t *testing.T) {
	var calls int32

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"slow down"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": {
				"balance": 5000,
				"held_balance": 200,
				"available_balance": 4800,
				"currency": "USD"
			}
		}`))
	})

	bal, err := client.Wallet.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance returned error after retry: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", got)
	}
	if bal.Balance != 5000 || bal.AvailableBalance != 4800 || bal.Currency != "USD" {
		t.Errorf("balance = %#v", bal)
	}
}

func TestRateLimit_ExhaustedReturnsRateLimitError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"too many"}`))
	})

	_, err := client.Wallet.Balance(context.Background())
	if err == nil {
		t.Fatalf("expected RateLimitError, got nil")
	}
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rl.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", rl.StatusCode)
	}
}

func TestCatalogServices_PassesModeAndQueriesProductsEndpoint(t *testing.T) {
	var gotPath, gotMode string

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMode = r.URL.Query().Get("mode")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"mode": "activation",
				"products": ["telegram","wa","vk"]
			}
		}`))
	})

	resp, err := client.Catalog.Services(context.Background(), &CatalogServicesParams{
		Mode:    OrderModeActivation,
		Country: "UA",
	})
	if err != nil {
		t.Fatalf("Services returned error: %v", err)
	}
	if gotPath != "/api/v1/numbers/products" {
		t.Errorf("path = %q, want /api/v1/numbers/products", gotPath)
	}
	if gotMode != "activation" {
		t.Errorf("mode query = %q, want activation", gotMode)
	}
	if len(resp.Services) != 3 || resp.Services[0] != "telegram" {
		t.Errorf("services = %v", resp.Services)
	}
	if resp.Country != "ua" {
		t.Errorf("country echo = %q, want ua (lowercased)", resp.Country)
	}
}

func TestCatalogPricing_TranslatesServiceAndDuration(t *testing.T) {
	var gotProduct, gotDuration, gotCountry string

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotProduct = r.URL.Query().Get("product")
		gotDuration = r.URL.Query().Get("duration")
		gotCountry = r.URL.Query().Get("country")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"mode": "rent",
				"country": "ua",
				"currency": "USD",
				"services": [{
					"name": "telegram",
					"durations": [
						{"duration_minutes": 60, "price_cents": 500, "available": true}
					]
				}]
			}
		}`))
	})

	dur := 60
	resp, err := client.Catalog.Pricing(context.Background(), &CatalogPricingParams{
		Country:         "UA",
		Service:         "telegram",
		Mode:            OrderModeRent,
		DurationMinutes: &dur,
	})
	if err != nil {
		t.Fatalf("Pricing returned error: %v", err)
	}
	if gotProduct != "telegram" {
		t.Errorf("product query = %q, want telegram (alias of service)", gotProduct)
	}
	if gotDuration != "60" {
		t.Errorf("duration query = %q, want 60 (alias of duration_minutes)", gotDuration)
	}
	if gotCountry != "ua" {
		t.Errorf("country query = %q, want ua", gotCountry)
	}
	if len(resp.Services) != 1 || resp.Services[0].Name != "telegram" {
		t.Fatalf("services = %#v", resp.Services)
	}
	if len(resp.Services[0].Durations) != 1 {
		t.Fatalf("durations = %#v", resp.Services[0].Durations)
	}
	d := resp.Services[0].Durations[0]
	if d.DurationMinutes != 60 || d.PriceCents == nil || *d.PriceCents != 500 {
		t.Errorf("duration = %#v", d)
	}
	if d.Available == nil || !*d.Available {
		t.Errorf("Available = %v, want true", d.Available)
	}
}

func TestVerifyWebhook(t *testing.T) {
	secret := "shh_super_secret"
	body := []byte(`{"event":"order.completed","order_id":"ord_abc"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	t.Run("valid signature", func(t *testing.T) {
		ok, err := VerifyWebhook(body, sig, ts, secret, 5*time.Minute)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !ok {
			t.Fatalf("expected valid signature to verify")
		}
	})

	t.Run("tampered body", func(t *testing.T) {
		ok, _ := VerifyWebhook([]byte(`{"event":"different"}`), sig, ts, secret, 5*time.Minute)
		if ok {
			t.Fatalf("expected tampered body to fail verification")
		}
	})

	t.Run("bad signature header", func(t *testing.T) {
		ok, _ := VerifyWebhook(body, "sha256=deadbeef", ts, secret, 5*time.Minute)
		if ok {
			t.Fatalf("expected bad signature to fail verification")
		}
	})

	t.Run("expired timestamp rejected", func(t *testing.T) {
		oldTs := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
		oldMac := hmac.New(sha256.New, []byte(secret))
		oldMac.Write([]byte(oldTs))
		oldMac.Write([]byte("."))
		oldMac.Write(body)
		oldSig := "sha256=" + hex.EncodeToString(oldMac.Sum(nil))
		ok, _ := VerifyWebhook(body, oldSig, oldTs, secret, 5*time.Minute)
		if ok {
			t.Fatalf("expected stale timestamp to be rejected")
		}
	})

	t.Run("tolerance=0 disables staleness check", func(t *testing.T) {
		oldTs := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
		oldMac := hmac.New(sha256.New, []byte(secret))
		oldMac.Write([]byte(oldTs))
		oldMac.Write([]byte("."))
		oldMac.Write(body)
		oldSig := "sha256=" + hex.EncodeToString(oldMac.Sum(nil))
		ok, err := VerifyWebhook(body, oldSig, oldTs, secret, 0)
		if err != nil || !ok {
			t.Fatalf("with tolerance=0 expected valid; ok=%v err=%v", ok, err)
		}
	})
}

func TestNew_RequiresAPIKey(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatalf("expected error when APIKey is empty")
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *Error, got %T", err)
	}
}
