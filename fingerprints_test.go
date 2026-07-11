package eveses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFingerprintsGenerate(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fingerprint":{"id":"fp_1","userAgent":{"value":"UA"}},"price_micro_usd":1600}`))
	})

	res, err := client.Fingerprints.Generate(context.Background(), map[string]any{"country": "US"})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if res.PriceMicroUSD != 1600 {
		t.Errorf("price = %d, want 1600", res.PriceMicroUSD)
	}
	if res.Payload["id"] != "fp_1" {
		t.Errorf("payload id = %v, want fp_1", res.Payload["id"])
	}
	if gotMethod != http.MethodPost || gotPath != "/api/account/fingerprints/generate" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody["country"] != "US" {
		t.Errorf("body country = %v", gotBody["country"])
	}
}

func TestFingerprintsRandom(t *testing.T) {
	var gotPath string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fingerprint":{"id":"fp_r"}}`))
	})

	res, err := client.Fingerprints.Random(context.Background(), nil)
	if err != nil {
		t.Fatalf("Random error: %v", err)
	}
	if res.Payload["id"] != "fp_r" {
		t.Errorf("payload id = %v, want fp_r", res.Payload["id"])
	}
	if !strings.HasSuffix(gotPath, "/fingerprints/random") {
		t.Errorf("path = %s", gotPath)
	}
}
