# `github.com/evesescom/go-sdk` — examples

Three runnable programs that exercise the SDK end-to-end. Each lives in
its own subdirectory with `package main` so they compile independently.

| Path | What it shows |
| --- | --- |
| `examples/quickstart/` | Construct the client, check wallet balance, list services, buy ONE activation with an idempotency key. |
| `examples/buy-and-poll/` | Full activation lifecycle: create → poll SMS every 5s for 5 min → `Finish()` (or `Cancel()` on Ctrl-C / timeout). |
| `examples/webhook-server/` | Minimal `net/http` server that verifies `X-Eveses-Signature` with `eveses.VerifyWebhook` and prints the parsed payload. |
| `examples/products/` | Browse the proxies, web-unblocker and emails product lines (list + quote, read-only). |

## Prerequisites

```bash
cd sdk/go
go mod download                          # resolves the SDK module

# Get a Sanctum API-key token (kind=api_key) from the Eveses dashboard.
export EVESES_API_KEY=sk_live_xxx

# For the webhook server only:
export EVESES_WEBHOOK_SECRET=whsec_xxx
```

Run any example:

```bash
go run ./examples/quickstart
go run ./examples/buy-and-poll
go run ./examples/webhook-server
go run ./examples/products
```

Build them all to check for compilation errors:

```bash
for d in examples/*/; do go build -o /dev/null ./"$d"; done
```
