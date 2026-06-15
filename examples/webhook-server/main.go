// webhook-server — Minimal net/http server that verifies Eveses webhooks.
//
// Run me
// ------
//
//	cd sdk/go
//	export EVESES_WEBHOOK_SECRET=whsec_xxx   # from your endpoint settings
//	export PORT=8787                          # optional
//	go run ./examples/webhook-server
//	# Then point Eveses at  http://localhost:8787/eveses/webhook
//	# (use ngrok / cloudflared in real life — Eveses needs a public URL.)
//
// What it does
// ------------
//
//   - Listens on POST /eveses/webhook
//   - Reads the RAW body via io.ReadAll BEFORE any JSON parsing (the
//     signature is over raw bytes — encoding/json.Unmarshal then Marshal
//     would reorder keys and invalidate the HMAC).
//   - Calls eveses.VerifyWebhook with X-Eveses-Signature and
//     X-Eveses-Timestamp. Default tolerance is 5 minutes — older
//     deliveries are rejected (replay protection).
//   - Returns 200 on success, 401 on bad signature, 400 on malformed body.
//
// Gotchas
// -------
//
//   - net/http hands you req.Body once; we drain it into a buffer because
//     the SDK needs the raw bytes.
//   - VerifyWebhook returns (false, nil) for ANY failure (missing header,
//     bad hex, expired timestamp). That's not an error — it just means
//     "not a valid Eveses delivery".
//   - Replay-protection window is 300s. Don't widen it unless your handler
//     is idempotent and you have a very good reason.
//   - Respond within ~10s. Enqueue heavy work and ACK fast.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	eveses "github.com/evesescom/go-sdk"
)

func main() {
	secret := envOr("EVESES_WEBHOOK_SECRET", "whsec_placeholder")
	port, err := strconv.Atoi(envOr("PORT", "8787"))
	if err != nil {
		log.Fatalf("invalid PORT: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/eveses/webhook", makeHandler(secret))

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Listening on http://%s/eveses/webhook", addr)
	log.Println("Configure this URL on your Eveses webhook endpoint.")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func makeHandler(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}

		// Slurp the raw body. Limit to 1 MiB to avoid OOM from a hostile
		// upstream; webhook payloads are tiny in practice.
		rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "read_failed"})
			return
		}
		defer r.Body.Close()

		signature := r.Header.Get("X-Eveses-Signature")
		timestamp := r.Header.Get("X-Eveses-Timestamp")

		ok, _ := eveses.VerifyWebhook(rawBody, signature, timestamp, secret, 5*time.Minute)
		if !ok {
			// Don't leak which check failed — that's a signature-forgery
			// oracle.
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_signature"})
			return
		}

		var payload map[string]any
		if len(rawBody) > 0 {
			if err := json.Unmarshal(rawBody, &payload); err != nil {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
				return
			}
		}

		eventType, _ := payload["type"].(string)
		if eventType == "" {
			eventType = "?"
		}
		log.Printf("Received verified webhook: type=%s", eventType)
		if pretty, err := json.MarshalIndent(payload, "", "  "); err == nil {
			log.Println(string(pretty))
		}

		// ACK fast. Real handlers should enqueue the event and respond here.
		respondJSON(w, http.StatusOK, map[string]bool{"received": true})
	}
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	data, _ := json.Marshal(body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
