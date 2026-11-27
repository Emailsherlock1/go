package emailsherlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Emailsherlock1/go/genclient"
)

// RateLimit reflects the per-key sliding-window state from the X-RateLimit-* headers.
type RateLimit struct {
	Limit     *int64
	Remaining *int64
	Reset     *int64
}

// APIError is returned for every non-2xx response, and with Code
// "invalid_response" for a 2xx response this SDK version's models cannot read.
// Inspect StatusCode / Code, or use the IsXxx helpers. Header-derived fields are
// populated where the API sends them.
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

// firstHeader returns the value of the first of keys the response carries, so a
// header can be read under its current name with its older name as fallback.
func firstHeader(h http.Header, keys ...string) string {
	for _, k := range keys {
		if v := h.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func parseFloatHeader(h http.Header, keys ...string) *float64 {
	v := firstHeader(h, keys...)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseIntHeader(h http.Header, keys ...string) *int64 {
	v := firstHeader(h, keys...)
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
		e.CreditsRemaining = parseFloatHeader(h, "X-Probe-Credits-Remaining", "X-Credits-Remaining")
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

// toAPIError maps a raw-client failure to an *APIError. The generated client
// returns a *GenericOpenAPIError carrying the response body, plus the
// *http.Response for status + headers. On a 2xx response the generated client
// returns that error type only when it could not decode the body into its
// models (a required property missing, a wrong type): that becomes an
// invalid_response APIError with the response's status. Any other failure
// (no response, a body that could not be read) becomes a network_error
// APIError.
func (c *Client) toAPIError(resp *http.Response, err error) error {
	var ge *genclient.GenericOpenAPIError
	isGenerated := errors.As(err, &ge)
	if resp != nil && resp.StatusCode >= 400 {
		var env errorEnvelope
		if isGenerated {
			_ = json.Unmarshal(ge.Body(), &env)
		}
		return errorFromResponse(resp.StatusCode, &env, resp.Header)
	}
	if resp != nil && isGenerated {
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       "invalid_response",
			Message:    "the API response does not match this SDK version's models: " + err.Error(),
		}
	}
	return &APIError{StatusCode: 0, Code: "network_error", Message: "emailsherlock: " + err.Error()}
}
