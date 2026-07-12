package eveses

import (
	"context"
	"encoding/json"
)

// Me is the authenticated identity returned by GET /api/v1/me. Alongside the
// existing profile fields (retained on Raw), v0.4.0 surfaces the token's
// Abilities and the account's product Features so callers stop hardcoding flags.
type Me struct {
	ID       any    `json:"id,omitempty"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Currency string `json:"currency,omitempty"`
	// Abilities lists what THIS token can do (e.g. ["*"]).
	Abilities []string `json:"abilities,omitempty"`
	// Features gates product entry points (trial/proxy/webunblocker/emails/captcha).
	Features map[string]bool `json:"features,omitempty"`
	// Raw retains the full payload for fields the SDK doesn't model yet.
	Raw map[string]any `json:"-"`
}

// MeService wraps /api/v1/me — the authenticated identity + token abilities +
// product features.
type MeService struct {
	client *Client
}

// Get returns the authenticated identity, including the new abilities and
// features fields.
func (s *MeService) Get(ctx context.Context) (*Me, error) {
	var raw json.RawMessage
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/me",
	}, &raw); err != nil {
		return nil, err
	}
	inner := unwrapData(raw)

	var me Me
	if len(inner) > 0 {
		if err := json.Unmarshal(inner, &me); err != nil {
			return nil, &Error{Message: "decode me: " + err.Error()}
		}
		var rawMap map[string]any
		if err := json.Unmarshal(inner, &rawMap); err == nil {
			me.Raw = rawMap
		}
	}
	return &me, nil
}

// Loyalty returns the caller's loyalty tier snapshot via
// GET /api/v1/me/loyalty. Returns the decoded JSON object as a map.
func (s *MeService) Loyalty(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   "/api/v1/me/loyalty",
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
