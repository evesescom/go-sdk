package eveses

// Error types for the Eveses SDK.
//
// Every non-2xx response is converted into an *Error or one of its
// specialised siblings:
//
//	400/422 -> *Error (StatusCode set; Code = "validation_failed")
//	401     -> *AuthError
//	403     -> *Error (Code = "forbidden")
//	404     -> *Error (Code = "not_found")
//	429     -> *RateLimitError (after the 1 auto-retry is exhausted)
//	5xx     -> *Error (Code = "server_error")
//
// Idiomatic check:
//
//	var authErr *eveses.AuthError
//	if errors.As(err, &authErr) { ... }

import (
	"fmt"
	"time"
)

// Error is the base Eveses SDK error returned for any non-2xx response or
// network failure. StatusCode is 0 for transport-level errors.
type Error struct {
	Message    string
	StatusCode int
	Code       string
	// Body is the parsed response body (decoded JSON or raw string),
	// retained for forward-compatible inspection.
	Body any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.StatusCode == 0 {
		return fmt.Sprintf("eveses: %s", e.Message)
	}
	return fmt.Sprintf("eveses: %d: %s", e.StatusCode, e.Message)
}

// AuthError is raised on 401. Use errors.As(err, &authErr) to detect.
//
// Note: rather than embedding *Error (which would cause a name-vs-method
// conflict on the "Error" identifier and break the error interface), each
// specialised error type carries its own fields and Unwrap chain.
type AuthError struct {
	Message    string
	StatusCode int
	Code       string
	Body       any
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("eveses: %d: %s", e.StatusCode, e.Message)
}

// Unwrap exposes the equivalent base *Error so errors.As(err, &*Error{}) also
// matches an *AuthError.
func (e *AuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &Error{
		Message:    e.Message,
		StatusCode: e.StatusCode,
		Code:       e.Code,
		Body:       e.Body,
	}
}

// RateLimitError is raised on 429 once the single auto-retry has been
// exhausted. RetryAfter mirrors the upstream Retry-After header (capped at
// 60s; 1s default when missing/unparseable).
type RateLimitError struct {
	Message    string
	StatusCode int
	Code       string
	Body       any
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("eveses: %d: %s (retry-after=%s)", e.StatusCode, e.Message, e.RetryAfter)
}

// Unwrap exposes the equivalent base *Error.
func (e *RateLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return &Error{
		Message:    e.Message,
		StatusCode: e.StatusCode,
		Code:       e.Code,
		Body:       e.Body,
	}
}

// newAuthError builds an *AuthError with consistent defaults.
func newAuthError(message string, body any) *AuthError {
	if message == "" {
		message = "Unauthenticated"
	}
	return &AuthError{
		Message:    message,
		StatusCode: 401,
		Code:       "unauthenticated",
		Body:       body,
	}
}

// newRateLimitError builds a *RateLimitError carrying the parsed Retry-After.
func newRateLimitError(message string, retryAfter time.Duration, body any) *RateLimitError {
	if message == "" {
		message = "Rate limited"
	}
	return &RateLimitError{
		Message:    message,
		StatusCode: 429,
		Code:       "rate_limited",
		Body:       body,
		RetryAfter: retryAfter,
	}
}
