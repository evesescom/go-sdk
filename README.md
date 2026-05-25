# Eveses Go SDK

Official Go SDK for the [Eveses](https://eveses.com) API. Idiomatic, context-first, stdlib-only.

> Module path is a placeholder — adjust `go.mod` and your `go get` URL to wherever you publish.

## Install

```bash
go get github.com/evesescom/go-sdk
```

Requires Go 1.21+. No external dependencies.

## Quickstart

```go
client, _ := eveses.New(eveses.Config{APIKey: os.Getenv("EVESES_API_KEY")})
order, _ := client.Activations.Create(ctx, &eveses.CreateActivationParams{Country: "ua", Service: "telegram"})
sms, _   := client.Activations.Sms(ctx, order.OrderID)
bal, _   := client.Wallet.Balance(ctx)
svcs, _  := client.Catalog.Services(ctx, &eveses.CatalogServicesParams{Mode: eveses.OrderModeActivation, Country: "ua"})
```

`EVESES_API_KEY` should be a Sanctum API token (kind=`api_key`) issued from your Eveses dashboard. Never commit it — read it from the environment, a secret manager, or your platform's config store.

## Activations

```go
import "context"

ctx := context.Background()
order, err := client.Activations.Create(ctx, &eveses.CreateActivationParams{
    Country:        "ua",
    Service:        "telegram",
    IdempotencyKey: "abc-123", // sent as Idempotency-Key header
})

// Poll for SMS
bundle, _ := client.Activations.Sms(ctx, order.OrderID)
for _, m := range bundle.Stored { fmt.Println(m.Text) }
for _, m := range bundle.Fresh  { fmt.Println(m.Text) }

// Finish or cancel
_, _ = client.Activations.Finish(ctx, order.OrderID)
_, _ = client.Activations.Cancel(ctx, order.OrderID)

// Re-fetch
again, _ := client.Activations.Get(ctx, order.OrderID)
_ = again
```

## Wallet

```go
bal, err := client.Wallet.Balance(ctx)
fmt.Println(bal.AvailableBalance, bal.Currency) // e.g. 4800 USD (cents)
```

## Catalog

```go
countries, _ := client.Catalog.Countries(ctx, &eveses.CatalogCountriesParams{Mode: eveses.OrderModeActivation})
services,  _ := client.Catalog.Services(ctx,  &eveses.CatalogServicesParams{Mode: eveses.OrderModeActivation, Country: "ua"})

dur := 60
pricing, _ := client.Catalog.Pricing(ctx, &eveses.CatalogPricingParams{
    Country:         "ua",
    Service:         "telegram",
    Mode:            eveses.OrderModeRent,
    DurationMinutes: &dur,
})
```

The `Service` field on `CatalogPricingParams` is the friendlier name for the wire param `product`; the SDK translates it for you. Same for `DurationMinutes` → `duration`.

## Webhooks

Eveses signs every webhook delivery with HMAC-SHA256 over `"{timestamp}.{body}"` using your endpoint's signing secret.

- Header `X-Eveses-Signature` carries `sha256=<hex>`
- Header `X-Eveses-Timestamp` carries unix seconds

Always verify with the **raw** request body bytes — round-tripping through `json.Unmarshal/json.Marshal` reorders keys and breaks the signature.

```go
http.HandleFunc("/webhooks/eveses", func(w http.ResponseWriter, r *http.Request) {
    rawBody, _ := io.ReadAll(r.Body)
    sig := r.Header.Get("X-Eveses-Signature")
    ts  := r.Header.Get("X-Eveses-Timestamp")

    ok, err := eveses.VerifyWebhook(rawBody, sig, ts, os.Getenv("EVESES_WEBHOOK_SECRET"), 5*time.Minute)
    if err != nil || !ok {
        http.Error(w, "invalid signature", http.StatusBadRequest)
        return
    }

    // ... process the webhook (rawBody is the original JSON) ...
    w.WriteHeader(http.StatusOK)
})
```

Pass `tolerance = 0` to disable the staleness check.

## Errors

Every non-2xx response becomes a typed error. Use `errors.As`:

```go
var authErr *eveses.AuthError
var rlErr   *eveses.RateLimitError
var apiErr  *eveses.Error

switch {
case errors.As(err, &authErr):
    // 401 — bad/expired API key
case errors.As(err, &rlErr):
    // 429 (after 1 retry honouring Retry-After up to 60s)
    fmt.Println("retry after", rlErr.RetryAfter)
case errors.As(err, &apiErr):
    fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Message)
}
```

Status code → SDK error mapping:

| HTTP | Type | Code |
| --- | --- | --- |
| 400, 422 | `*Error` | `validation_failed` |
| 401 | `*AuthError` | `unauthenticated` |
| 403 | `*Error` | `forbidden` |
| 404 | `*Error` | `not_found` |
| 429 | `*RateLimitError` | `rate_limited` |
| 5xx | `*Error` | `server_error` |
| other | `*Error` | (empty) |

## Config

```go
client, _ := eveses.New(eveses.Config{
    APIKey:     "sk_…",
    BaseURL:    "https://api.eveses.com",   // optional
    Timeout:    30 * time.Second,           // ignored if HTTPClient is set
    HTTPClient: &http.Client{Timeout: 30 * time.Second},
    UserAgent:  "myapp/1.0",
    DefaultHeaders: map[string]string{"X-Tenant": "acme"},
})
```

## Behaviour notes

- **Auto-retry**: on `429`, the SDK sleeps up to `Retry-After` seconds (capped at 60s, default 1s) and retries once. Subsequent 429s surface as `*RateLimitError`.
- **Bearer auth**: every request carries `Authorization: Bearer <APIKey>`.
- **Idempotency**: when `IdempotencyKey` is set on `CreateActivationParams`, it is sent both in the JSON body and as the `Idempotency-Key` HTTP header.
- **Envelope unwrap**: responses of the form `{"data": {...}}` are auto-unwrapped; raw `{...}` objects are accepted too.
- **Concurrent use**: `*Client` is safe to share across goroutines.

## License

MIT
