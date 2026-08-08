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
order, _ := client.Numbers.Create(ctx, &eveses.CreateNumberParams{Country: "ua", Service: "telegram"})
sms, _   := client.Numbers.Sms(ctx, order.OrderID)
bal, _   := client.Wallet.Balance(ctx)
prods, _ := client.Numbers.Products(ctx, &eveses.NumbersProductsParams{Mode: eveses.OrderModeActivation, Country: "ua"})
```

> **v0.4.0 note:** all request paths moved to `/api/v1/*`. The old `Activations`
> + `Catalog` modules are now the single `Numbers` module; `Proxy.Purchase` /
> `WebUnblocker.Purchase` / `Emails.Purchase` are now `Buy`; and the
> `Fingerprints` module has been removed. See the [changelog](#040).

`EVESES_API_KEY` should be a Sanctum API token (kind=`api_key`) issued from your Eveses dashboard. Never commit it — read it from the environment, a secret manager, or your platform's config store.

## Numbers

The merged number-ordering (activations + rent) and catalog module — hits
`/api/v1/numbers/*`.

```go
import "context"

ctx := context.Background()
order, err := client.Numbers.Create(ctx, &eveses.CreateNumberParams{
    Country:        "ua",
    Service:        "telegram",
    IdempotencyKey: "abc-123", // sent as Idempotency-Key header
})

// Poll for SMS
bundle, _ := client.Numbers.Sms(ctx, order.OrderID)
for _, m := range bundle.Stored { fmt.Println(m.Text) }
for _, m := range bundle.Fresh  { fmt.Println(m.Text) }

// Order actions
_, _ = client.Numbers.Finish(ctx, order.OrderID)
_, _ = client.Numbers.Cancel(ctx, order.OrderID)
_, _ = client.Numbers.Retry(ctx, order.OrderID)
_, _ = client.Numbers.Repeat(ctx, order.OrderID)
_, _ = client.Numbers.AutoRenew(ctx, order.OrderID, true) // rent orders
again, _ := client.Numbers.Get(ctx, order.OrderID)
_ = again

// Buy several at once
orders, _ := client.Numbers.CreateBatch(ctx, []*eveses.CreateNumberParams{
    {Country: "ua", Service: "telegram"},
    {Country: "pl", Service: "wa"},
})
_ = orders

// Catalog reads
countries, _ := client.Numbers.Countries(ctx, &eveses.NumbersCountriesParams{Mode: eveses.OrderModeActivation})
products,  _ := client.Numbers.Products(ctx,  &eveses.NumbersProductsParams{Mode: eveses.OrderModeActivation, Country: "ua"})
carriers,  _ := client.Numbers.Carriers(ctx, "ua", eveses.OrderModeActivation)
states,    _ := client.Numbers.States(ctx, "us", eveses.OrderModeActivation)
_, _, _, _ = countries, products, carriers, states

dur := 60
pricing, _ := client.Numbers.Pricing(ctx, &eveses.NumbersPricingParams{
    Country:         "ua",
    Service:         "telegram",
    Mode:            eveses.OrderModeRent,
    DurationMinutes: &dur,
})
_ = pricing
```

The `Service` field on `NumbersPricingParams` is the friendlier name for the wire param `product`; the SDK translates it for you. Same for `DurationMinutes` → `duration`.

## Wallet

```go
bal, err := client.Wallet.Balance(ctx)
fmt.Println(bal.AvailableBalance, bal.Currency) // e.g. 4800 USD (cents)
```

## Proxy

Residential (metered, per-GB) and static (per-IP: ISP, datacenter, IPv6, sneaker, mobile) proxies. Read the catalogue, quote, buy, list, extend, auto-renew, reset sessions, view usage, and manage the residential subscription.

All paths hit `/api/v1/proxy/*` (singular). Orders live under `/orders`.

```go
// Pricing / connection info
prices, _ := client.Proxy.Pricing(ctx)                                  // residential GB ladder + static catalogue
eps, _    := client.Proxy.Endpoints(ctx)                                // white-label entry subdomains + ports
locs, _   := client.Proxy.Locations(ctx, eveses.ProxyTypeResidential)   // targeting
quotas, _ := client.Proxy.Quotas(ctx)                                   // remaining prepaid GB

// Quote + buy residential GB
q, _ := client.Proxy.Quote(ctx, &eveses.ProxyQuoteParams{Type: eveses.ProxyTypeResidential, GB: 5})
order, _ := client.Proxy.Buy(ctx, &eveses.ProxyPurchaseParams{
    Type: eveses.ProxyTypeResidential, GB: 5, IdempotencyKey: "abc-123",
})

// Or buy static IPs via a product/plan/location selection
_, _ = client.Proxy.Buy(ctx, &eveses.ProxyPurchaseParams{
    Type:      eveses.ProxyTypeISP,
    Selection: &eveses.ProxyStaticSelection{ProductID: 1, PlanID: 2, LocationID: 3, Quantity: 2},
})

list, _ := client.Proxy.List(ctx)                      // GET /orders — residential + subscription + per-IP orders
one, _  := client.Proxy.Get(ctx, order.UUID)           // GET /orders/{uuid}
_ = one
_, _ = client.Proxy.Extend(ctx, order.UUID, 30)        // POST /orders/{uuid}/extend
_, _ = client.Proxy.AutoRenew(ctx, order.UUID, true)   // POST /orders/{uuid}/auto-renew
_ = client.Proxy.ResetSessions(ctx)                    // rotate residential sticky sessions
usage, _ := client.Proxy.Usage(ctx, "2026-06-01", "2026-06-30")
_, _ = client.Proxy.Trial(ctx)                         // one-time free trial

// Residential subscription lifecycle
_, _ = client.Proxy.SubscriptionCancel(ctx)
_, _ = client.Proxy.SubscriptionPause(ctx)
_, _ = client.Proxy.SubscriptionResume(ctx)
```

## Marketplace

Browse and buy from the account marketplace. Read/browse routes are public
(`/api/public/marketplace/*`); quote, buy, and order management are
authenticated (`/api/v1/marketplace/*`). The catalog normalizes the upstream
provider: attributes are standardized (`country`, `origin`, `format`, `twofa`)
and `GroupBy = "attributes"` collapses same-type products into groups carrying
`prices_cents` variants.

```go
cats, _    := client.Marketplace.Categories(ctx)          // available categories
filters, _ := client.Marketplace.Filters(ctx, "accounts") // facets for a category
_, _ = cats, filters

catalog, _ := client.Marketplace.Catalog(ctx, &eveses.MarketplaceCatalogParams{
    Category: "accounts",
    Country:  "US",
    Origin:   "autoreg",
    GroupBy:  "attributes",
})
_ = catalog

// Quote + buy a SKU
q, _ := client.Marketplace.Quote(ctx, "accounts", "some-sku")
order, _ := client.Marketplace.Buy(ctx, &eveses.MarketplaceBuyParams{
    Category: "accounts", SKU: "some-sku", Quantity: 1, IdempotencyKey: "abc-123",
})
_ = q

// List orders, fetch one, reveal the delivered secret payload
orders, _ := client.Marketplace.Orders(ctx)
one, _    := client.Marketplace.Order(ctx, "b1f2-…-uuid")
secret, _ := client.Marketplace.Reveal(ctx, order["uuid"].(string))
_, _, _ = orders, one, secret
```

## WebUnblocker

Request-metered Web Unblocker access — hits `/api/v1/webunblocker/*` (no
hyphen). Pricing, quoting, buying, listing, trial, access/quota checks, and
subscription lifecycle.

```go
prices, _ := client.WebUnblocker.Pricing(ctx)
q, _    := client.WebUnblocker.Quote(ctx, 10000, false)              // requests, subscription
order, _ := client.WebUnblocker.Buy(ctx, 10000, false, "abc-123")    // requests, subscription, idempotencyKey — POST /orders
orders, _ := client.WebUnblocker.List(ctx)                           // GET /orders
_, _ = orders, prices
_, _ = client.WebUnblocker.Trial(ctx)                                // one-time free trial

access, _ := client.WebUnblocker.Access(ctx)                         // credentials + quota + orders
fmt.Println(access.Host, access.Port, access.Username, access.RequestsLeft)

_, _ = client.WebUnblocker.SubscriptionCancel(ctx)
_, _ = client.WebUnblocker.SubscriptionPause(ctx)
_, _ = client.WebUnblocker.SubscriptionResume(ctx)
```

## Emails

Rent disposable email mailboxes and read their inbound messages — hits
`/api/v1/emails/*`. Inbox routes are keyed on the email address.

```go
prices, _  := client.Emails.Pricing(ctx, "")                          // optional site filter; domains under the `domains` key
q, _       := client.Emails.Quote(ctx, "example.com", "site", "provider")
order, _   := client.Emails.Buy(ctx, "example.com", "site", "provider", "abc-123") // POST /orders
_ = prices

orders, _  := client.Emails.List(ctx, false)                          // GET /orders — includeReleased
mailbox, _ := client.Emails.Get(ctx, order.Email)                     // GET /{email}
msgs, _    := client.Emails.Messages(ctx, order.Email, 1, 20)         // email, page, perPage
for _, m := range msgs { fmt.Println(m.From, m.Subject) }

_ = client.Emails.MarkRead(ctx, order.Email, msgs[0].ID)
_ = client.Emails.Release(ctx, order.Email)                           // delete the mailbox
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

// Per-solve price list and task history
rates, _ := client.Captcha.Rates(ctx)                       // GET /api/v1/captcha/rates
usage, _ := client.Captcha.Usage(ctx, &eveses.CaptchaUsageParams{Limit: 50})
_ = rates
for _, t := range usage.Data { fmt.Println(t.ID, t.Type, t.Status, t.CostMicroUSD) }
```

## Orders (global)

The unified cross-product order feed — `/api/v1/orders`. Captcha is not here
(see `Captcha.Usage`).

```go
page, _ := client.Orders.List(ctx, &eveses.OrdersListParams{
    Service: []string{"proxy", "numbers"}, Limit: 20,
})
for _, o := range page.Data {
    fmt.Println(o.Source, o.ID, o.Status, o.AmountCents, o.Title)
}
if page.Meta.HasMore { /* fetch next with Cursor: page.Meta.NextCursor */ }

