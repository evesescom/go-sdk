package eveses

import "context"

// Fingerprint is a generated browser fingerprint returned by the fingerprints
// service.
type Fingerprint struct {
	// Payload is the raw fingerprint object as returned by the provider.
	Payload map[string]any `json:"fingerprint"`
	// PriceMicroUSD is what this fingerprint cost, in µUSD (1e-6 USD).
	PriceMicroUSD int `json:"price_micro_usd,omitempty"`
}

// FingerprintsService wraps /api/account/fingerprints.
//
// It resells 2captcha's Fingerprint API, billed pay-per-use from the wallet
// (count-on-success). Unlike captcha-solving this is synchronous: one request
// returns a complete fingerprint.
type FingerprintsService struct {
	client *Client
}

type fingerprintResponse struct {
	Fingerprint   map[string]any `json:"fingerprint"`
	PriceMicroUSD int            `json:"price_micro_usd"`
}

// Generate creates a browser fingerprint from the given filter params (format,
// tags, country, build_version, min_browser_version, …).
func (s *FingerprintsService) Generate(ctx context.Context, params map[string]any) (*Fingerprint, error) {
	return s.request(ctx, "/api/account/fingerprints/generate", params)
}

// Random fetches a random fingerprint, optionally narrowed by filter params.
func (s *FingerprintsService) Random(ctx context.Context, params map[string]any) (*Fingerprint, error) {
	return s.request(ctx, "/api/account/fingerprints/random", params)
}

func (s *FingerprintsService) request(ctx context.Context, path string, params map[string]any) (*Fingerprint, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if params == nil {
		params = map[string]any{}
	}

	var res fingerprintResponse
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   path,
		body:   params,
	}, &res); err != nil {
		return nil, err
	}

	return &Fingerprint{Payload: res.Fingerprint, PriceMicroUSD: res.PriceMicroUSD}, nil
}
