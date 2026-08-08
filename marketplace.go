package eveses

import (
	"context"
	"net/url"
)

// MarketplaceService wraps the account marketplace.
//
// The marketplace hides the upstream provider behind a normalized catalog:
// attributes are standardized (country ISO-2 uppercase or a region slug —
// mix/cis/eu/asia/africa/latam; origin autoreg|selfreg|real|retrieve;
// format tdata|session_json|session; twofa bool). With group_by=attributes
// the catalog collapses SKUs into groups carrying prices_cents.
//
// Read/browse endpoints live under /api/public/marketplace; quote, buy, and
// order management under /api/v1/marketplace.
type MarketplaceService struct {
	client *Client
}

const (
	marketplacePublicBase = "/api/public/marketplace"
	marketplaceBase       = "/api/v1/marketplace"
	marketplaceOrdersPath = marketplaceBase + "/orders"
)

// MarketplaceCatalogParams filters MarketplaceService.Catalog. All fields are
// optional; only non-empty params are sent.
type MarketplaceCatalogParams struct {
	Category string
	Country  string
	Origin   string
	Format   string
	// Twofa filters by the two-factor-auth flag when non-nil.
	Twofa *bool
	// GroupBy is sent as group_by; "country" or "attributes". With
	// "attributes" the catalog returns groups carrying prices_cents.
	GroupBy string
}

// MarketplaceBuyParams configures MarketplaceService.Buy.
type MarketplaceBuyParams struct {
	Category string
	SKU      string
	Quantity int
	// Inputs carries any SKU-specific purchase inputs.
	Inputs map[string]any
	// IdempotencyKey — replays return the same order via the Idempotency-Key
	// header instead of a new purchase.
	IdempotencyKey string
}

// Catalog lists marketplace offers, optionally filtered/grouped. Accepts nil
// opts. GET /api/public/marketplace/catalog.
func (s *MarketplaceService) Catalog(ctx context.Context, opts *MarketplaceCatalogParams) (map[string]any, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Category != "" {
			q.Set("category", opts.Category)
		}
		if opts.Country != "" {
			q.Set("country", opts.Country)
		}
		if opts.Origin != "" {
			q.Set("origin", opts.Origin)
		}
		if opts.Format != "" {
			q.Set("format", opts.Format)
		}
		if opts.Twofa != nil {
			if *opts.Twofa {
				q.Set("twofa", "true")
			} else {
				q.Set("twofa", "false")
			}
		}
		if opts.GroupBy != "" {
			q.Set("group_by", opts.GroupBy)
		}
	}
	return s.getMap(ctx, marketplacePublicBase+"/catalog", q)
}

// Categories lists the available marketplace categories.
// GET /api/public/marketplace/categories.
func (s *MarketplaceService) Categories(ctx context.Context) (map[string]any, error) {
	return s.getMap(ctx, marketplacePublicBase+"/categories", nil)
}

// Filters lists the available filter facets, optionally scoped to a category.
// GET /api/public/marketplace/filters.
func (s *MarketplaceService) Filters(ctx context.Context, category string) (map[string]any, error) {
	q := url.Values{}
	if category != "" {
		q.Set("category", category)
	}
	return s.getMap(ctx, marketplacePublicBase+"/filters", q)
}

// Quote estimates the price for a category/sku before buying.
// POST /api/v1/marketplace/quote.
func (s *MarketplaceService) Quote(ctx context.Context, category, sku string) (map[string]any, error) {
	return s.postMap(ctx, marketplaceBase+"/quote", map[string]any{
		"category": category,
		"sku":      sku,
	}, nil)
}

// Buy purchases a marketplace SKU. POST /api/v1/marketplace/buy.
func (s *MarketplaceService) Buy(ctx context.Context, p *MarketplaceBuyParams) (map[string]any, error) {
	if p == nil {
		return nil, &Error{Message: "params is required"}
	}
	body := map[string]any{
		"category": p.Category,
		"sku":      p.SKU,
		"quantity": p.Quantity,
	}
	if p.Inputs != nil {
		body["inputs"] = p.Inputs
	}

	headers := map[string]string{}
	if p.IdempotencyKey != "" {
		headers["Idempotency-Key"] = p.IdempotencyKey
	}
	return s.postMap(ctx, marketplaceBase+"/buy", body, headers)
}

// Orders lists the user's marketplace orders.
// GET /api/v1/marketplace/orders.
func (s *MarketplaceService) Orders(ctx context.Context) (map[string]any, error) {
	return s.getMap(ctx, marketplaceOrdersPath, nil)
}

// Order returns a single marketplace order by UUID.
// GET /api/v1/marketplace/orders/{uuid}.
func (s *MarketplaceService) Order(ctx context.Context, uuid string) (map[string]any, error) {
	return s.getMap(ctx, marketplaceOrdersPath+"/"+url.PathEscape(uuid), nil)
}

// Reveal discloses the delivered secret payload for a marketplace order.
// POST /api/v1/marketplace/orders/{uuid}/reveal.
func (s *MarketplaceService) Reveal(ctx context.Context, uuid string) (map[string]any, error) {
	return s.postMap(ctx, marketplaceOrdersPath+"/"+url.PathEscape(uuid)+"/reveal", nil, nil)
}

// getMap issues a GET and returns the decoded JSON object as a map.
func (s *MarketplaceService) getMap(ctx context.Context, path string, query url.Values) (map[string]any, error) {
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

// postMap issues a POST (with optional JSON body + headers) and returns the
// decoded JSON object as a map.
func (s *MarketplaceService) postMap(ctx context.Context, path string, body any, headers map[string]string) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    path,
		body:    body,
		headers: headers,
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
