package eveses

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// ResidentialAccess is the white-label connection detail for a user's
// residential (metered/GB) proxy sub-user. Money-adjacent fields are floats
// (GB traffic); the SDK never surfaces provider identity.
type ResidentialAccess struct {
	Host               string           `json:"host"`
	Ports              ResidentialPorts `json:"ports"`
	Username           string           `json:"username"`
	Password           string           `json:"password"`
	Example            string           `json:"example"`
	Curl               string           `json:"curl"`
	TrafficGBAvailable float64          `json:"traffic_gb_available"`
	TrafficGBUsed      float64          `json:"traffic_gb_used"`
}

// ResidentialPorts holds the HTTP / SOCKS5 ports for residential access.
type ResidentialPorts struct {
	HTTP   int `json:"http"`
	Socks5 int `json:"socks5"`
}

// ProxySubscription is the residential auto-renew subscription snapshot.
type ProxySubscription struct {
	Status        string  `json:"status"`
	GB            float64 `json:"gb"`
	DiscountPct   int     `json:"discount_pct"`
	NextRenewsAt  string  `json:"next_renews_at,omitempty"`
	RenewFailures int     `json:"renew_failures"`
}

// ProxyOrder is a single proxy order (residential top-up or a per-IP static
// order). Raw retains the original JSON for forward-compat.
type ProxyOrder struct {
	UUID       string         `json:"uuid"`
	Type       string         `json:"type"`
	Kind       string         `json:"kind"`
	GB         *float64       `json:"gb,omitempty"`
	Quantity   int            `json:"quantity"`
	Location   any            `json:"location,omitempty"`
	Status     string         `json:"status"`
	PriceCents int            `json:"price_cents"`
	Currency   string         `json:"currency"`
	Proxies    []any          `json:"proxies,omitempty"`
	AutoExtend bool           `json:"auto_extend"`
	Extendable bool           `json:"extendable"`
	ExpiresAt  string         `json:"expires_at,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	Raw        map[string]any `json:"-"`
}

// ProxyOverview is the response of ProxiesService.List: the user's residential
// access (nil when none provisioned), residential subscription (nil when none),
// and their recent orders.
type ProxyOverview struct {
	Residential  *ResidentialAccess `json:"residential"`
	Subscription *ProxySubscription `json:"subscription"`
	Orders       []ProxyOrder       `json:"orders"`
}

// ResidentialPackage is one rung of the residential GB price ladder. Raw
// retains any extra fields (e.g. discount_pct) not modelled here.
type ResidentialPackage struct {
	GB          int            `json:"gb"`
	PerGBCents  int            `json:"per_gb_cents"`
	Recommended bool           `json:"recommended,omitempty"`
	Raw         map[string]any `json:"-"`
}

// ProxyPackagesResponse is the response of ProxiesService.Packages.
type ProxyPackagesResponse struct {
	Packages []ResidentialPackage `json:"packages"`
	Currency string               `json:"currency"`
}

// StaticPlan is a plan inside a StaticProduct. PriceCents is nil when the
// provider has no flat price list — call Quote in that case.
type StaticPlan struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	PriceCents  *int   `json:"price_cents"`
	MinQuantity int    `json:"min_quantity"`
	MaxQuantity int    `json:"max_quantity"`
}

// StaticLocation is a location inside a StaticProduct.
type StaticLocation struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	OutOfStock bool   `json:"out_of_stock"`
}

// StaticProduct is a per-IP (ISP/DC/IPv6/Mobile/Sneaker) product with its
// plans and locations.
type StaticProduct struct {
	ID        int              `json:"id"`
	Type      string           `json:"type"`
	Name      string           `json:"name"`
	Plans     []StaticPlan     `json:"plans"`
	Locations []StaticLocation `json:"locations"`
}

// ProxyCatalogResponse is the response of ProxiesService.Catalog.
type ProxyCatalogResponse struct {
	Products []StaticProduct `json:"products"`
	Currency string          `json:"currency"`
}

// ProxyQuote is a public quote. The shape varies by proxy type, so the SDK
// decodes leniently and exposes the full map on Raw. The commonly-present
// fields are promoted for convenience.
type ProxyQuote struct {
	Type        string         `json:"type,omitempty"`
	GB          *float64       `json:"gb,omitempty"`
	Quantity    *int           `json:"quantity,omitempty"`
	PriceCents  int            `json:"price_cents"`
	Currency    string         `json:"currency"`
	DiscountPct *int           `json:"discount_pct,omitempty"`
	PerGBCents  *int           `json:"per_gb_cents,omitempty"`
	Raw         map[string]any `json:"-"`
}

// ProxyLocations is the response of ProxiesService.Locations. For metered
// (residential) types Geo is populated; for static families Products is
// populated. Type echoes the requested proxy type.
type ProxyLocations struct {
	Type     string `json:"type"`
	Geo      any    `json:"geo,omitempty"`
	Products []any  `json:"products,omitempty"`
}

// ProxyUsage is the response of ProxiesService.Usage: a residential usage
// timeline plus the requested window. Raw carries the full timeline object.
type ProxyUsage struct {
	From string         `json:"from"`
	To   string         `json:"to"`
	Raw  map[string]any `json:"-"`
}

// ProxyRegion is one selectable connection region for the shared proxy
// gateway. Code "auto" routes to the nearest region.
type ProxyRegion struct {
	Code  string `json:"code"`
	Host  string `json:"host"`
	Label string `json:"label"`
}

// ProxyEndpointPorts lists the available gateway ports per protocol.
type ProxyEndpointPorts struct {
	HTTP   []int `json:"http"`
	Socks5 []int `json:"socks5"`
}

// ProxyEndpoints is the response of ProxiesService.Endpoints: the connectable
// regions, the ports per protocol, and the supported protocols.
type ProxyEndpoints struct {
	Regions   []ProxyRegion      `json:"regions"`
	Ports     ProxyEndpointPorts `json:"ports"`
	Protocols []string           `json:"protocols"`
}

// ProxiesService wraps /api/account/proxies.
type ProxiesService struct {
	client *Client
}

// List returns the user's residential access, residential subscription and
// their recent proxy orders.
func (s *ProxiesService) List(ctx context.Context) (*ProxyOverview, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/proxies",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out ProxyOverview
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode proxies: " + err.Error()}
		}
	}
	if out.Orders == nil {
		out.Orders = []ProxyOrder{}
	}
	return &out, nil
}

// Endpoints returns the shared proxy gateway connection details: the
// selectable regions, ports per protocol, and supported protocols.
func (s *ProxiesService) Endpoints(ctx context.Context) (*ProxyEndpoints, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/proxies/endpoints",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out ProxyEndpoints
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode proxy endpoints: " + err.Error()}
		}
	}
	if out.Regions == nil {
		out.Regions = []ProxyRegion{}
	}
	if out.Ports.HTTP == nil {
		out.Ports.HTTP = []int{}
	}
	if out.Ports.Socks5 == nil {
		out.Ports.Socks5 = []int{}
	}
	if out.Protocols == nil {
		out.Protocols = []string{}
	}
	return &out, nil
}

// Packages returns the residential GB price ladder.
func (s *ProxiesService) Packages(ctx context.Context) (*ProxyPackagesResponse, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/proxies/packages",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out struct {
		Packages []map[string]any `json:"packages"`
		Currency string           `json:"currency"`
	}
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode proxy packages: " + err.Error()}
		}
	}
	resp := &ProxyPackagesResponse{
		Packages: make([]ResidentialPackage, 0, len(out.Packages)),
		Currency: defaultCurrency(out.Currency),
	}
	for _, m := range out.Packages {
		pkg := ResidentialPackage{Raw: m}
		if v, ok := m["gb"].(float64); ok {
			pkg.GB = int(v)
		}
		if v, ok := m["per_gb_cents"].(float64); ok {
			pkg.PerGBCents = int(v)
		}
		if v, ok := m["recommended"].(bool); ok {
			pkg.Recommended = v
		}
		resp.Packages = append(resp.Packages, pkg)
	}
	return resp, nil
}

// Catalog returns the static (per-IP) product catalogue with user prices.
func (s *ProxiesService) Catalog(ctx context.Context) (*ProxyCatalogResponse, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/proxies/catalog",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out ProxyCatalogResponse
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode proxy catalog: " + err.Error()}
		}
	}
	if out.Products == nil {
		out.Products = []StaticProduct{}
	}
	out.Currency = defaultCurrency(out.Currency)
	return &out, nil
}

// ProxyQuoteParams configures ProxiesService.Quote. For metered/residential
// quotes set Type="residential" (or leave empty) and GB (with optional
// Subscription). For static quotes set Type and the ProductID/PlanID/
// LocationID/Quantity fields.
type ProxyQuoteParams struct {
	// Type is the proxy type ("residential", "isp", "datacenter", "ipv6",
	// "mobile", "sneaker"). Empty defaults to residential server-side.
	Type string
	// GB is the residential traffic amount (metered quotes).
	GB *float64
	// Subscription requests the subscription (discounted) residential quote.
	Subscription bool
	// ProductID/PlanID/LocationID identify a static selection.
	ProductID  *int
	PlanID     *int
	LocationID *int
	// Quantity is the number of static IPs (defaults to 1 server-side).
	Quantity *int
}

// Quote prices a purchase before buying (residential GB or a static selection).
func (s *ProxiesService) Quote(ctx context.Context, p *ProxyQuoteParams) (*ProxyQuote, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	q := url.Values{}
	if p.Type != "" {
		q.Set("type", p.Type)
	}
	if p.GB != nil {
		q.Set("gb", strconv.FormatFloat(*p.GB, 'f', -1, 64))
	}
	if p.Subscription {
		q.Set("subscription", "1")
	}
	if p.ProductID != nil {
		q.Set("product_id", strconv.Itoa(*p.ProductID))
	}
	if p.PlanID != nil {
		q.Set("plan_id", strconv.Itoa(*p.PlanID))
	}
	if p.LocationID != nil {
		q.Set("location_id", strconv.Itoa(*p.LocationID))
	}
	if p.Quantity != nil {
		q.Set("quantity", strconv.Itoa(*p.Quantity))
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/proxies/quote",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	return decodeProxyQuote(raw)
}

// Locations returns available targeting for the given proxy type. For metered
// (residential) types Geo is populated; for static families Products is.
func (s *ProxiesService) Locations(ctx context.Context, proxyType string) (*ProxyLocations, error) {
	q := url.Values{}
	if proxyType != "" {
		q.Set("type", proxyType)
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/proxies/locations",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var out ProxyLocations
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out); err != nil {
			return nil, &Error{Message: "decode proxy locations: " + err.Error()}
		}
	}
	return &out, nil
}

// Usage returns residential usage analytics for the window [from, to]
// (YYYY-MM-DD). Empty strings let the server default the window (last 30 days).
func (s *ProxiesService) Usage(ctx context.Context, from, to string) (*ProxyUsage, error) {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/account/proxies/usage",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	out := &ProxyUsage{}
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &out.Raw); err != nil {
			return nil, &Error{Message: "decode proxy usage: " + err.Error()}
		}
		if v, ok := out.Raw["from"].(string); ok {
			out.From = v
		}
		if v, ok := out.Raw["to"].(string); ok {
			out.To = v
		}
	}
	return out, nil
}

// PurchaseProxyParams configures ProxiesService.Purchase.
//
// Residential top-up: set Type="residential", GB, and optionally Subscription.
// Static (per-IP): set Type to one of "isp"|"datacenter"|"ipv6"|"mobile"|
// "sneaker" plus ProductID/PlanID/LocationID (and optional LocationName /
// Quantity).
//
// When IdempotencyKey is set it is sent as the Idempotency-Key HTTP header.
type PurchaseProxyParams struct {
	Type           string
	GB             *float64
	Subscription   bool
	ProductID      *int
	PlanID         *int
	LocationID     *int
	LocationName   string
	Quantity       *int
	IdempotencyKey string
}

// Purchase buys proxies (residential GB top-up or static IPs) and returns the
// created order (HTTP 201).
func (s *ProxiesService) Purchase(ctx context.Context, p *PurchaseProxyParams) (*ProxyOrder, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	if p.Type == "" {
		return nil, &Error{Message: "Type is required"}
	}

	body := map[string]any{"type": p.Type}
	if p.GB != nil {
		body["gb"] = *p.GB
	}
	if p.Subscription {
		body["subscription"] = true
	}
	if p.ProductID != nil {
		body["product_id"] = *p.ProductID
	}
	if p.PlanID != nil {
		body["plan_id"] = *p.PlanID
	}
	if p.LocationID != nil {
		body["location_id"] = *p.LocationID
	}
	if p.LocationName != "" {
		body["location_name"] = p.LocationName
	}
	if p.Quantity != nil {
		body["quantity"] = *p.Quantity
	}

	headers := map[string]string{}
	if p.IdempotencyKey != "" {
		headers["Idempotency-Key"] = p.IdempotencyKey
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    "/api/account/proxies/purchase",
		body:    body,
		headers: headers,
	}, &raw); err != nil {
		return nil, err
	}
	return decodeProxyOrder(raw)
}

// Extend extends a static (per-IP) order for another period (re-charges its
// price). days defaults to 30 when <= 0.
func (s *ProxiesService) Extend(ctx context.Context, orderUUID string, days int) (*ProxyOrder, error) {
	if orderUUID == "" {
		return nil, &Error{Message: "orderUUID is required"}
	}
	body := map[string]any{}
	if days > 0 {
		body["days"] = days
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/account/proxies/" + url.PathEscape(orderUUID) + "/extend",
		body:   body,
	}, &raw); err != nil {
		return nil, err
	}
	return decodeProxyOrder(raw)
}

// AutoRenew toggles auto-renew (auto_extend) on a per-IP order.
func (s *ProxiesService) AutoRenew(ctx context.Context, orderUUID string, enabled bool) (*ProxyOrder, error) {
	if orderUUID == "" {
		return nil, &Error{Message: "orderUUID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/account/proxies/" + url.PathEscape(orderUUID) + "/auto-renew",
		body:   map[string]any{"enabled": enabled},
	}, &raw); err != nil {
		return nil, err
	}
	return decodeProxyOrder(raw)
}

// CancelSubscription stops the residential subscription's auto-renewal.
func (s *ProxiesService) CancelSubscription(ctx context.Context) (*ProxySubscription, error) {
	return s.subscriptionAction(ctx, "cancel")
}

// PauseSubscription skips residential renewals until resumed.
func (s *ProxiesService) PauseSubscription(ctx context.Context) (*ProxySubscription, error) {
	return s.subscriptionAction(ctx, "pause")
}

// ResumeSubscription resumes the residential subscription (next renewal a
// month out).
func (s *ProxiesService) ResumeSubscription(ctx context.Context) (*ProxySubscription, error) {
	return s.subscriptionAction(ctx, "resume")
}

// ResetSessions resets the current user's residential sticky sessions. The
// next request through the residential proxy rotates to fresh IPs. Requires a
// provisioned residential sub-user (else the API returns 404 no_subuser).
func (s *ProxiesService) ResetSessions(ctx context.Context) error {
	var raw json.RawMessage
	return s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/account/proxies/sessions/reset",
	}, &raw)
}

func (s *ProxiesService) subscriptionAction(ctx context.Context, action string) (*ProxySubscription, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/account/proxies/subscription/" + action,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var sub ProxySubscription
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &sub); err != nil {
			return nil, &Error{Message: "decode proxy subscription: " + err.Error()}
		}
	}
	return &sub, nil
}

// decodeProxyOrder unwraps a {"data": {...}} envelope, decodes the order, and
// retains the raw JSON on Order.Raw.
func decodeProxyOrder(raw json.RawMessage) (*ProxyOrder, error) {
	inner := unwrapData(raw)
	if len(inner) == 0 {
		return &ProxyOrder{}, nil
	}
	var order ProxyOrder
	if err := json.Unmarshal(inner, &order); err != nil {
		return nil, &Error{Message: "decode proxy order: " + err.Error()}
	}
	if order.Currency == "" {
		order.Currency = "USD"
	}
	var rawMap map[string]any
	if err := json.Unmarshal(inner, &rawMap); err == nil {
		order.Raw = rawMap
	}
	return &order, nil
}

// decodeProxyQuote unwraps the envelope and decodes a lenient quote, retaining
// the full map on Quote.Raw.
func decodeProxyQuote(raw json.RawMessage) (*ProxyQuote, error) {
	inner := unwrapData(raw)
	if len(inner) == 0 {
		return &ProxyQuote{Currency: "USD"}, nil
	}
	var quote ProxyQuote
	if err := json.Unmarshal(inner, &quote); err != nil {
		return nil, &Error{Message: "decode proxy quote: " + err.Error()}
	}
	if quote.Currency == "" {
		quote.Currency = "USD"
	}
	var rawMap map[string]any
	if err := json.Unmarshal(inner, &rawMap); err == nil {
		quote.Raw = rawMap
	}
	return &quote, nil
}

// defaultCurrency returns c or "USD" when c is empty.
func defaultCurrency(c string) string {
	if c == "" {
		return "USD"
	}
	return c
}
