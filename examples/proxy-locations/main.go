// proxy-locations — browse residential proxy geo targeting.
//
// Run me
// ------
//
//	cd sdk/go
//	export EVESES_API_KEY=sk_live_xxx
//	go run ./examples/proxy-locations
//
// What it does
// ------------
//
//  1. Builds an authenticated client (Bearer Sanctum API-key token).
//  2. Reads the top-level residential targeting (countries/regions/sets).
//  3. Drills into one country for its state/city/ISP detail.
//
// Drill-down note
// ---------------
//
// Locations() gives the coarse targeting list; LocationsDetail(country, type)
// returns the per-country state/city/ISP geo you'd wire into a picker.
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

	// Every SDK call takes a context.Context so callers can cancel /
	// time out the request from the outside. context.Background() is
	// fine for short-lived CLIs.
	ctx := context.Background()

	// Coarse residential targeting: countries / regions / sets.
	locs, err := client.Proxy.Locations(ctx, eveses.ProxyTypeResidential)
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("Residential targeting: %d top-level key(s)\n", len(locs))

	// Per-country drill-down: states / cities / ISPs for a picker.
	detail, err := client.Proxy.LocationsDetail(ctx, "us", eveses.ProxyTypeResidential)
	if err != nil {
		handle(err)
		return
	}

	states := extractList(detail, "states")
	cities := extractList(detail, "cities")
	fmt.Printf("US detail: %d state(s), %d city entr(ies)\n", len(states), len(cities))

	for i, s := range states {
		if i >= 5 {
			break
		}
		fmt.Printf("  state: %v\n", s)
	}
	for i, c := range cities {
		if i >= 5 {
			break
		}
		fmt.Printf("  city: %v\n", c)
	}
}

// extractList pulls a named array out of the detail envelope as []any.
func extractList(detail map[string]any, key string) []any {
	if raw, ok := detail[key].([]any); ok {
		return raw
	}
	return nil
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
