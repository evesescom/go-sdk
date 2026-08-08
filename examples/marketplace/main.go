// marketplace — browse and (optionally) buy from the Eveses marketplace.
//
// Run me
// ------
//
//	cd sdk/go
//	export EVESES_API_KEY=sk_live_xxx
//	go run ./examples/marketplace
//
// What it does
// ------------
//
//  1. Builds an authenticated client (Bearer Sanctum API-key token).
//  2. Reads the filter facets for the "accounts" category.
//  3. Lists the marketplace categories.
//  4. Fetches the public catalog grouped by attributes and prints a few
//     groups with their prices_cents.
//
// Grouping note
// -------------
//
// With GroupBy="attributes" the catalog collapses same-type products into
// groups, each carrying prices_cents variants — handy for a picker UI. Use
// GroupBy="country" to bucket by country instead.
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

	// Filter facets scoped to a category — country/origin/format/twofa.
	filters, err := client.Marketplace.Filters(ctx, "accounts")
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("Filters for 'accounts': %d facet keys\n", len(filters))

	// The available marketplace categories.
	categories, err := client.Marketplace.Categories(ctx)
	if err != nil {
		handle(err)
		return
	}
	fmt.Printf("Categories: %d keys\n", len(categories))

	// Public catalog, grouped by attributes so same-type products collapse
	// into groups carrying prices_cents variants.
	catalog, err := client.Marketplace.Catalog(ctx, &eveses.MarketplaceCatalogParams{
		Category: "accounts",
		Country:  "US",
		Origin:   "autoreg",
		GroupBy:  "attributes",
	})
	if err != nil {
		handle(err)
		return
	}

	// The catalog is a normalized JSON object; the groups live under "data"
	// (or "groups" depending on the shape). Print a few with prices_cents.
	groups := extractGroups(catalog)
	fmt.Printf("Catalog returned %d group(s):\n", len(groups))
	for i, g := range groups {
		if i >= 3 {
			break
		}
		fmt.Printf("  - group %d prices_cents=%v\n", i+1, g["prices_cents"])
	}

	// Buy + reveal (uncomment to actually purchase — this charges your wallet):
	//
	//	order, err := client.Marketplace.Buy(ctx, &eveses.MarketplaceBuyParams{
	//		Category:       "accounts",
	//		SKU:            "some-sku",
	//		Quantity:       1,
	//		IdempotencyKey: "abc-123",
	//	})
	//	if err != nil {
	//		handle(err)
	//		return
	//	}
	//	uuid, _ := order["uuid"].(string)
	//	secret, err := client.Marketplace.Reveal(ctx, uuid)
	//	if err != nil {
	//		handle(err)
	//		return
	//	}
	//	fmt.Printf("Revealed payload: %v\n", secret)
}

// extractGroups pulls the group list out of the catalog envelope, tolerating
// either a "data" or "groups" key.
func extractGroups(catalog map[string]any) []map[string]any {
	for _, key := range []string{"data", "groups"} {
		raw, ok := catalog[key].([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if g, ok := item.(map[string]any); ok {
				out = append(out, g)
			}
		}
		return out
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