one, _ := client.Orders.Get(ctx, "b1f2-…-uuid")            // OrderView for any product
_ = one
```

## Pricing (aggregate)

Every product's prices in one call — `/api/v1/pricing`.

```go
p, _ := client.Pricing.All(ctx)
fmt.Println(p.Currency, p.Numbers, p.Proxy, p.WebUnblocker, p.Emails, p.Captcha)
```

## Quotas

Remaining prepaid balances — `/api/v1/quotas`. Only products with a
decrementing counter appear.

```go
q, _ := client.Quotas.Get(ctx)
for _, t := range q.Trial        { fmt.Println("trial", t.Service, t.Remaining) }
for _, t := range q.Proxy        { fmt.Println("proxy", t.Provider, t.Remaining) }
for _, t := range q.WebUnblocker { fmt.Println("wu", t.Provider, t.Remaining) }
```

## Me

The authenticated identity plus the token's abilities and product features —
`/api/v1/me`.

```go
me, _ := client.Me.Get(ctx)
fmt.Println(me.Email, me.Abilities)          // e.g. ["*"]
if me.Features["proxy"] { /* show the proxy product entry point */ }

loyalty, _ := client.Me.Loyalty(ctx)         // GET /api/v1/me/loyalty
_ = loyalty
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
- **Idempotency**: when `IdempotencyKey` is set on `CreateNumberParams` (and the proxy/webunblocker/emails buy helpers), it is sent both in the JSON body and as the `Idempotency-Key` HTTP header.
- **Envelope unwrap**: responses of the form `{"data": {...}}` are auto-unwrapped; raw `{...}` objects are accepted too.
- **Concurrent use**: `*Client` is safe to share across goroutines.

