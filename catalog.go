package eveses

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// CatalogCountriesParams configures Catalog.Countries. Mode defaults to
// "activation" when empty.
type CatalogCountriesParams struct {
	Mode OrderMode
}

// CatalogServicesParams configures Catalog.Services. Country and Currency
// are accepted for symmetry with the wider catalog API but are currently
// informational on the v1 endpoint, which returns the unified product list.
type CatalogServicesParams struct {
	Mode     OrderMode
	Country  string
	Currency string
}

// CatalogPricingParams configures Catalog.Pricing. Country and Service are
// required; the endpoint expects `product=<service>` on the wire — the SDK
// translates the friendlier `Service` name.
type CatalogPricingParams struct {
	Country         string
	Service         string
	Mode            OrderMode
	Currency        string
	DurationMinutes *int
}

// CatalogCountriesResponse is the response of Catalog.Countries.
type CatalogCountriesResponse struct {
	Mode      string   `json:"mode"`
	Countries []string `json:"countries"`
}

// CatalogServicesResponse is the response of Catalog.Services.
type CatalogServicesResponse struct {
	Mode     string   `json:"mode"`
	Services []string `json:"services"`
	Country  string   `json:"country,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

// CatalogPricingDuration is a single price/duration combination inside
// CatalogPricingResponse.Services[].Durations.
type CatalogPricingDuration struct {
	DurationMinutes int            `json:"duration_minutes"`
	PriceCents      *int           `json:"price_cents,omitempty"`
	Price           *float64       `json:"price,omitempty"`
	Currency        string         `json:"currency,omitempty"`
	Available       *bool          `json:"available,omitempty"`
	Raw             map[string]any `json:"-"`
}

// CatalogServiceWithDurations is a single service entry inside
// CatalogPricingResponse.
type CatalogServiceWithDurations struct {
	Name      string                   `json:"name"`
	Durations []CatalogPricingDuration `json:"durations"`
}

// CatalogPricingResponse is the response of Catalog.Pricing. The API returns
// a list of services even when one was requested.
type CatalogPricingResponse struct {
	Mode     string                        `json:"mode"`
	Country  string                        `json:"country"`
	Services []CatalogServiceWithDurations `json:"services"`
	Currency string                        `json:"currency,omitempty"`
	Service  string                        `json:"service,omitempty"`
}

// CatalogService wraps /api/v1/numbers/{countries,products,pricing}.
type CatalogService struct {
	client *Client
}

// Countries lists ISO-3166-1 alpha-2 country codes that have stock for mode.
func (s *CatalogService) Countries(ctx context.Context, p *CatalogCountriesParams) (*CatalogCountriesResponse, error) {
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
	return &CatalogCountriesResponse{Mode: probe.Mode, Countries: probe.Countries}, nil
}

// Services lists service / product codes available globally for mode.
func (s *CatalogService) Services(ctx context.Context, p *CatalogServicesParams) (*CatalogServicesResponse, error) {
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

	// The wire field is "products"; we expose it as "services".
	var probe struct {
		Mode     string   `json:"mode"`
		Products []string `json:"products"`
	}
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &probe); err != nil {
			return nil, &Error{Message: "decode services: " + err.Error()}
		}
	}
	if probe.Mode == "" {
		probe.Mode = mode
	}
	services := probe.Products
	if services == nil {
		services = []string{}
	}

	resp := &CatalogServicesResponse{
		Mode:     probe.Mode,
		Services: services,
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

// Pricing fetches pricing for a country/service pair, optionally filtered
// by duration_minutes. The wire param `product=` is sent as `service` on
// the SDK surface.
func (s *CatalogService) Pricing(ctx context.Context, p *CatalogPricingParams) (*CatalogPricingResponse, error) {
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

	out := &CatalogPricingResponse{
		Mode:     probe.Mode,
		Country:  probe.Country,
		Currency: probe.Currency,
		Service:  p.Service,
		Services: make([]CatalogServiceWithDurations, 0, len(probe.Services)),
	}

	for i, svc := range probe.Services {
		mapped := CatalogServiceWithDurations{
			Name:      svc.Name,
			Durations: make([]CatalogPricingDuration, 0, len(svc.Durations)),
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
			mapped.Durations = append(mapped.Durations, CatalogPricingDuration{
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
