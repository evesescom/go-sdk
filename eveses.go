// Package eveses is the official Go SDK for the Eveses API.
//
// Quickstart:
//
//	client, err := eveses.New(eveses.Config{APIKey: os.Getenv("EVESES_API_KEY")})
//	if err != nil { log.Fatal(err) }
//
//	order, err := client.Numbers.Create(ctx, &eveses.CreateNumberParams{
//	    Country: "ua", Service: "telegram",
//	})
//	bal, err := client.Wallet.Balance(ctx)
//
// Webhooks:
//
//	ok, err := eveses.VerifyWebhook(rawBody, sigHeader, tsHeader, secret, 300*time.Second)
package eveses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production API root.
	DefaultBaseURL = "https://api.eveses.io"
	// DefaultTimeout matches the JS / Python / PHP SDKs.
	DefaultTimeout = 30 * time.Second
	// DefaultUserAgent identifies SDK + version on every request.
	DefaultUserAgent = "eveses-go/0.5.0"
	// Version is the SDK semver.
	Version = "0.5.0"
)

// Config configures a Client. Only APIKey is required; the rest fall back
// to sane production defaults.
type Config struct {
	// APIKey is the Sanctum API token (kind=api_key). Required.
	APIKey string
	// BaseURL overrides the API root. Trailing slashes are stripped.
	BaseURL string
	// Timeout caps each individual HTTP request (default 30s). Ignored
	// when HTTPClient is supplied — set the timeout on the http.Client
	// directly in that case.
	Timeout time.Duration
	// HTTPClient lets callers inject a custom transport (test servers,
	// proxies, instrumented clients). When nil a fresh *http.Client with
	// Timeout is used.
	HTTPClient *http.Client
	// UserAgent overrides the default UA string.
	UserAgent string
	// DefaultHeaders are merged into every request (after the SDK's own
	// auth/accept headers, before per-request overrides).
	DefaultHeaders map[string]string
}

// Client is the top-level SDK entrypoint. Service handles hang off it.
//
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	httpClient     *http.Client
	baseURL        string
	apiKey         string
	userAgent      string
	defaultHeaders map[string]string

	Numbers      *NumbersService
	Wallet       *WalletService
	Captcha      *CaptchaService
	Proxy        *ProxyService
	Marketplace  *MarketplaceService
	WebUnblocker *WebUnblockerService
	Emails       *EmailsService
	Trial        *TrialService
	Orders       *OrdersService
	Pricing      *PricingService
	Quotas       *QuotasService
	Me           *MeService
}

// New constructs a Client from Config. Returns an error iff APIKey is empty.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, &Error{Message: "APIKey is required", StatusCode: 0}
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = DefaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}

	c := &Client{
		httpClient:     httpClient,
		baseURL:        baseURL,
		apiKey:         cfg.APIKey,
		userAgent:      ua,
		defaultHeaders: copyHeaders(cfg.DefaultHeaders),
	}
	c.Numbers = &NumbersService{client: c}
	c.Wallet = &WalletService{client: c}
	c.Captcha = &CaptchaService{client: c}
	c.Proxy = &ProxyService{client: c}
	c.Marketplace = &MarketplaceService{client: c}
	c.WebUnblocker = &WebUnblockerService{client: c}
	c.Emails = &EmailsService{client: c}
	c.Trial = &TrialService{client: c}
	c.Orders = &OrdersService{client: c}
	c.Pricing = &PricingService{client: c}
	c.Quotas = &QuotasService{client: c}
	c.Me = &MeService{client: c}
	return c, nil
}

// requestOptions is the internal request descriptor used by every service.
type requestOptions struct {
	method  string
	path    string
	query   url.Values
	body    any
	headers map[string]string
}

// do issues an authenticated request and decodes the JSON response into out
// (which may be nil to discard). Performs one automatic retry on 429 honouring
// Retry-After (capped at 60s).
func (c *Client) do(ctx context.Context, opts requestOptions, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var bodyBytes []byte
	if opts.body != nil {
		b, err := json.Marshal(opts.body)
		if err != nil {
			return &Error{Message: "marshal request body: " + err.Error()}
		}
		bodyBytes = b
	}

	resp, raw, err := c.executeWithRetry(ctx, opts, bodyBytes, 0)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return &Error{
				Message:    "decode response: " + err.Error(),
				StatusCode: resp.StatusCode,
			}
		}
		return nil
	}

	return c.mapError(resp, raw)
}

