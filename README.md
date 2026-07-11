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

## Proxy

Residential (metered, per-GB) and static (per-IP: ISP, datacenter, IPv6, sneaker, mobile) proxies. Read the catalogue, quote, buy, list, extend, auto-renew, reset sessions, view usage, and manage the residential subscription.

```go
// Catalogue / connection info
pkgs, _   := client.Proxy.Packages(ctx)                                 // residential GB ladder
eps, _    := client.Proxy.Endpoints(ctx)                                // white-label entry subdomains + ports
cat, _    := client.Proxy.Catalog(ctx)                                  // static (per-IP) products/plans
locs, _   := client.Proxy.Locations(ctx, eveses.ProxyTypeResidential)   // targeting

// Quote + purchase residential GB
q, _ := client.Proxy.Quote(ctx, &eveses.ProxyQuoteParams{Type: eveses.ProxyTypeResidential, GB: 5})
order, _ := client.Proxy.Purchase(ctx, &eveses.ProxyPurchaseParams{
    Type: eveses.ProxyTypeResidential, GB: 5, IdempotencyKey: "abc-123",
})

// Or buy static IPs via a product/plan/location selection
_, _ = client.Proxy.Purchase(ctx, &eveses.ProxyPurchaseParams{
    Type:      eveses.ProxyTypeISP,
    Selection: &eveses.ProxyStaticSelection{ProductID: 1, PlanID: 2, LocationID: 3, Quantity: 2},
})

list, _ := client.Proxy.List(ctx)                      // residential sub-user + subscription + per-IP orders
_, _ = client.Proxy.Extend(ctx, order.UUID, 30)        // renew a per-IP order
_, _ = client.Proxy.AutoRenew(ctx, order.UUID, true)   // toggle auto-extend
_ = client.Proxy.ResetSessions(ctx)                    // rotate residential sticky sessions
usage, _ := client.Proxy.Usage(ctx, "2026-06-01", "2026-06-30")
_, _ = client.Proxy.Trial(ctx)                         // one-time free trial

// Residential subscription lifecycle
_, _ = client.Proxy.SubscriptionCancel(ctx)
_, _ = client.Proxy.SubscriptionPause(ctx)
_, _ = client.Proxy.SubscriptionResume(ctx)
```

## WebUnblocker

Request-metered Web Unblocker access. Packages, quoting, purchasing, trial, access/quota checks, and subscription lifecycle.

```go
pkgs, _ := client.WebUnblocker.Packages(ctx)
q, _    := client.WebUnblocker.Quote(ctx, 10000, false)              // requests, subscription
order, _ := client.WebUnblocker.Purchase(ctx, 10000, false, "abc-123") // requests, subscription, idempotencyKey
_, _ = client.WebUnblocker.Trial(ctx)                                // one-time free trial

access, _ := client.WebUnblocker.Access(ctx)                         // credentials + quota + orders
fmt.Println(access.Host, access.Port, access.Username, access.RequestsLeft)

_, _ = client.WebUnblocker.SubscriptionCancel(ctx)
_, _ = client.WebUnblocker.SubscriptionPause(ctx)
_, _ = client.WebUnblocker.SubscriptionResume(ctx)
```

## Emails

Rent disposable email mailboxes and read their inbound messages.

```go
domains, _ := client.Emails.Domains(ctx, "")                          // optional site filter
q, _       := client.Emails.Quote(ctx, "example.com", "site", "provider")
order, _   := client.Emails.Purchase(ctx, "example.com", "site", "provider", "abc-123")

orders, _  := client.Emails.List(ctx, false)                          // includeReleased
mailbox, _ := client.Emails.Get(ctx, order.UUID)
msgs, _    := client.Emails.Messages(ctx, order.UUID, 1, 20)          // uuid, page, perPage
for _, m := range msgs { fmt.Println(m.From, m.Subject) }

_ = client.Emails.MarkRead(ctx, order.UUID, msgs[0].ID)
_ = client.Emails.Release(ctx, order.UUID)                            // delete the mailbox
```

## Captcha

Pay-per-use captcha solving (count-on-success). `Solve` submits the task and blocks, polling until the result is ready, failed, or the timeout elapses.

```go
sol, err := client.Captcha.Solve(ctx, "recaptcha_v2", map[string]any{
    "sitekey": "6Le-...",
    "url":     "https://example.com",
}, &eveses.CaptchaSolveParams{TimeoutSec: 180})
if err != nil { log.Fatal(err) }
fmt.Println(sol.Solution)
```

## Fingerprints

Pay-per-use browser-fingerprint generation (count-on-success). Synchronous — one request returns a complete fingerprint.

```go
fp, _ := client.Fingerprints.Generate(ctx, map[string]any{
    "format": "chrome", "country": "us",
})
rnd, _ := client.Fingerprints.Random(ctx, nil)
fmt.Println(fp.Payload, fp.PriceMicroUSD)
```

## Trial

Inspect the account's trial state and subscribe to trial-eligible services.

```go
st, _ := client.Trial.Status(ctx)
fmt.Println(st.Active, st.Services)
_, _ = client.Trial.Subscribe(ctx, []string{"web-unblocker", "proxies"})
```

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

## Changelog

### 0.3.0

- **Proxy** (`client.Proxy`) — residential (per-GB) and static (per-IP: ISP, datacenter, IPv6, sneaker, mobile) proxies: `Packages`, `Endpoints`, `Catalog`, `Locations`, `Quote`, `Purchase`, `List`, `Extend`, `AutoRenew`, `ResetSessions`, `Usage`, `Trial`, and residential subscription control (`SubscriptionCancel` / `SubscriptionPause` / `SubscriptionResume`).
- **WebUnblocker** (`client.WebUnblocker`) — `Packages`, `Quote`, `Purchase`, `Trial`, `Access`, and subscription lifecycle (`SubscriptionCancel` / `SubscriptionPause` / `SubscriptionResume`).
- **Emails** (`client.Emails`) — rentable disposable mailboxes: `Domains`, `Quote`, `Purchase`, `List`, `Get`, `Messages`, `MarkRead`, `Release`.
- **Captcha** (`client.Captcha`) — pay-per-use captcha solving with a blocking, self-polling `Solve`.
- **Fingerprints** (`client.Fingerprints`) — pay-per-use synchronous browser-fingerprint generation: `Generate`, `Random`.
- **Trial** (`client.Trial`) — `Status`, `Subscribe`.
- Renamed the previous `proxies` / `web_unblocker` modules to `proxy` / `webunblocker` with a fuller, typed API surface.

### 0.2.0

- Activations, Wallet, Catalog, Emails modules; webhook signature verification; typed error mapping; automatic 429 retry.

## License

MIT
