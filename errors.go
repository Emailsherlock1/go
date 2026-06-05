package emailsherlock

import (
	"fmt"
	"net/http"
	"strconv"
)

// RateLimit reflects the per-key sliding-window state from the X-RateLimit-* headers.
type RateLimit struct {
	Limit     *int64
	Remaining *int64
	Reset     *int64
}

// APIError is returned for every non-2xx response. Inspect StatusCode / Code, or
// use the IsXxx helpers. Header-derived fields are populated where the API sends them.
type APIError struct {
	StatusCode int
	Code       string
	Message    string

	// 403
	RequiredScope string
	// 402
	CreditsRequired  *float64
	CreditsRemaining *float64
	// 429
	RetryAfter *int64
	RateLimit  *RateLimit
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("emailsherlock: %s (%d %s)", e.Message, e.StatusCode, e.Code)
	}
	return fmt.Sprintf("emailsherlock: %s (HTTP %d)", e.Message, e.StatusCode)
}

// IsUnauthorized reports a 401 (missing or invalid API key).
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == http.StatusUnauthorized }

// IsForbidden reports a 403 (the key lacks the endpoint's scope).
func (e *APIError) IsForbidden() bool { return e.StatusCode == http.StatusForbidden }

// IsInsufficientCredits reports a 402.
func (e *APIError) IsInsufficientCredits() bool { return e.StatusCode == http.StatusPaymentRequired }

// IsRateLimited reports a 429.
func (e *APIError) IsRateLimited() bool { return e.StatusCode == http.StatusTooManyRequests }

// IsUnavailable reports a 503 (verify engine down; the credit is auto-refunded).
func (e *APIError) IsUnavailable() bool { return e.StatusCode == http.StatusServiceUnavailable }

type errorEnvelope struct {
	Error struct {
		Code          string `json:"code"`
		Message       string `json:"message"`
		RequiredScope string `json:"required_scope"`
	} `json:"error"`
}

func parseFloatHeader(h http.Header, key string) *float64 {
	v := h.Get(key)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseIntHeader(h http.Header, key string) *int64 {
	v := h.Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func errorFromResponse(status int, env *errorEnvelope, h http.Header) *APIError {
	e := &APIError{StatusCode: status}
	if env != nil {
		e.Code = env.Error.Code
		e.Message = env.Error.Message
		e.RequiredScope = env.Error.RequiredScope
	}
	if e.Message == "" {
		e.Message = fmt.Sprintf("HTTP %d", status)
	}

	switch status {
	case http.StatusForbidden:
		if e.RequiredScope == "" {
			e.RequiredScope = h.Get("X-Required-Scope")
		}
	case http.StatusPaymentRequired:
		e.CreditsRequired = parseFloatHeader(h, "X-Credits-Required")
		e.CreditsRemaining = parseFloatHeader(h, "X-Credits-Remaining")
	case http.StatusTooManyRequests:
		e.RetryAfter = parseIntHeader(h, "Retry-After")
		e.RateLimit = &RateLimit{
			Limit:     parseIntHeader(h, "X-RateLimit-Limit"),
			Remaining: parseIntHeader(h, "X-RateLimit-Remaining"),
			Reset:     parseIntHeader(h, "X-RateLimit-Reset"),
		}
	}
	return e
}
