// Package emailsherlock is the official Go client for the EmailSherlock
// email-verification API. It verifies one address or a batch over HTTPS with
// an API key. See https://emailsherlock.com/api/docs.
//
// The HTTP client and models are generated from the OpenAPI spec (sub-package
// genclient); this package is a thin sugar layer over it: APIError with IsXxx
// predicates, CreditsRemaining / LastRateLimit, an env-var key fallback, and
// the Verify / Guard services.
package emailsherlock

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Emailsherlock1/go/genclient"
)

const (
	defaultBaseURL = "https://api.emailsherlock.com"
	version        = "0.3.0"
)

// Client talks to the EmailSherlock API. Create one with New. It is safe for
// concurrent use by multiple goroutines.
type Client struct {
	api    *genclient.APIClient
	apiKey string
	cfg    config // resolved options, kept so WithClientID can rebuild

	// Verify holds the verify endpoints; Guard holds the Email-Guard endpoints.
	Verify *VerifyService
	Guard  *GuardService

	mu               sync.Mutex
	creditsRemaining *float64
	rateLimit        RateLimit
}

// errNoKey is returned (without any network call) when the client has no API key.
var errNoKey = &APIError{
	StatusCode: 0,
	Code:       "config_error",
	Message:    "emailsherlock: no API key provided; pass it to New or set ES_KEY",
}

// ensureKey guards every endpoint so a missing key fails fast, not over the wire.
func (c *Client) ensureKey() error {
	if c.apiKey == "" {
		return errNoKey
	}
	if c.cfg.clientIDSet && c.cfg.clientID <= 0 {
		return errBadClientID
	}
	return nil
}

// errBadClientID is returned (without any network call) when WithClientID got a
// non-positive id. New cannot return an error, so, like a missing key, the
// problem surfaces from every call instead of sending a bad header.
var errBadClientID = &APIError{
	StatusCode: 0,
	Code:       "config_error",
	Message:    "emailsherlock: client id must be a positive integer",
}

// Option configures a Client.
type Option func(*config)

type config struct {
	baseURL     string
	httpClient  *http.Client
	clientID    int64
	clientIDSet bool
}

// WithBaseURL overrides the API base URL (e.g. a staging host).
func WithBaseURL(u string) Option {
	return func(c *config) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient injects a custom *http.Client (timeouts, transport, proxy).
func WithHTTPClient(h *http.Client) Option {
	return func(c *config) { c.httpClient = h }
}

// WithClientID makes the client work inside one tenant of a provider account:
// it sends the X-Client-Id header on every request. Only a provider key may
// name a tenant (the id of one of its own tenants); any other key gets a 400
// client_id_not_allowed. A non-positive id is a config error: New still
// returns a client, and every call on it returns an *APIError with Code
// "config_error" before any request is made. Without this option the header is
// never sent.
func WithClientID(id int64) Option {
	return func(c *config) { c.clientID, c.clientIDSet = id, true }
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

	cfg := config{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(&cfg)
	}
	return newFromConfig(apiKey, cfg)
}

func newFromConfig(apiKey string, cfg config) *Client {
	gc := genclient.NewConfiguration()
	gc.Servers = genclient.ServerConfigurations{{URL: cfg.baseURL}}
	gc.HTTPClient = cfg.httpClient
	gc.UserAgent = "emailsherlock-go/" + version
	gc.AddDefaultHeader("X-API-Key", apiKey)
	if cfg.clientIDSet && cfg.clientID > 0 {
		gc.AddDefaultHeader("X-Client-Id", strconv.FormatInt(cfg.clientID, 10))
	}

	c := &Client{api: genclient.NewAPIClient(gc), apiKey: apiKey, cfg: cfg}
	c.Verify = &VerifyService{client: c}
	c.Guard = &GuardService{client: c}
	return c
}

// WithClientID returns a new Client that works inside the given tenant, with the
// same key, base URL and HTTP client. The receiver is unchanged and the two keep
// separate CreditsRemaining / LastRateLimit state. A non-positive id behaves as
// in the WithClientID option.
func (c *Client) WithClientID(id int64) *Client {
	cfg := c.cfg
	cfg.clientID, cfg.clientIDSet = id, true
	return newFromConfig(c.apiKey, cfg)
}

// ClientID returns the tenant id the client sends as X-Client-Id, or 0 if none.
func (c *Client) ClientID() int64 {
	if c.cfg.clientIDSet && c.cfg.clientID > 0 {
		return c.cfg.clientID
	}
	return 0
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

// do captures the response meta headers and maps any raw-client error to an
// *APIError. Execute() returns (*T, *http.Response, error), which lines up with
// this signature so call sites read `return do(c, ...Execute())`.
func do[T any](c *Client, val *T, resp *http.Response, err error) (*T, error) {
	if resp != nil {
		c.captureMeta(resp.Header)
	}
	if err != nil {
		return nil, c.toAPIError(resp, err)
	}
	return val, nil
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
