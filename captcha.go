package eveses

import (
	"context"
	"fmt"
	"net/url"
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

// CaptchaService wraps /api/account/captcha.
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
		path:    "/api/account/captcha/solve",
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
			path:   "/api/account/captcha/result/" + url.PathEscape(fmt.Sprintf("%d", started.TaskID)),
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
