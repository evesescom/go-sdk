package eveses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestEmailsList_HappyPath(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {"emails": [
				{"uuid": "eml_1", "address": "a@x.com", "domain": "x.com", "site": null, "status": "active", "price_cents": 100, "currency": "USD", "message_count": 0, "expires_at": null, "created_at": "2026-06-01T00:00:00+00:00"}
			]}
		}`))
	})

	emails, err := client.Emails.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if gotPath != "/api/account/emails" {
		t.Errorf("path = %q", gotPath)
	}
	if len(emails) != 1 || emails[0].Address != "a@x.com" {
		t.Fatalf("emails = %#v", emails)
	}
}

func TestEmailsQuote(t *testing.T) {
	var gotDomain, gotProvider string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotDomain = r.URL.Query().Get("domain")
		gotProvider = r.URL.Query().Get("provider")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"domain": "x.com", "provider": "hero", "price_cents": 150, "currency": "USD"}}`))
	})

	quote, err := client.Emails.Quote(context.Background(), &EmailQuoteParams{Domain: "x.com", Provider: "hero"})
	if err != nil {
		t.Fatalf("Quote returned error: %v", err)
	}
	if gotDomain != "x.com" {
		t.Errorf("domain query = %q, want x.com", gotDomain)
	}
	if gotProvider != "hero" {
		t.Errorf("provider query = %q, want hero", gotProvider)
	}
	if quote.PriceCents != 150 {
		t.Errorf("price_cents = %d, want 150", quote.PriceCents)
	}
}

func TestEmailsPurchase_WithIdempotency(t *testing.T) {
	var gotPath, gotIdempotency string
	var gotBody map[string]any
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIdempotency = r.Header.Get("Idempotency-Key")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data": {"uuid": "eml_new", "address": "b@x.com", "domain": "x.com", "status": "active", "price_cents": 100, "currency": "USD", "message_count": 0}}`))
	})

	addr, err := client.Emails.Purchase(context.Background(), &PurchaseEmailParams{
		Domain:         "x.com",
		Provider:       "hero",
		IdempotencyKey: "idem_eml_1",
	})
	if err != nil {
		t.Fatalf("Purchase returned error: %v", err)
	}
	if gotPath != "/api/account/emails/purchase" {
		t.Errorf("path = %q", gotPath)
	}
	if gotIdempotency != "idem_eml_1" {
		t.Errorf("Idempotency-Key = %q", gotIdempotency)
	}
	if gotBody["domain"] != "x.com" || gotBody["provider"] != "hero" {
		t.Errorf("body = %#v", gotBody)
	}
	if addr.UUID != "eml_new" || addr.Address != "b@x.com" {
		t.Errorf("addr = %#v", addr)
	}
}

func TestEmailsGet_Inbox(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {"uuid": "eml_1", "address": "a@x.com", "domain": "x.com", "status": "received", "price_cents": 100, "currency": "USD", "message_count": 1,
				"messages": [{"from": "sender@y.com", "subject": "Hi", "body": "Your code is 1234", "received_at": "2026-06-02T00:00:00+00:00"}]}
		}`))
	})

	order, err := client.Emails.Get(context.Background(), "eml_1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if gotPath != "/api/account/emails/eml_1" {
		t.Errorf("path = %q", gotPath)
	}
	if len(order.Messages) != 1 || order.Messages[0].Body != "Your code is 1234" {
		t.Fatalf("messages = %#v", order.Messages)
	}
	if order.Address != "a@x.com" {
		t.Errorf("address = %q", order.Address)
	}
}

func TestEmailsDelete(t *testing.T) {
	var gotPath, gotMethod string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"uuid": "eml_1", "address": "a@x.com", "domain": "x.com", "status": "cancelled", "price_cents": 100, "currency": "USD", "message_count": 0}}`))
	})

	addr, err := client.Emails.Delete(context.Background(), "eml_1")
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/account/emails/eml_1" {
		t.Errorf("path = %q", gotPath)
	}
	if addr.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", addr.Status)
	}
}
