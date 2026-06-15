// quickstart — Hello-world for the Eveses Go SDK.
//
// Run me
// ------
//
//	cd sdk/go
//	export EVESES_API_KEY=sk_live_xxx
//	go run ./examples/quickstart
//
// What it does
// ------------
//
//  1. Builds an authenticated client (Bearer Sanctum API-key token).
//  2. Reads the wallet balance (currency + available funds).
//  3. Lists service codes for one country.
//  4. Buys ONE activation with an idempotency key.
//
// Idempotency note
// ----------------
//
// We send a random IdempotencyKey so this script is safe to retry on
// network blips: the API returns the SAME order on a retry rather than
// charging you twice for two numbers. In production, generate the key
// once per user intent (when the user clicks Buy), not per HTTP attempt.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"

	eveses "github.com/evesescom/go-sdk"
)

func main() {
	apiKey := envOr("EVESES_API_KEY", "sk_test_placeholder")
	country := envOr("EVESES_COUNTRY", "ua")
	service := envOr("EVESES_SERVICE", "telegram")

	client, err := eveses.New(eveses.Config{APIKey: apiKey})
	if err != nil {
		log.Fatalf("construct client: %v", err)
	}

	// Every SDK call takes a context.Context so callers can cancel /
	// time out the request from the outside. context.Background() is
	// fine for short-lived CLIs.
	ctx := context.Background()

	// Wallet balance is reported in MINOR units (cents). Mind the split:
	//   AvailableBalance — spendable right now
	//   HeldBalance      — reserved against in-flight orders
	//   Balance          — AvailableBalance + HeldBalance
	wallet, err := client.Wallet.Balance(ctx)
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf(
		"Wallet: %.2f %s available (held: %.2f)\n",
		float64(wallet.AvailableBalance)/100,
		wallet.Currency,
		float64(wallet.HeldBalance)/100,
	)

	// Services() is the global product catalog for the mode; Country is
	// informational on the v1 endpoint today.
	services, err := client.Catalog.Services(ctx, &eveses.CatalogServicesParams{
		Mode:    eveses.OrderModeActivation,
		Country: country,
	})
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("%d services available (mode=%s)\n", len(services.Services), services.Mode)
	if !contains(services.Services, service) {
		fmt.Fprintf(os.Stderr, "Warning: '%s' not in catalog — request may 404.\n", service)
	}

	// Idempotency key MUST be stable across retries of the same intent.
	// 16 random bytes hex-encoded is plenty for a one-shot call.
	idempotencyKey, err := randomHex(16)
	if err != nil {
		log.Fatalf("rand: %v", err)
	}

	order, err := client.Activations.Create(ctx, &eveses.CreateActivationParams{
		Country:        country,
		Service:        service,
		Mode:           eveses.OrderModeActivation,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("Created order %s: phone=%s status=%s\n", order.OrderID, order.Phone, order.Status)
	fmt.Println("Next: poll client.Activations.Sms(ctx, order.OrderID) for the code.")
}

// handle prints SDK errors in an idiomatic Go style using errors.As to
// pluck out the typed sub-errors.
func handle(err error) {
	var authErr *eveses.AuthError
	if errors.As(err, &authErr) {
		fmt.Fprintln(os.Stderr, "Auth failed — check EVESES_API_KEY (must start with sk_).")
		os.Exit(1)
	}
	var rlErr *eveses.RateLimitError
	if errors.As(err, &rlErr) {
		fmt.Fprintf(os.Stderr, "Rate limited (retry-after=%s): %s\n", rlErr.RetryAfter, rlErr.Message)
		os.Exit(1)
	}
	var sdkErr *eveses.Error
	if errors.As(err, &sdkErr) {
		fmt.Fprintf(os.Stderr, "SDK error (%d): %s\n", sdkErr.StatusCode, sdkErr.Message)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Unexpected error: %v\n", err)
	os.Exit(1)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
