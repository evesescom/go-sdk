package eveses

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// OrderMode is "activation" (default) or "rent".
type OrderMode string

const (
	OrderModeActivation OrderMode = "activation"
	OrderModeRent       OrderMode = "rent"
)

// Order is the on-the-wire shape of a single number order returned by
// /api/v1/numbers/orders/*. Snake_case wire fields are translated to
// idiomatic Go names; Raw retains the original JSON for forward-compat.
type Order struct {
	OrderID    string         `json:"order_id"`
	Status     string         `json:"status"`
	Phone      string         `json:"phone,omitempty"`
	Country    string         `json:"country,omitempty"`
	Service    string         `json:"service,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	PriceCents *int           `json:"price_cents,omitempty"`
	ExpiresAt  string         `json:"expires_at,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	Raw        map[string]any `json:"-"`
}

// OrderSms is a single SMS message attached to an order.
type OrderSms struct {
	ID         int    `json:"id"`
	Text       string `json:"text"`
	Sender     string `json:"sender,omitempty"`
	ReceivedAt string `json:"received_at,omitempty"`
}

// OrderSmsBundle combines the webhook-stored and on-demand-fresh SMS lists
// for an order.
type OrderSmsBundle struct {
	OrderID string     `json:"order_id"`
	Stored  []OrderSms `json:"stored"`
	Fresh   []OrderSms `json:"fresh"`
}

// CreateNumberParams is the input to NumbersService.Create.
//
// Country and Service are required. When IdempotencyKey is set, it is sent
// both in the JSON body (for legacy server-side dedup) and as the
// `Idempotency-Key` HTTP header.
type CreateNumberParams struct {
	// Country is an ISO 3166-1 alpha-2 code, lowercased ("ua", "pl"). Required.
	Country string
	// Service is the upstream service code ("telegram", "wa"). Required.
	Service string
	// Mode defaults to "activation" when empty.
	Mode OrderMode
	// DurationMinutes is required for rent mode (>= 1).
	DurationMinutes *int
	// IdempotencyKey is sent both in body and as the Idempotency-Key header.
	IdempotencyKey string
	// MaxPriceCents bounds the highest acceptable price in cents.
	MaxPriceCents *int
}

// NumbersCountriesParams configures Numbers.Countries. Mode defaults to
// "activation" when empty.
type NumbersCountriesParams struct {
	Mode OrderMode
}

// NumbersProductsParams configures Numbers.Products. Country and Currency
// are accepted for symmetry with the wider catalog API but are currently
// informational on the v1 endpoint, which returns the unified product list.
type NumbersProductsParams struct {
	Mode     OrderMode
	Country  string
	Currency string
}

// NumbersPricingParams configures Numbers.Pricing. Country and Service are
// required; the endpoint expects `product=<service>` on the wire — the SDK
// translates the friendlier `Service` name.
type NumbersPricingParams struct {
	Country         string
	Service         string
	Mode            OrderMode
	Currency        string
	DurationMinutes *int
}

// NumbersCountriesResponse is the response of Numbers.Countries.
type NumbersCountriesResponse struct {
	Mode      string   `json:"mode"`
	Countries []string `json:"countries"`
}

