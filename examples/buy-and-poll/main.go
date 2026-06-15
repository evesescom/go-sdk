// buy-and-poll — Full activation lifecycle.
//
// Run me
// ------
//
//	cd sdk/go
//	export EVESES_API_KEY=sk_live_xxx
//	go run ./examples/buy-and-poll
//	# Ctrl-C at any point to cancel the active order cleanly.
//
// What it does
// ------------
//
//  1. Creates an activation order for COUNTRY/SERVICE.
//  2. Polls Sms() every 5s for up to 5 minutes.
//  3. On SMS: prints text and calls Finish() to commit the spend.
//  4. On Ctrl-C OR poll timeout: calls Cancel() to refund the hold.
//
// Gotchas
// -------
//
//   - Sms() returns BOTH `Stored` (delivered via webhook) and `Fresh`
//     (pulled on demand). We de-duplicate by id.
//   - Don't poll faster than 5s — the API will 429. The SDK auto-retries
//     once on 429 honouring Retry-After, but heavy polling burns through
//     that allowance fast.
//   - Always Finish() or Cancel(). A dangling order keeps the held balance
//     locked until server-side expiry.
//   - We use signal.NotifyContext so Ctrl-C cancels the context AND
//     cancels any in-flight HTTP request the SDK is making.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	eveses "github.com/evesescom/go-sdk"
)

const (
	pollInterval = 5 * time.Second
	pollTimeout  = 5 * time.Minute
)

func main() {
	apiKey := envOr("EVESES_API_KEY", "sk_test_placeholder")
	country := envOr("EVESES_COUNTRY", "ua")
	service := envOr("EVESES_SERVICE", "telegram")

	client, err := eveses.New(eveses.Config{APIKey: apiKey})
	if err != nil {
		log.Fatalf("construct client: %v", err)
	}

	// Cancel cleanly on Ctrl-C. The returned ctx is cancelled the moment
	// SIGINT/SIGTERM lands, which aborts any in-flight HTTP request.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	idempotencyKey, err := randomHex(16)
	if err != nil {
		log.Fatalf("rand: %v", err)
	}

	order, err := client.Activations.Create(ctx, &eveses.CreateActivationParams{
		Country:        country,
		Service:        service,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		handleSDKError(err)
		return
	}
	fmt.Printf("Created order %s → phone %s\n", order.OrderID, order.Phone)
	fmt.Println("Polling for SMS (Ctrl-C to cancel the order)…")

	// We use a SEPARATE Background context for cancel/finish at the end:
	// if the main ctx is cancelled by SIGINT, we still need to make a
	// follow-up call to tell the server. Bound it with a fresh timeout.
	sms, pollErr := pollForSMS(ctx, client, order.OrderID)

	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelCleanup()

	if errors.Is(pollErr, context.Canceled) {
		fmt.Fprintln(os.Stderr, "\nCancellation requested — releasing the number…")
		if _, err := client.Activations.Cancel(cleanupCtx, order.OrderID); err != nil {
			var sdkErr *eveses.Error
			if errors.As(err, &sdkErr) && sdkErr.StatusCode == 404 {
				fmt.Println("Order already in a terminal state; nothing to cancel.")
				return
			}
			fmt.Fprintf(os.Stderr, "Cancel failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Cancelled cleanly.")
		return
	}
	if pollErr != nil {
		handleSDKError(pollErr)
		return
	}

	if sms == nil {
		fmt.Println("Timed out waiting for SMS — cancelling and refunding held balance.")
		if _, err := client.Activations.Cancel(cleanupCtx, order.OrderID); err != nil {
			handleSDKError(err)
		}
		return
	}

	fmt.Printf("Got SMS from %s: %q\n", sms.Sender, sms.Text)
	finished, err := client.Activations.Finish(cleanupCtx, order.OrderID)
	if err != nil {
		handleSDKError(err)
		return
	}
	fmt.Printf("Order %s finished (status=%s).\n", finished.OrderID, finished.Status)
}

// pollForSMS polls every pollInterval until a SMS arrives, the timeout
// fires, or ctx is cancelled. The first SMS is returned; (nil, nil) means
// "timed out cleanly"; ctx.Err() is wrapped on cancellation.
func pollForSMS(ctx context.Context, client *eveses.Client, orderID string) (*eveses.OrderSms, error) {
	deadline := time.Now().Add(pollTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		bundle, err := client.Activations.Sms(ctx, orderID)
		if err != nil {
			// ctx-cancelled errors come back as transport-level *Error.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		if msg := firstSMS(bundle.Stored, bundle.Fresh); msg != nil {
			return msg, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, nil
		}
		fmt.Printf("  ...no SMS yet, sleeping %s (deadline in %s)\n", pollInterval, remaining.Truncate(time.Second))

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// Loop and poll again.
		}
	}
}

// firstSMS returns the first message across the stored + fresh lists,
// de-duplicated by id. Returns nil if both lists are empty.
func firstSMS(stored, fresh []eveses.OrderSms) *eveses.OrderSms {
	seen := map[int]bool{}
	for _, s := range append(stored, fresh...) {
		if seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		copy := s
		return &copy
	}
	return nil
}

func handleSDKError(err error) {
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

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
