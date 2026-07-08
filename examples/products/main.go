// products — Browse the proxies, web-unblocker and emails product lines.
//
// Run me
// ------
//
//	cd sdk/go
//	export EVESES_API_KEY=sk_live_xxx
//	go run ./examples/products
//
// What it does
// ------------
//
//  1. Lists the user's residential proxies, static orders and subscription.
//  2. Quotes a residential GB top-up (read-only — does NOT purchase).
//  3. Lists the Web Unblocker quota and quotes a request bundle.
//  4. Lists rented inbox addresses and, if any exist, refreshes the first
//     inbox (Emails.Get live-syncs reseller mail).
//
// Everything here is read-only. The commented Purchase() calls show how to
// buy with an idempotency key — uncomment at your own (billed) risk.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	eveses "github.com/evesescom/go-sdk"
)

func main() {
	apiKey := envOr("EVESES_API_KEY", "sk_test_placeholder")

	client, err := eveses.New(eveses.Config{APIKey: apiKey})
	if err != nil {
		log.Fatalf("construct client: %v", err)
	}
	ctx := context.Background()

	// ── Proxies ──────────────────────────────────────────────────────────
	proxies, err := client.Proxies.List(ctx)
	if err != nil {
		handle(err)
		return
	}
	if proxies.Residential != nil {
		fmt.Printf(
			"Residential: %.2f GB available (used %.2f) @ %s\n",
			proxies.Residential.TrafficGBAvailable,
			proxies.Residential.TrafficGBUsed,
			proxies.Residential.Host,
		)
	} else {
		fmt.Println("Residential: not provisioned yet.")
	}
	fmt.Printf("Proxy orders: %d\n", len(proxies.Orders))

	gb := 5.0
	pq, err := client.Proxies.Quote(ctx, &eveses.ProxyQuoteParams{Type: "residential", GB: &gb})
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("Quote for %.0f GB residential: %.2f %s\n", gb, float64(pq.PriceCents)/100, pq.Currency)

	// To buy (billed!):
	// order, _ := client.Proxies.Purchase(ctx, &eveses.PurchaseProxyParams{
	//     Type: "residential", GB: &gb, IdempotencyKey: "unique-intent-id",
	// })

	// ── Web Unblocker ────────────────────────────────────────────────────
	unblock, err := client.WebUnblocker.List(ctx)
	if err != nil {
		handle(err)
		return
	}
	if unblock.Access != nil {
		fmt.Printf("Web Unblocker: %d requests remaining\n", unblock.Access.RequestsRemaining)
	} else {
		fmt.Println("Web Unblocker: not provisioned yet.")
	}
	wq, err := client.WebUnblocker.Quote(ctx, 25000, false)
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("Quote for %d requests: %.2f %s (%d/1k)\n", wq.Requests, float64(wq.PriceCents)/100, wq.Currency, wq.Per1kCents)

	// ── Emails ───────────────────────────────────────────────────────────
	emails, err := client.Emails.List(ctx, false)
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("Rented inboxes: %d\n", len(emails))
	if len(emails) > 0 {
		// Emails.Get live-syncs reseller inboxes — poll it for new mail.
		order, err := client.Emails.Get(ctx, emails[0].UUID)
		if err != nil {
			handle(err)
			return
		}
		fmt.Printf("Inbox %s has %d message(s):\n", order.Address, len(order.Messages))
		for _, m := range order.Messages {
			fmt.Printf("  from %s — %s\n", m.From, m.Subject)
		}
	}
}

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
