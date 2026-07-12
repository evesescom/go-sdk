package eveses

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// CaptchaSolution is a resolved captcha task returned by Captcha.Solve.
type CaptchaSolution struct {
	TaskID        int    `json:"task_id"`
	Status        string `json:"status"`
	Solution      string `json:"solution,omitempty"`
	Error         string `json:"error,omitempty"`
	PriceMicroUSD int    `json:"price_micro_usd,omitempty"`
}

// CaptchaSolveParams are the optional knobs for Captcha.Solve.
type CaptchaSolveParams struct {
	// CallbackURL, when set, is also POSTed the result by the API.
	CallbackURL string
	// IdempotencyKey — replays return the same task instead of a new solve.
	IdempotencyKey string
	// TimeoutSec caps how long Solve blocks waiting for the result (default 180).
	TimeoutSec int
}

const captchaBase = "/api/v1/captcha"

// CaptchaUsageTask is a single captcha task in the usage history.
type CaptchaUsageTask struct {
	ID           int    `json:"id"`
	Type         string `json:"type"`
	Status       string `json:"status"` // queued|processing|ready|failed
	CostMicroUSD int    `json:"cost_micro_usd"`
	CostCents    any    `json:"cost_cents,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	ResolvedAt   string `json:"resolved_at,omitempty"`
	Error        string `json:"error,omitempty"`
}

// CaptchaUsageMeta is the cursor + billing meta on a captcha usage page.
type CaptchaUsageMeta struct {
	NextCursor       string `json:"next_cursor,omitempty"`
	HasMore          bool   `json:"has_more"`
	UnbilledMicroUSD int    `json:"unbilled_micro_usd,omitempty"`
	UnbilledCents    any    `json:"unbilled_cents,omitempty"`
}

// CaptchaUsageResponse is the response of Captcha.Usage — cursor-paginated
// captcha task history.
type CaptchaUsageResponse struct {
	Data []CaptchaUsageTask `json:"data"`
	Meta CaptchaUsageMeta   `json:"meta"`
}

// CaptchaUsageParams are the optional filters for Captcha.Usage.
type CaptchaUsageParams struct {
	Status string
	Type   string
	Cursor string
	Limit  int
}

// CaptchaService wraps /api/v1/captcha.
//
// It resells 2captcha solving, billed pay-per-use from the wallet
// (count-on-success).
type CaptchaService struct {
	client *Client
}

type captchaStartResponse struct {
	TaskID        int    `json:"task_id"`
	Status        string `json:"status"`
	Solution      string `json:"solution"`
	Error         string `json:"error"`
	PriceMicroUSD int    `json:"price_micro_usd"`
	RetryAfter    int    `json:"retry_after"`
}

type captchaResultResponse struct {
	Status     string `json:"status"`
	Solution   string `json:"solution"`
	Error      string `json:"error"`
	RetryAfter int    `json:"retry_after"`
}

// Solve submits a captcha task and blocks, polling the result endpoint while
// honouring the API's retry_after, until the task is ready/failed or TimeoutSec
// elapses. Returns the solution, or an error on failure/timeout.
func (s *CaptchaService) Solve(ctx context.Context, captchaType string, params map[string]any, opts *CaptchaSolveParams) (*CaptchaSolution, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = &CaptchaSolveParams{}
	}
	if params == nil {
		params = map[string]any{}
	}

	body := map[string]any{"type": captchaType, "params": params}
	headers := map[string]string{}
	if opts.CallbackURL != "" {
		body["callback_url"] = opts.CallbackURL
	}
	if opts.IdempotencyKey != "" {
		headers["Idempotency-Key"] = opts.IdempotencyKey
	}

	var started captchaStartResponse
	if err := s.client.do(ctx, requestOptions{
		method:  "POST",
		path:    captchaBase + "/solve",
		body:    body,
		headers: headers,
	}, &started); err != nil {
		return nil, err
	}

	price := started.PriceMicroUSD
	retryAfter := started.RetryAfter
	timeoutSec := opts.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 180
	}
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	if started.Status == "ready" || started.Status == "failed" {
		return finaliseCaptcha(started.TaskID, started.Status, started.Solution, started.Error, price)
	}

	for {
		wait := retryAfter
		if wait < 0 {
			wait = 0
		}
		select {
		case <-ctx.Done():
			return nil, &Error{Message: "context cancelled while polling captcha: " + ctx.Err().Error()}
		case <-time.After(time.Duration(wait) * time.Second):
		}

		var res captchaResultResponse
		if err := s.client.do(ctx, requestOptions{
			method: "GET",
			path:   captchaBase + "/result/" + url.PathEscape(fmt.Sprintf("%d", started.TaskID)),
		}, &res); err != nil {
			return nil, err
		}
		if res.RetryAfter > 0 {
			retryAfter = res.RetryAfter
		}

		if res.Status == "ready" || res.Status == "failed" {
			return finaliseCaptcha(started.TaskID, res.Status, res.Solution, res.Error, price)
		}

		if time.Now().After(deadline) {
			return nil, &Error{Message: fmt.Sprintf("captcha task %d timed out before resolving", started.TaskID)}
		}
	}
}

func finaliseCaptcha(taskID int, status, solution, errMsg string, price int) (*CaptchaSolution, error) {
	if status == "failed" {
		msg := errMsg
		if msg == "" {
			msg = "unknown error"
		}
		return nil, &Error{Message: fmt.Sprintf("captcha task %d failed: %s", taskID, msg)}
	}
	return &CaptchaSolution{
		TaskID:        taskID,
		Status:        status,
		Solution:      solution,
		Error:         errMsg,
		PriceMicroUSD: price,
	}, nil
}

// Rates returns the per-solve captcha price list via GET /api/v1/captcha/rates.
func (s *CaptchaService) Rates(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   captchaBase + "/rates",
	}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// Usage returns the cursor-paginated captcha task history via
// GET /api/v1/captcha/usage. params may be nil.
func (s *CaptchaService) Usage(ctx context.Context, params *CaptchaUsageParams) (*CaptchaUsageResponse, error) {
	q := url.Values{}
	if params != nil {
		if params.Status != "" {
			q.Set("status", params.Status)
		}
		if params.Type != "" {
			q.Set("type", params.Type)
		}
		if params.Cursor != "" {
			q.Set("cursor", params.Cursor)
		}
		if params.Limit > 0 {
			q.Set("limit", strconv.Itoa(params.Limit))
		}
	}

	var out CaptchaUsageResponse
	if err := s.client.do(ctx, requestOptions{
		method: "GET",
		path:   captchaBase + "/usage",
		query:  q,
	}, &out); err != nil {
		return nil, err
	}
	if out.Data == nil {
		out.Data = []CaptchaUsageTask{}
	}
	return &out, nil
}