## Changelog

### 0.5.0

- **NEW `Marketplace`** (`client.Marketplace`) — browse (`Catalog`, `Categories`, `Filters`) and purchase (`Quote`, `Buy`, `Orders`, `Order`, `Reveal`) the account marketplace. The public catalog supports attribute filters (`country` / `origin` / `format` / `twofa`) and `GroupBy = "country" | "attributes"` — with `"attributes"` same-type products collapse into groups carrying `prices_cents` variants.
- **`Proxy.LocationsDetail`** (`client.Proxy.LocationsDetail(ctx, country, proxyType)`) — per-country residential state/city/ISP geo drill-down.
- Default user-agent bumped to `eveses-go/0.5.0`.

### 0.4.0

- **Moved every request path to `/api/v1/*`** (was `/api/account/*` / a v1 mix). The base URL is unchanged.
- **`Numbers`** (`client.Numbers`) — the old `Activations` + `Catalog` modules merged into one, hitting `/api/v1/numbers/*`: orders (`Create`, `Get`, `Sms`, `Cancel`, `Finish`, `Retry`, `Repeat`, `AutoRenew`, `CreateBatch`) and catalog reads (`Pricing`, `Countries`, `Products`, `Carriers`, `States`).
- **`WebUnblocker`** renamed on the wire to `/api/v1/webunblocker/*` (no hyphen); `Purchase` → `Buy` (`POST /orders`), added `List` (`GET /orders`), `Packages` → `Pricing`.
- **`Proxy`** repointed to `/api/v1/proxy/*` (singular); `Purchase` → `Buy` (`POST /orders`), `List`/`Get` under `/orders`, extend/auto-renew under `/orders/{uuid}`, `Packages`/`Catalog` → `Pricing`, added `Quotas`.
- **`Emails`** repointed to `/api/v1/emails/*`; `Purchase` → `Buy` (`POST /orders`), `List` (`GET /orders`), `Domains` → `Pricing` (domains under the `domains` key); inbox routes keyed on the email address.
- **`Captcha`** — kept `Solve`/`Rates`; added `Usage` (`GET /api/v1/captcha/usage`).
- **NEW `Orders`** (`client.Orders`) — unified cross-product order feed (`GET /api/v1/orders` + `/{uuid}`), returning the normalized `OrderView`.
- **NEW `Pricing`** (`client.Pricing`) — aggregate price list (`GET /api/v1/pricing`).
- **NEW `Quotas`** (`client.Quotas`) — remaining prepaid balances (`GET /api/v1/quotas`).
- **NEW `Me`** (`client.Me`) — `GET /api/v1/me` now carries `Abilities` + `Features`; also `Loyalty`.
- **REMOVED `Fingerprints`** — the whole product is gone (module, tests, examples, wiring).

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
