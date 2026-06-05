// Package emailsherlock is the official Go client for the EmailSherlock
// email-verification API. It verifies one address or a batch over HTTPS with
// an API key. See https://emailsherlock.com/api/docs.
package emailsherlock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://api.emailsherlock.com"
	version        = "0.1.0"
)

// Client talks to the EmailSherlock API. Create one with New. It is safe for
// concurrent use by multiple goroutines.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	userAgent  string

	// Verify holds the verify endpoints.
	Verify *VerifyService

	mu               sync.Mutex
	creditsRemaining *float64
	rateLimit        RateLimit
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (e.g. a staging host).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient injects a custom *http.Client (timeouts, transport, proxy).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// New creates a Client. If apiKey is empty it falls back to the ES_KEY or
// EMAILSHERLOCK_API_KEY environment variable.
func New(apiKey string, opts ...Option) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("ES_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("EMAILSHERLOCK_API_KEY")
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "emailsherlock-go/" + version,
	}
	for _, o := range opts {
		o(c)
	}
	c.Verify = &VerifyService{client: c}
	return c
}

// CreditsRemaining returns the credits left after the most recent request, or
// nil if no request has carried the header yet.
func (c *Client) CreditsRemaining() *float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.creditsRemaining
}

// LastRateLimit returns the rate-limit window after the most recent request.
func (c *Client) LastRateLimit() RateLimit {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rateLimit
}

func (c *Client) do(ctx context.Context, path string, payload, out any) error {
	if c.apiKey == "" {
		return &APIError{StatusCode: 0, Code: "config_error", Message: "no API key provided; pass it to New or set ES_KEY"}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("emailsherlock: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("emailsherlock: build request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("emailsherlock: request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("emailsherlock: read response: %w", err)
	}

	c.captureMeta(resp.Header)

	if resp.StatusCode >= 400 {
		var env errorEnvelope
		_ = json.Unmarshal(raw, &env)
		return errorFromResponse(resp.StatusCode, &env, resp.Header)
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("emailsherlock: decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) captureMeta(h http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v := h.Get("X-Credits-Remaining"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.creditsRemaining = &f
		}
	}
	if h.Get("X-RateLimit-Limit") != "" {
		c.rateLimit = RateLimit{
			Limit:     parseIntHeader(h, "X-RateLimit-Limit"),
			Remaining: parseIntHeader(h, "X-RateLimit-Remaining"),
			Reset:     parseIntHeader(h, "X-RateLimit-Reset"),
		}
	}
}