func (c *Client) executeWithRetry(
	ctx context.Context,
	opts requestOptions,
	body []byte,
	attempt int,
) (*http.Response, []byte, error) {
	req, err := c.buildRequest(ctx, opts, body)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Surface ctx cancellation as a transport-level *Error.
		return nil, nil, &Error{Message: "network error: " + err.Error()}
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, nil, &Error{
			Message:    "read response body: " + readErr.Error(),
			StatusCode: resp.StatusCode,
		}
	}

	if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
		wait := parseRetryAfter(resp.Header.Get("Retry-After"))
		select {
		case <-ctx.Done():
			return nil, nil, &Error{Message: "context cancelled during 429 backoff: " + ctx.Err().Error()}
		case <-time.After(wait):
		}
		return c.executeWithRetry(ctx, opts, body, attempt+1)
	}

	return resp, raw, nil
}

func (c *Client) buildRequest(ctx context.Context, opts requestOptions, body []byte) (*http.Request, error) {
	full, err := c.buildURL(opts.path, opts.query)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, opts.method, full, reader)
	if err != nil {
		return nil, &Error{Message: "build request: " + err.Error()}
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.defaultHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range opts.headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

func (c *Client) buildURL(path string, query url.Values) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", &Error{Message: "build URL: " + err.Error()}
	}
	if len(query) > 0 {
		// Drop empty values so omitted optional params don't appear as `key=`.
		filtered := url.Values{}
		for k, vs := range query {
			for _, v := range vs {
				if v == "" {
					continue
				}
				filtered.Add(k, v)
			}
		}
		u.RawQuery = filtered.Encode()
	}
	return u.String(), nil
}

func (c *Client) mapError(resp *http.Response, raw []byte) error {
	parsed, message := parseErrorBody(resp, raw)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return newAuthError(message, parsed)
	case http.StatusForbidden:
		return &Error{Message: message, StatusCode: 403, Code: "forbidden", Body: parsed}
	case http.StatusNotFound:
		return &Error{Message: message, StatusCode: 404, Code: "not_found", Body: parsed}
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return &Error{Message: message, StatusCode: resp.StatusCode, Code: "validation_failed", Body: parsed}
	case http.StatusTooManyRequests:
		return newRateLimitError(message, parseRetryAfter(resp.Header.Get("Retry-After")), parsed)
	}
	if resp.StatusCode >= 500 {
		return &Error{Message: message, StatusCode: resp.StatusCode, Code: "server_error", Body: parsed}
	}
	return &Error{Message: message, StatusCode: resp.StatusCode, Body: parsed}
}

// parseErrorBody decodes JSON when the content-type advertises it, falling back
// to the response status text for the error message.
func parseErrorBody(resp *http.Response, raw []byte) (any, string) {
	var parsed any
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") && len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parsed = string(raw)
		}
	} else if len(raw) > 0 {
		parsed = string(raw)
	}

	message := extractMessage(parsed)
	if message == "" {
		if resp.Status != "" {
			// resp.Status is "200 OK" — strip the leading code if present.
			message = stripStatusCode(resp.Status)
		} else {
			message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}
	return parsed, message
}

func extractMessage(body any) string {
	m, ok := body.(map[string]any)
	if !ok {
		return ""
	}
	if msg, ok := m["message"].(string); ok && msg != "" {
		return msg
	}
	if errStr, ok := m["error"].(string); ok && errStr != "" {
		return errStr
	}
	return ""
}

func stripStatusCode(status string) string {
	parts := strings.SplitN(status, " ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return status
}

// parseRetryAfter parses the Retry-After header. Returns a default of 1s if
// missing/invalid, capped at 60s otherwise. HTTP-date form is not supported
// (matches JS/Python).
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return time.Second
	}
	secs, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || secs < 0 {
		return time.Second
	}
	if secs > 60 {
		secs = 60
	}
	return time.Duration(secs) * time.Second
}

// unwrapData returns payload["data"] when payload is a {"data": {...}} envelope,
// else payload itself. All Eveses endpoints either return naked JSON objects
// or the {data: ...} envelope; both are treated identically by the SDK.
func unwrapData(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return raw
	}
	if data, ok := probe["data"]; ok && len(data) > 0 && data[0] == '{' {
		return data
	}
	return raw
}

// unwrapArray returns the array payload from a {"data": [...]} or
// {"orders": [...]} envelope, else raw itself (which may already be a naked
// array). Non-array payloads are returned unchanged.
func unwrapArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return trimmed
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return raw
	}
	for _, key := range []string{"data", "orders"} {
		if v, ok := probe[key]; ok {
			vt := bytes.TrimSpace(v)
			if len(vt) > 0 && vt[0] == '[' {
				return vt
			}
		}
	}
	return raw
}

func copyHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
