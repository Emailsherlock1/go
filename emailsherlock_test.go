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

func TestSingleV2Fields(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"email": "jane@acme.com",
			"result": "valid",
			"mx": true,
			"disposable": false,
			"role": false,
			"catch_all": false,
			"score": 0.95,
			"freshness": "fresh",
			"deliverable": true,
			"reason": "mailbox_accepts",
			"mx_record": "mx1.acme.com",
			"free_email": false,
			"checked_at": "2026-06-09T10:15:00+00:00",
			"domain": {
				"name": "acme.com",
				"types": ["company", "custom"],
				"score": 87.5,
				"spf": true,
				"dkim": true,
				"dmarc": true,
				"dmarc_policy": "reject",
				"mta_sts": true,
				"tls_rpt": false,
				"bimi": null,
				"dane": false,
				"blacklists": 0,
				"dnssec": "secure",
				"caa": true
			}
		}`))
	})
	defer srv.Close()

	got, err := c.Verify.Single(context.Background(), "jane@acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Deliverable == nil || !*got.Deliverable {
		t.Errorf("deliverable should be true: %+v", got.Deliverable)
	}
	if got.Reason == nil || *got.Reason != "mailbox_accepts" {
		t.Errorf("unexpected reason: %v", got.Reason)
	}
	if got.MXRecord == nil || *got.MXRecord != "mx1.acme.com" {
		t.Errorf("unexpected mx_record: %v", got.MXRecord)
	}
	if got.FreeEmail == nil || *got.FreeEmail {
		t.Errorf("free_email should be false: %v", got.FreeEmail)
	}
	if got.CheckedAt == nil || *got.CheckedAt != "2026-06-09T10:15:00+00:00" {
		t.Errorf("unexpected checked_at: %v", got.CheckedAt)
	}
	d := got.Domain
	if d == nil {
		t.Fatal("domain should be set")
	}
	if d.Name != "acme.com" {
		t.Errorf("unexpected domain name: %q", d.Name)
	}
	if len(d.Types) != 2 || d.Types[0] != "company" || d.Types[1] != "custom" {
		t.Errorf("unexpected domain types: %v", d.Types)
	}
	if d.Score == nil || *d.Score != 87.5 {
		t.Errorf("unexpected domain score: %v", d.Score)
	}
	if d.SPF == nil || !*d.SPF || d.DKIM == nil || !*d.DKIM || d.DMARC == nil || !*d.DMARC {
		t.Errorf("spf/dkim/dmarc should all be true: %+v", d)
	}
	if d.DMARCPolicy == nil || *d.DMARCPolicy != "reject" {
		t.Errorf("unexpected dmarc_policy: %v", d.DMARCPolicy)
	}
	if d.MTASTS == nil || !*d.MTASTS {
		t.Errorf("mta_sts should be true: %v", d.MTASTS)
	}
	if d.TLSRPT == nil || *d.TLSRPT {
		t.Errorf("tls_rpt should be false: %v", d.TLSRPT)
	}
	if d.BIMI != nil {
		t.Errorf("bimi should be nil for json null: %v", d.BIMI)
	}
	if d.DANE == nil || *d.DANE {
		t.Errorf("dane should be false: %v", d.DANE)
	}
	if d.Blacklists == nil || *d.Blacklists != 0 {
		t.Errorf("unexpected blacklists: %v", d.Blacklists)
	}
	if d.DNSSEC == nil || *d.DNSSEC != "secure" {
		t.Errorf("unexpected dnssec: %v", d.DNSSEC)
	}
	if d.CAA == nil || !*d.CAA {
		t.Errorf("caa should be true: %v", d.CAA)
	}
}

func TestSingleV1ResponseStillParses(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
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
	if got.Deliverable != nil {
		t.Errorf("deliverable should be nil on a v1 response: %v", got.Deliverable)
	}
	if got.Reason != nil {
		t.Errorf("reason should be nil on a v1 response: %v", got.Reason)
	}
	if got.MXRecord != nil {
		t.Errorf("mx_record should be nil on a v1 response: %v", got.MXRecord)
	}
	if got.FreeEmail != nil {
		t.Errorf("free_email should be nil on a v1 response: %v", got.FreeEmail)
	}
	if got.CheckedAt != nil {
		t.Errorf("checked_at should be nil on a v1 response: %v", got.CheckedAt)
	}
	if got.Domain != nil {
		t.Errorf("domain should be nil on a v1 response: %+v", got.Domain)
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
