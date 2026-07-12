package eveses

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// OrderView is the normalized cross-product order shape returned by the global
// order feed (GET /api/v1/orders and /api/v1/orders/{uuid}).
type OrderView struct {
	Source      string         `json:"source"` // numbers | proxy | webunblocker | emails
	ID          string         `json:"id"`
	Status      string         `json:"status"` // pending|active|awaiting|completed|expired|canceled|failed
	AmountCents int            `json:"amount_cents"`
	Currency    string         `json:"currency,omitempty"`
	Title       string         `json:"title,omitempty"`
	CreatedAt   string         `json:"created_at,omitempty"`
	ExpiresAt   string         `json:"expires_at,omitempty"`
	DetailURL   string         `json:"detail_url,omitempty"`
	Raw         map[string]any `json:"-"`
}

// OrdersMeta is the cursor pagination meta on a global order page.
type OrdersMeta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// OrdersResponse is the response of Orders.List — a cursor-paginated,
// newest-first global order history across every product.
type OrdersResponse struct {
	Data []OrderView `json:"data"`
	Meta OrdersMeta  `json:"meta"`
}

// OrdersListParams are the optional filters for Orders.List.
type OrdersListParams struct {
	// Service filters by one or more of numbers|proxy|webunblocker|emails.
	Service []string
	// Status filters by a canonical status.
	Status string
	// CreatedGte / CreatedLte bound the created_at window (RFC3339 or date).
	CreatedGte string
	CreatedLte string
	// Cursor / Limit drive pagination (limit default 20, max 100).
	Cursor string
	Limit  int
}

// OrdersService wraps /api/v1/orders — the unified cross-product order feed.
//
// Captcha is NOT here (it is usage; see Captcha.Usage).
type OrdersService struct {
	client *Client
}

// List returns the cursor-paginated global order history. params may be nil.
func (s *OrdersService) List(ctx context.Context, params *OrdersListParams) (*OrdersResponse, error) {
	q := url.Values{}
	if params != nil {
		if len(params.Service) > 0 {
			q.Set("service", strings.Join(params.Service, ","))
		}
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.CreatedGte != "" {
			q.Set("created[gte]", params.CreatedGte)
		}
		if params.CreatedLte != "" {
			q.Set("created[lte]", params.CreatedLte)
		}
		if params.Cursor != "" {
			q.Set("cursor", params.Cursor)
		}
		if params.Limit > 0 {
			q.Set("limit", strconv.Itoa(params.Limit))
		}
	}

	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/orders",
		query:  q,
	}, &raw); err != nil {
		return nil, err
	}

	var resp OrdersResponse
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, &Error{Message: "decode orders: " + err.Error()}
		}
	}
	if resp.Data == nil {
		resp.Data = []OrderView{}
	}
	// Attach per-item raw blobs for forward-compat.
	var rawProbe struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &rawProbe); err == nil {
		for i := range resp.Data {
			if i < len(rawProbe.Data) {
				resp.Data[i].Raw = rawProbe.Data[i]
			}
		}
	}
	return &resp, nil
}

// Get returns the normalized OrderView for a single order of any product via
// GET /api/v1/orders/{uuid}.
func (s *OrdersService) Get(ctx context.Context, uuid string) (*OrderView, error) {
	if uuid == "" {
		return nil, &Error{Message: "uuid is required"}
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/orders/" + url.PathEscape(uuid),
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var view OrderView
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &view); err != nil {
			return nil, &Error{Message: "decode order: " + err.Error()}
		}
		var rawMap map[string]any
		if err := json.Unmarshal(inner, &rawMap); err == nil {
			view.Raw = rawMap
		}
	}
	return &view, nil
}