// NumbersProductsResponse is the response of Numbers.Products.
type NumbersProductsResponse struct {
	Mode     string   `json:"mode"`
	Products []string `json:"products"`
	Country  string   `json:"country,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

// NumbersPricingDuration is a single price/duration combination inside
// NumbersPricingResponse.Services[].Durations.
type NumbersPricingDuration struct {
	DurationMinutes int            `json:"duration_minutes"`
	PriceCents      *int           `json:"price_cents,omitempty"`
	Price           *float64       `json:"price,omitempty"`
	Currency        string         `json:"currency,omitempty"`
	Available       *bool          `json:"available,omitempty"`
	Raw             map[string]any `json:"-"`
}

// NumbersServiceWithDurations is a single service entry inside
// NumbersPricingResponse.
type NumbersServiceWithDurations struct {
	Name      string                   `json:"name"`
	Durations []NumbersPricingDuration `json:"durations"`
}

// NumbersPricingResponse is the response of Numbers.Pricing. The API returns
// a list of services even when one was requested.
type NumbersPricingResponse struct {
	Mode     string                        `json:"mode"`
	Country  string                        `json:"country"`
	Services []NumbersServiceWithDurations `json:"services"`
	Currency string                        `json:"currency,omitempty"`
	Service  string                        `json:"service,omitempty"`
}

// NumbersService wraps /api/v1/numbers — the merged number-ordering
// (activations + rent) and catalog surface.
//
// Orders: Create, Get, Sms, Cancel, Finish, Retry, Repeat, AutoRenew,
// CreateBatch. Catalog reads: Pricing, Countries, Products, Carriers, States.
type NumbersService struct {
	client *Client
}

const numbersOrdersPath = "/api/v1/numbers/orders"

// Create provisions a number for a country/service. Returns the created order.
func (s *NumbersService) Create(ctx context.Context, p *CreateNumberParams) (*Order, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	if p.Country == "" {
		return nil, &Error{Message: "Country is required"}
	}
	if p.Service == "" {
		return nil, &Error{Message: "Service is required"}
	}

	mode := string(p.Mode)
	if mode == "" {
		mode = string(OrderModeActivation)
	}

	body := map[string]any{
		"mode":    mode,
		"country": p.Country,
		"service": p.Service,
	}
	if p.DurationMinutes != nil {
		body["duration_minutes"] = *p.DurationMinutes
	}
	if p.IdempotencyKey != "" {
		body["idempotency_key"] = p.IdempotencyKey
	}
	if p.MaxPriceCents != nil {
		body["max_price_cents"] = *p.MaxPriceCents
	}

	headers := map[string]string{}
	if p.IdempotencyKey != "" {
		headers["Idempotency-Key"] = p.IdempotencyKey
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    numbersOrdersPath,
		body:    body,
		headers: headers,
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// CreateBatch provisions several numbers in one call via
// POST /api/v1/numbers/orders/batch. Each item is a CreateNumberParams; the
// server returns one order per requested item.
func (s *NumbersService) CreateBatch(ctx context.Context, items []*CreateNumberParams) ([]*Order, error) {
	if len(items) == 0 {
		return nil, &Error{Message: "at least one item is required"}
	}
	orders := make([]map[string]any, 0, len(items))
	for _, p := range items {
		if p == nil {
			return nil, &Error{Message: "batch item must not be nil"}
		}
		if p.Country == "" {
			return nil, &Error{Message: "Country is required for every batch item"}
		}
		if p.Service == "" {
			return nil, &Error{Message: "Service is required for every batch item"}
		}
		mode := string(p.Mode)
		if mode == "" {
			mode = string(OrderModeActivation)
		}
		item := map[string]any{
			"mode":    mode,
			"country": p.Country,
			"service": p.Service,
		}
		if p.DurationMinutes != nil {
			item["duration_minutes"] = *p.DurationMinutes
		}
		if p.MaxPriceCents != nil {
			item["max_price_cents"] = *p.MaxPriceCents
		}
		orders = append(orders, item)
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   numbersOrdersPath + "/batch",
		body:   map[string]any{"orders": orders},
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrderList(raw)
}

// Get fetches an order by its UUID.
func (s *NumbersService) Get(ctx context.Context, orderID string) (*Order, error) {
	return s.orderGet(ctx, "GET", orderID, "")
}

// Cancel releases the number and refunds the user (where supported).
func (s *NumbersService) Cancel(ctx context.Context, orderID string) (*Order, error) {
	return s.orderGet(ctx, "POST", orderID, "/cancel")
}

// Finish marks the order completed once the SMS has been consumed.
func (s *NumbersService) Finish(ctx context.Context, orderID string) (*Order, error) {
	return s.orderGet(ctx, "POST", orderID, "/finish")
}

// Retry requests a new attempt on a failed / stuck order.
func (s *NumbersService) Retry(ctx context.Context, orderID string) (*Order, error) {
	return s.orderGet(ctx, "POST", orderID, "/retry")
}

// Repeat re-orders the same country/service as an existing order.
func (s *NumbersService) Repeat(ctx context.Context, orderID string) (*Order, error) {
	return s.orderGet(ctx, "POST", orderID, "/repeat")
}

// AutoRenew toggles auto-renew on a rent order.
func (s *NumbersService) AutoRenew(ctx context.Context, orderID string, enabled bool) (*Order, error) {
	if orderID == "" {
		return nil, &Error{Message: "orderID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   numbersOrdersPath + "/" + url.PathEscape(orderID) + "/auto-renew",
		body:   map[string]any{"enabled": enabled},
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// Sms returns all SMS messages for an order, combining `stored` (delivered
// to us via webhook) with `fresh` (pulled from the upstream provider on demand).
func (s *NumbersService) Sms(ctx context.Context, orderID string) (*OrderSmsBundle, error) {
	if orderID == "" {
		return nil, &Error{Message: "orderID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   numbersOrdersPath + "/" + url.PathEscape(orderID) + "/sms",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var bundle OrderSmsBundle
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &bundle); err != nil {
			return nil, &Error{Message: "decode sms bundle: " + err.Error()}
		}
	}
	if bundle.OrderID == "" {
		bundle.OrderID = orderID
	}
	if bundle.Stored == nil {
		bundle.Stored = []OrderSms{}
	}
	if bundle.Fresh == nil {
		bundle.Fresh = []OrderSms{}
	}
	return &bundle, nil
}

func (s *NumbersService) orderGet(ctx context.Context, method, orderID, suffix string) (*Order, error) {
	if orderID == "" {
		return nil, &Error{Message: "orderID is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: method,
		path:   numbersOrdersPath + "/" + url.PathEscape(orderID) + suffix,
	}, &raw); err != nil {
		return nil, err
	}
	return decodeOrder(raw)
}

// Countries lists ISO-3166-1 alpha-2 country codes that have stock for mode.
func (s *NumbersService) Countries(ctx context.Context, p *NumbersCountriesParams) (*NumbersCountriesResponse, error) {
	var requested OrderMode
	if p != nil {
		requested = p.Mode
	}
	mode := defaultMode(requested)

	q := url.Values{}
	q.Set("mode", mode)

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/numbers/countries",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var probe struct {
		Mode      string   `json:"mode"`
		Countries []string `json:"countries"`
	}
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &probe); err != nil {
			return nil, &Error{Message: "decode countries: " + err.Error()}
		}
	}
	if probe.Mode == "" {
		probe.Mode = mode
	}
	if probe.Countries == nil {
		probe.Countries = []string{}
	}
	return &NumbersCountriesResponse{Mode: probe.Mode, Countries: probe.Countries}, nil
}

// Products lists service / product codes available globally for mode.
func (s *NumbersService) Products(ctx context.Context, p *NumbersProductsParams) (*NumbersProductsResponse, error) {
	var requested OrderMode
	if p != nil {
		requested = p.Mode
	}
	mode := defaultMode(requested)

	q := url.Values{}
	q.Set("mode", mode)

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/numbers/products",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var probe struct {
		Mode     string   `json:"mode"`
		Products []string `json:"products"`
	}
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &probe); err != nil {
			return nil, &Error{Message: "decode products: " + err.Error()}
		}
	}
	if probe.Mode == "" {
		probe.Mode = mode
	}
	products := probe.Products
	if products == nil {
		products = []string{}
	}

	resp := &NumbersProductsResponse{
		Mode:     probe.Mode,
		Products: products,
	}
	if p != nil {
		if p.Country != "" {
			resp.Country = strings.ToLower(p.Country)
		}
		if p.Currency != "" {
			resp.Currency = strings.ToUpper(p.Currency)
		}
	}
	return resp, nil
}

// Carriers returns available carriers/operators for a country, optionally
// filtered by mode. Returns the decoded JSON object as a map (forward-compat).
func (s *NumbersService) Carriers(ctx context.Context, country string, mode OrderMode) (map[string]any, error) {
	q := url.Values{}
	if country != "" {
		q.Set("country", strings.ToLower(country))
	}
	q.Set("mode", defaultMode(mode))
	return s.getMap(ctx, "/api/v1/numbers/carriers", q)
}

// States returns available states/regions for a country. Returns the decoded
// JSON object as a map (forward-compat).
func (s *NumbersService) States(ctx context.Context, country string, mode OrderMode) (map[string]any, error) {
	q := url.Values{}
	if country != "" {
		q.Set("country", strings.ToLower(country))
	}
	q.Set("mode", defaultMode(mode))
	return s.getMap(ctx, "/api/v1/numbers/states", q)
}

// Pricing fetches pricing for a country/service pair, optionally filtered
// by duration_minutes. The wire param `product=` is sent as `service` on
// the SDK surface.
func (s *NumbersService) Pricing(ctx context.Context, p *NumbersPricingParams) (*NumbersPricingResponse, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	if p.Country == "" {
		return nil, &Error{Message: "Country is required"}
	}
	if p.Service == "" {
		return nil, &Error{Message: "Service is required"}
	}

	mode := string(p.Mode)
	if mode == "" {
		mode = string(OrderModeActivation)
	}

	q := url.Values{}
	q.Set("mode", mode)
	q.Set("country", strings.ToLower(p.Country))
	q.Set("product", p.Service)
	if p.Currency != "" {
		q.Set("currency", strings.ToUpper(p.Currency))
	}
	if p.DurationMinutes != nil {
		q.Set("duration", strconv.Itoa(*p.DurationMinutes))
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/numbers/pricing",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	type rawDuration struct {
		DurationMinutes int      `json:"duration_minutes"`
		PriceCents      *int     `json:"price_cents,omitempty"`
		Price           *float64 `json:"price,omitempty"`
		Currency        string   `json:"currency,omitempty"`
		Available       *bool    `json:"available,omitempty"`
		InStock         *bool    `json:"in_stock,omitempty"`
	}
	type rawService struct {
		Name      string        `json:"name"`
		Durations []rawDuration `json:"durations"`
	}
	var probe struct {
		Mode     string       `json:"mode"`
		Country  string       `json:"country"`
		Currency string       `json:"currency,omitempty"`
		Services []rawService `json:"services"`
	}
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &probe); err != nil {
			return nil, &Error{Message: "decode pricing: " + err.Error()}
		}
	}

	if probe.Mode == "" {
		probe.Mode = mode
	}
	if probe.Country == "" {
		probe.Country = strings.ToLower(p.Country)
	}
	if probe.Currency == "" && p.Currency != "" {
		probe.Currency = strings.ToUpper(p.Currency)
	}

	// Extract per-duration raw blobs in a second pass for forward-compat.
	var rawProbe struct {
		Services []map[string]any `json:"services"`
	}
	_ = json.Unmarshal(inner, &rawProbe)

	out := &NumbersPricingResponse{
		Mode:     probe.Mode,
		Country:  probe.Country,
		Currency: probe.Currency,
		Service:  p.Service,
		Services: make([]NumbersServiceWithDurations, 0, len(probe.Services)),
	}

	for i, svc := range probe.Services {
		mapped := NumbersServiceWithDurations{
			Name:      svc.Name,
			Durations: make([]NumbersPricingDuration, 0, len(svc.Durations)),
		}
		var rawDurations []any
		if i < len(rawProbe.Services) {
			if rs, ok := rawProbe.Services[i]["durations"].([]any); ok {
				rawDurations = rs
			}
		}
		for j, d := range svc.Durations {
			available := d.Available
			if available == nil && d.InStock != nil {
				available = d.InStock
			}
			mapped.Durations = append(mapped.Durations, NumbersPricingDuration{
				DurationMinutes: d.DurationMinutes,
				PriceCents:      d.PriceCents,
				Price:           d.Price,
				Currency:        d.Currency,
				Available:       available,
				Raw:             rawDurationAt(rawDurations, j),
			})
		}
		out.Services = append(out.Services, mapped)
	}
	return out, nil
}

func (s *NumbersService) getMap(ctx context.Context, path string, query url.Values) (map[string]any, error) {
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

// defaultMode normalises an OrderMode to its string form, defaulting to
// "activation" when empty.
func defaultMode(m OrderMode) string {
	if m == "" {
		return string(OrderModeActivation)
	}
	return string(m)
}

func rawDurationAt(rawDurations []any, idx int) map[string]any {
	if idx < 0 || idx >= len(rawDurations) {
		return nil
	}
	if m, ok := rawDurations[idx].(map[string]any); ok {
		return m
	}
	return nil
}

// decodeOrder unwraps {"data": {...}} envelopes and decodes the order. The
// raw JSON is also retained on Order.Raw for callers that need fields the
// SDK doesn't model yet.
func decodeOrder(raw json.RawMessage) (*Order, error) {
	inner := unwrapData(raw)
	if len(inner) == 0 {
		return &Order{}, nil
	}

	var order Order
	if err := json.Unmarshal(inner, &order); err != nil {
		return nil, &Error{Message: "decode order: " + err.Error()}
	}

	var rawMap map[string]any
	if err := json.Unmarshal(inner, &rawMap); err == nil {
		order.Raw = rawMap
	}
	return &order, nil
}

// decodeOrderList unwraps a {"data": [...]} / {"orders": [...]} envelope (or a
// naked array) and decodes each element into an *Order.
func decodeOrderList(raw json.RawMessage) ([]*Order, error) {
	arr := unwrapArray(raw)
	if len(arr) == 0 {
		return []*Order{}, nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(arr, &elems); err != nil {
		return nil, &Error{Message: "decode order list: " + err.Error()}
	}
	out := make([]*Order, 0, len(elems))
	for _, e := range elems {
		o, err := decodeOrder(e)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}
