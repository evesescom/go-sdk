package eveses

import "context"

// TrialStatus is the current trial state returned by Trial.Status.
type TrialStatus struct {
	Active    bool     `json:"active"`
	Services  []string `json:"services,omitempty"`
	ExpiresAt string   `json:"expires_at,omitempty"`
}

// TrialService wraps /api/v1/trial.
//
// It lets callers inspect the active trial state and subscribe to one or more
// trial-eligible services.
type TrialService struct {
	client *Client
}

// Status returns the account's current trial status and which services are
// covered.
func (s *TrialService) Status(ctx context.Context) (*TrialStatus, error) {
	var out TrialStatus
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/trial",
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Subscribe opts the account into a trial for the given services. services is
// the list of product slugs to enable (e.g. []string{"web-unblocker",
// "proxies"}).
func (s *TrialService) Subscribe(ctx context.Context, services []string) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "POST",
		path:   "/api/v1/trial/subscribe",
		body:   map[string]any{"services": services},
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
