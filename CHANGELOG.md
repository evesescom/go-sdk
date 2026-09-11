# Changelog — github.com/evesescom/go-sdk

All notable changes to this SDK. Versions follow the API's own
[changelog](https://eveses.com/changelog/api); the SDK version and the API
version are deliberately not the same number.

## 0.7.0

### Added

- **Billing.** `client.Billing` covers the invoicing surface: read and update the billing
  profile, list, create, fetch and cancel invoices, download an invoice as a
  PDF, and raise an invoice against a deposit that has already settled.
- **Binary responses.** Downloading an invoice PDF returns bytes, not text.
  A `*[]byte` out-param receives the raw body.

## 0.6.0

### Added

- **Proxy paid extras.** `extra_requirements` (and its display labels) are
  carried on both the proxy quote and the proxy purchase, so an add-on a
  buyer paid for reaches the supplier instead of being dropped in transit.

## 0.5.1

### Changed

- README documents the marketplace module.

## 0.5.0

### Added

- **Marketplace module** — catalogue, quote, buy, orders, and credential reveal.
- **Proxy per-country geo drill-down** — state, city and ISP under a country.

## 0.4.0

### Changed

- **Moved to the `/api/v1` surface.** The `/api/account/*` prefix is gone.
- The numbers modules were merged into one.

### Added

- `orders`, `pricing`, `quotas` and account endpoints.

### Removed

- **Fingerprints**, withdrawn from the product with no replacement.

## 0.3.0

### Added

- Proxy, web-unblocker, emails, trial and captcha modules.
