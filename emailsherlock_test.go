package emailsherlock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	c := New("test-key", WithBaseURL(srv.URL))
	return c, srv
}

func TestSingleOK(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("missing api key header")
		}
		w.Header().Set("X-Credits-Remaining", "41")
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "59")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		w.Write([]byte(`{"email":"jane@acme.com","result":"valid","mx":true,"disposable":false,"role":false,"catch_all":false,"score":0.95,"freshness":"fresh"}`))
	})
	defer srv.Close()

	got, err := c.Verify.Single(context.Background(), "jane@acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Result != "valid" || got.Score != 0.95 {
		t.Errorf("unexpected result: %+v", got)
	}
	if cr := c.CreditsRemaining(); cr == nil || *cr != 41 {
		t.Errorf("credits not captured: %v", cr)
	}
	if rl := c.LastRateLimit(); rl.Limit == nil || *rl.Limit != 60 {
		t.Errorf("rate limit not captured: %+v", rl)
	}
}

func TestBatchMixed(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"email":"jane@acme.com","result":"valid","mx":true,"score":0.9,"freshness":"fresh"},{"email":"nope@","error":"invalid_email"}]}`))
	})
	defer srv.Close()

	got, err := c.Verify.Batch(context.Background(), []string{"jane@acme.com", "nope@"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("want 2 results, got %d", len(got.Results))
	}
	if got.Results[0].Failed() {
		t.Errorf("first item should not be failed")
	}
	if !got.Results[1].Failed() || got.Results[1].Error != "invalid_email" {
		t.Errorf("second item should be invalid_email: %+v", got.Results[1])
	}
}

func TestUnauthorized(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"code":"unauthorized","message":"Invalid API key."}}`))
	})
	defer srv.Close()

	_, err := c.Verify.Single(context.Background(), "a@b.com")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if !apiErr.IsUnauthorized() || apiErr.Code != "unauthorized" {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestInsufficientCredits(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Credits-Required", "1")
		w.Header().Set("X-Credits-Remaining", "0")
		w.WriteHeader(402)
		w.Write([]byte(`{"error":{"code":"insufficient_credits","message":"Not enough."}}`))
	})
	defer srv.Close()

	_, err := c.Verify.Single(context.Background(), "a@b.com")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if !apiErr.IsInsufficientCredits() || apiErr.CreditsRequired == nil || *apiErr.CreditsRequired != 1 {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestRateLimited(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"code":"rate_limit_exceeded","message":"Slow down."}}`))
	})
	defer srv.Close()

	_, err := c.Verify.Single(context.Background(), "a@b.com")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if !apiErr.IsRateLimited() || apiErr.RetryAfter == nil || *apiErr.RetryAfter != 30 {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestMissingKey(t *testing.T) {
	t.Setenv("ES_KEY", "")
	t.Setenv("EMAILSHERLOCK_API_KEY", "")
	c := New("")
	_, err := c.Verify.Single(context.Background(), "a@b.com")
	if err == nil {
		t.Fatal("want error for missing key")
	}
}
