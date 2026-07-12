package eveses

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// ProxyType is a proxy family. Residential is metered (per-GB); the rest are
// static (per-IP).
type ProxyType string

const (
	ProxyTypeResidential ProxyType = "residential"
	ProxyTypeISP         ProxyType = "isp"
	ProxyTypeDatacenter  ProxyType = "datacenter"
	ProxyTypeIPv6        ProxyType = "ipv6"
	ProxyTypeSneaker     ProxyType = "sneaker"
	ProxyTypeMobile      ProxyType = "mobile"
)

// ProxyStaticSelection identifies a per-IP product/plan/location to quote or buy.
type ProxyStaticSelection struct {
	ProductID  int
	PlanID     int
	LocationID int
	// LocationName is an optional human label stored on the order (buy only).
	LocationName string
	// Quantity is the number of IPs to buy (buy only; default 1).
	Quantity int
}

// ProxyQuote is a pre-purchase price estimate returned by Proxy.Quote.
type ProxyQuote struct {
	Raw map[string]any `json:"-"`
}

// ProxyOrder is a proxy order returned by Proxy.Buy / List / Get / Extend /
// AutoRenew. The user-facing payload never leaks the provider cost.
type ProxyOrder struct {
	UUID       string         `json:"uuid"`
	Type       string         `json:"type"`
	Kind       string         `json:"kind,omitempty"`
	GB         *float64       `json:"gb,omitempty"`
	Quantity   *int           `json:"quantity,omitempty"`
	Location   string         `json:"location,omitempty"`
	Status     string         `json:"status"`
	PriceCents int            `json:"price_cents"`
	Currency   string         `json:"currency,omitempty"`
	Proxies    any            `json:"proxies,omitempty"`
	AutoExtend bool           `json:"auto_extend"`
	Extendable bool           `json:"extendable"`
	ExpiresAt  string         `json:"expires_at,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	Raw        map[string]any `json:"-"`
}

// ProxySubuser is the residential sub-user connection block from Proxy.List.
type ProxySubuser struct {
	Host               string         `json:"host"`
	Ports              map[string]any `json:"ports"`
	Username           string         `json:"username"`
	Password           string         `json:"password"`
	Example            string         `json:"example"`
	Curl               string         `json:"curl"`
	TrafficGBAvailable float64        `json:"traffic_gb_available"`
	TrafficGBUsed      float64        `json:"traffic_gb_used"`
}

// ProxySubscription is the residential subscription block.
type ProxySubscription struct {
	Status        string  `json:"status"`
	GB            float64 `json:"gb"`
	DiscountPct   int     `json:"discount_pct"`
	NextRenewsAt  string  `json:"next_renews_at,omitempty"`
	RenewFailures int     `json:"renew_failures"`
}

// ProxyList is the response of Proxy.List — the user's residential connection,
// subscription, and per-IP orders.
type ProxyList struct {
	Residential  *ProxySubuser      `json:"residential"`
	Subscription *ProxySubscription `json:"subscription"`
	Orders       []ProxyOrder       `json:"orders"`
}

const proxyBase = "/api/v1/proxy"
const proxyOrdersPath = proxyBase + "/orders"

// ProxyQuoteParams configures Proxy.Quote.
type ProxyQuoteParams struct {
	Type ProxyType
	// GB is the residential top-up size (metered types only).
	GB float64
	// Subscription requests the subscription price (residential only).
	Subscription bool
	// Selection identifies the static product/plan/location (per-IP types only).
	Selection *ProxyStaticSelection
	// Quantity is the number of IPs to quote (per-IP types only; default 1).
	Quantity int
}

// ProxyPurchaseParams configures Proxy.Buy.
type ProxyPurchaseParams struct {
	Type ProxyType
	// GB is the residential top-up size (metered types only).
	GB float64
	// Subscription starts a monthly auto-renewing subscription (residential only).
	Subscription bool
	// Selection identifies the static product/plan/location (per-IP types only).
	Selection *ProxyStaticSelection
	// IdempotencyKey — replays return the same order instead of a new purchase.
	IdempotencyKey string
}

// ProxyService wraps /api/v1/proxy.
//
// It covers the residential (metered, per-GB) and static (per-IP) proxy
// surface: read pricing, quote, buy, list, show, extend, auto-renew, reset
// sessions, usage analytics, and residential subscription control.
type ProxyService struct {
	client *Client
}

// Pricing returns all proxy prices — residential GB ladder + static per-IP
// catalogue. Replaces the old `packages` / `catalog` verbs.
func (s *ProxyService) Pricing(ctx context.Context) (map[string]any, error) {
	return s.getMap(ctx, proxyBase+"/pricing", nil)
}

// Endpoints returns the white-label connection endpoints: regional entry
// subdomains + the HTTP / SOCKS5 ports users can connect on.
func (s *ProxyService) Endpoints(ctx context.Context) (map[string]any, error) {
	return s.getMap(ctx, proxyBase+"/endpoints", nil)
}

// Quotas returns the remaining prepaid proxy balances (per provider).
func (s *ProxyService) Quotas(ctx context.Context) (map[string]any, error) {
	return s.getMap(ctx, proxyBase+"/quotas", nil)
}

// Locations returns available targeting for a proxy type: residential geo
// (countries/regions/sets) or a static family's catalogue locations. Type
// defaults to residential when empty.
func (s *ProxyService) Locations(ctx context.Context, proxyType ProxyType) (map[string]any, error) {
	q := url.Values{}
	if proxyType != "" {
		q.Set("type", string(proxyType))
	}
	return s.getMap(ctx, proxyBase+"/locations", q)
}

// Quote estimates a purchase before buying (residential GB or a static
// selection). Type defaults to residential when empty.
func (s *ProxyService) Quote(ctx context.Context, p *ProxyQuoteParams) (*ProxyQuote, error) {
	if p == nil {
		p = &ProxyQuoteParams{}
	}
	proxyType := p.Type
	if proxyType == "" {
		proxyType = ProxyTypeResidential
	}

	q := url.Values{}
	q.Set("type", string(proxyType))
	if proxyType == ProxyTypeResidential {
		q.Set("gb", strconv.FormatFloat(p.GB, 'f', -1, 64))
		if p.Subscription {
			q.Set("subscription", "true")
		}
	} else if p.Selection != nil {
		q.Set("product_id", strconv.Itoa(p.Selection.ProductID))
		q.Set("plan_id", strconv.Itoa(p.Selection.PlanID))
		q.Set("location_id", strconv.Itoa(p.Selection.LocationID))
		quantity := p.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		q.Set("quantity", strconv.Itoa(quantity))
	}

	raw, err := s.getMap(ctx, proxyBase+"/quote", q)
	if err != nil {
		return nil, err
	}
	return &ProxyQuote{Raw: raw}, nil
}

// Buy buys proxies (residential GB top-up or static IPs) via
// POST /api/v1/proxy/orders. Returns the created order.
func (s *ProxyService) Buy(ctx context.Context, p *ProxyPurchaseParams) (*ProxyOrder, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	proxyType := p.Type
	if proxyType == "" {
		proxyType = ProxyTypeResidential
	}

	body := map[string]any{"type": string(proxyType)}
	if proxyType == ProxyTypeResidential {
		body["gb"] = p.GB
		if p.Subscription {
			body["subscription"] = true
		}
	} else if p.Selection != nil {
		body["product_id"] = p.Selection.ProductID
		body["plan_id"] = p.Selection.PlanID
		body["location_id"] = p.Selection.LocationID
		if p.Selection.LocationName != "" {
			body["location_name"] = p.Selection.LocationName
		}
		quantity := p.Selection.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		body["quantity"] = quantity
	}

	headers := map[string]string{}
	if p.IdempotencyKey != "" {
		headers["Idempotency-Key"] = p.IdempotencyKey
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    proxyOrdersPath,
		body:    body,
		headers: headers,
	}, &raw); err != nil {
		return nil, err
	}
	return parseProxyOrder(raw)
}

// List returns the user's proxy orders (residential connection, subscription,
// and per-IP orders) via GET /api/v1/proxy/orders.
func (s *ProxyService) List(ctx context.Context) (*ProxyList, error) {
	var out ProxyList
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   proxyOrdersPath,
	}, &out); err != nil {
		return nil, err
	}
	if out.Orders == nil {
		out.Orders = []ProxyOrder{}
	}
	return &out, nil
}

// Get returns a single proxy order by UUID via
// GET /api/v1/proxy/orders/{uuid}.
func (s *ProxyService) Get(ctx context.Context, orderUUID string) (*ProxyOrder, error) {
	if orderUUID == "" {
		return nil, &Error{Message: "orderUUID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   proxyOrdersPath + "/" + url.PathEscape(orderUUID),
	}, &raw); err != nil {
		return nil, err
	}
	return parseProxyOrder(unwrapData(raw))
}

// Extend renews a static (per-IP) order for another period (re-charges its
// price) via POST /api/v1/proxy/orders/{uuid}/extend. Days defaults to 30
// when <= 0.
func (s *ProxyService) Extend(ctx context.Context, orderUUID string, days int) (*ProxyOrder, error) {
	body := map[string]any{}
	if days > 0 {
		body["days"] = days
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   proxyOrdersPath + "/" + url.PathEscape(orderUUID) + "/extend",
		body:   body,
	}, &raw); err != nil {
		return nil, err
	}
	return parseProxyOrder(raw)
}

// AutoRenew toggles auto-renew (auto_extend) on a per-IP order via
// POST /api/v1/proxy/orders/{uuid}/auto-renew.
func (s *ProxyService) AutoRenew(ctx context.Context, orderUUID string, enabled bool) (*ProxyOrder, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   proxyOrdersPath + "/" + url.PathEscape(orderUUID) + "/auto-renew",
		body:   map[string]any{"enabled": enabled},
	}, &raw); err != nil {
		return nil, err
	}
	return parseProxyOrder(raw)
}

// ResetSessions resets the user's residential sticky sessions (next request
// rotates IPs).
func (s *ProxyService) ResetSessions(ctx context.Context) error {
	return s.client.do(ctx, requestOptions{
		method: "POST",
		path:   proxyBase + "/sessions/reset",
	}, nil)
}

// Usage returns residential usage analytics — daily traffic/requests timeline
// + top hosts. from/to are YYYY-MM-DD; empty values fall back to the API's
// default 30-day window.
func (s *ProxyService) Usage(ctx context.Context, from, to string) (map[string]any, error) {
	q := url.Values{}
	q.Set("from", from)
	q.Set("to", to)
	return s.getMap(ctx, proxyBase+"/usage", q)
}

// SubscriptionCancel stops the residential subscription's auto-renewal
// (traffic stays).
func (s *ProxyService) SubscriptionCancel(ctx context.Context) (*ProxySubscription, error) {
	return s.subscriptionAction(ctx, "cancel")
}

// SubscriptionPause skips residential renewals until resumed.
func (s *ProxyService) SubscriptionPause(ctx context.Context) (*ProxySubscription, error) {
	return s.subscriptionAction(ctx, "pause")
}

// SubscriptionResume resumes the residential subscription (next renewal a
// month out).
func (s *ProxyService) SubscriptionResume(ctx context.Context) (*ProxySubscription, error) {
	return s.subscriptionAction(ctx, "resume")
}

func (s *ProxyService) subscriptionAction(ctx context.Context, action string) (*ProxySubscription, error) {
	var sub ProxySubscription
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   proxyBase + "/subscription/" + action,
	}, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// Trial activates the free proxy trial allocation (one-time).
func (s *ProxyService) Trial(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   proxyBase + "/trial",
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// getMap issues a GET and returns the decoded JSON object as a map.
func (s *ProxyService) getMap(ctx context.Context, path string, query url.Values) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   path,
		query:  query,
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func parseProxyOrder(raw json.RawMessage) (*ProxyOrder, error) {
	var order ProxyOrder
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &order); err != nil {
			return nil, &Error{Message: "decode proxy order: " + err.Error()}
		}
		var rawMap map[string]any
		if err := json.Unmarshal(raw, &rawMap); err == nil {
			order.Raw = rawMap
		}
	}
	if order.Currency == "" {
		order.Currency = "USD"
	}
	return &order, nil
}
