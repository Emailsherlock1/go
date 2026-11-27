package emailsherlock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	// The generated client only decodes application/json; the real API always
	// sets it, so the stub does too (set before any WriteHeader in the handler).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
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
		w.Write([]byte(`{"email":"jane@acme.com","result":"valid","mx":true,"disposable":false,"role":false,"catch_all":false,"relay":false,"parked":false,"spam_trap":false,"score":0.95,"freshness":"fresh","decision":{"recommendation":"allow","reasons":["mailbox_accepts"]}}`))
	})
	defer srv.Close()

	got, err := c.Verify.Single(context.Background(), "jane@acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Result != "valid" || got.GetScore() != 0.95 {
		t.Errorf("unexpected result: %+v", got)
	}
	if cr := c.CreditsRemaining(); cr == nil || *cr != 41 {
		t.Errorf("credits not captured: %v", cr)
	}
	if rl := c.LastRateLimit(); rl.Limit == nil || *rl.Limit != 60 {
		t.Errorf("rate limit not captured: %+v", rl)
	}
}

// Regression (EM-1034): the wire stays snake_case via json tags; the generated
// Go fields/accessors expose it without losing any field.
func TestSingleKeepsWireFields(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"email":"jane@acme.com","result":"valid","mx":true,"mx_record":"mx1.acme.com","disposable":false,"role":false,"catch_all":true,"relay":false,"parked":false,"spam_trap":false,"free_email":false,"score":0.95,"freshness":"fresh","checked_at":"2026-06-09T14:32:00+00:00","decision":{"recommendation":"allow","reasons":["mailbox_accepts"]}}`))
	})
	defer srv.Close()

	got, err := c.Verify.Single(context.Background(), "jane@acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.CatchAll {
		t.Errorf("catch_all not decoded")
	}
	if got.GetMxRecord() != "mx1.acme.com" {
		t.Errorf("mx_record not decoded: %q", got.GetMxRecord())
	}
	if got.GetFreeEmail() != false || !got.HasFreeEmail() {
		t.Errorf("free_email not decoded")
	}
	if got.GetCheckedAt() != "2026-06-09T14:32:00+00:00" {
		t.Errorf("checked_at not decoded: %q", got.GetCheckedAt())
	}
}

func TestBatchMixed(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"email":"jane@acme.com","result":"valid","mx":true,"disposable":false,"role":false,"catch_all":false,"relay":false,"parked":false,"spam_trap":false,"freshness":"fresh","decision":{"recommendation":"allow","reasons":["mailbox_accepts"]}},{"email":"nope@","error":"invalid_email"}]}`))
	})
	defer srv.Close()

	got, err := c.Verify.Batch(context.Background(), []string{"jane@acme.com", "nope@"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("want 2 results, got %d", len(got.Results))
	}
	if !IsVerifyResult(got.Results[0]) {
		t.Errorf("first item should be a verify result")
	}
	if IsVerifyResult(got.Results[1]) {
		t.Errorf("second item should be a per-address error")
	}
	if got.Results[1].BatchItemError.Error != "invalid_email" {
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

// EM-4994: a 2xx body the generated models refuse (here without the required
// freshness) is a response problem, not a network one.
func TestResponseTheModelsRefuseIsInvalidResponse(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"email":"jane@acme.com","result":"valid","mx":true,"disposable":false,"role":false,"catch_all":false,"relay":false,"parked":false,"spam_trap":false,"score":0.95,"decision":{"recommendation":"allow","reasons":["mailbox_accepts"]}}`))
	})
	defer srv.Close()

	_, err := c.Verify.Single(context.Background(), "jane@acme.com")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.Code != "invalid_response" || apiErr.StatusCode != 200 {
		t.Errorf("want 200 invalid_response, got %+v", apiErr)
	}
	if !strings.Contains(apiErr.Message, "freshness") {
		t.Errorf("message should name the refused property: %q", apiErr.Message)
	}
}

func TestUnreachableServerIsNetworkError(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {})
	srv.Close()

	_, err := c.Verify.Single(context.Background(), "jane@acme.com")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "network_error" || apiErr.StatusCode != 0 {
		t.Errorf("want 0 network_error, got %v", err)
	}
}

const validBody = `{"email":"jane@acme.com","result":"valid","mx":true,"disposable":false,"role":false,"catch_all":false,"relay":false,"parked":false,"spam_trap":false,"score":0.95,"freshness":"fresh","decision":{"recommendation":"allow","reasons":["mailbox_accepts"]}}`

func singleWithHeaders(t *testing.T, headers map[string]string) *Client {
	t.Helper()
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.Write([]byte(validBody))
	})
	t.Cleanup(srv.Close)
	if _, err := c.Verify.Single(context.Background(), "jane@acme.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return c
}

// EM-4987: the alias X-Credits-Remaining carries another value on purpose, so
// the test fails if the client still reads the alias first.
func TestReadsBothBalancesFromTheirCurrentHeaders(t *testing.T) {
	c := singleWithHeaders(t, map[string]string{
		"X-Probe-Credits-Remaining": "41",
		"X-Credits-Remaining":       "7",
		"X-Validations-Remaining":   "950",
		"X-Verify-Free-Remaining":   "3",
	})
	if p := c.ProbeCreditsRemaining(); p == nil || *p != 41 {
		t.Errorf("probe credits: %v", p)
	}
	if v := c.ValidationsRemaining(); v == nil || *v != 950 {
		t.Errorf("validations: %v", v)
	}
	if cr := c.CreditsRemaining(); cr == nil || *cr != 41 {
		t.Errorf("deprecated alias: %v", cr)
	}
}

func TestFallsBackToTheLegacyBalanceHeaders(t *testing.T) {
	c := singleWithHeaders(t, map[string]string{"X-Credits-Remaining": "12", "X-Verify-Free-Remaining": "30"})
	if p := c.ProbeCreditsRemaining(); p == nil || *p != 12 {
		t.Errorf("probe credits: %v", p)
	}
	if v := c.ValidationsRemaining(); v == nil || *v != 30 {
		t.Errorf("validations: %v", v)
	}
}

func TestValidationsStayNilWhenThePlanIsUnlimited(t *testing.T) {
	c := singleWithHeaders(t, map[string]string{"X-Probe-Credits-Remaining": "41"})
	if v := c.ValidationsRemaining(); v != nil {
		t.Errorf("want nil validations, got %d", *v)
	}
	if p := c.ProbeCreditsRemaining(); p == nil || *p != 41 {
		t.Errorf("probe credits: %v", p)
	}
}

func TestInsufficientCreditsPrefersProbeCreditsHeader(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Probe-Credits-Remaining", "2")
		w.Header().Set("X-Credits-Remaining", "9")
		w.WriteHeader(402)
		w.Write([]byte(`{"error":{"code":"insufficient_credits","message":"Not enough."}}`))
	})
	defer srv.Close()

	_, err := c.Verify.Single(context.Background(), "a@b.com")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.CreditsRemaining == nil || *apiErr.CreditsRemaining != 2 {
		t.Errorf("credits remaining: %v", apiErr.CreditsRemaining)
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

func TestCreditsAndJob(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/credits":
			w.Write([]byte(`{"credits":{"total":1240,"purchased":1000,"gifted":240},"rate_limit":{"limit":60,"remaining":59,"reset":1700000000},"plan":"Free","sandbox":false}`))
		case "/v1/verify/jobs":
			w.WriteHeader(202)
			w.Write([]byte(`{"id":"job-1","status":"processing","total":2,"progress":{"total":2,"done":0},"created_at":"2026-06-10T14:32:00+00:00","expires_at":"2026-06-17T14:32:00+00:00"}`))
		}
	})
	defer srv.Close()

	acc, err := c.Credits(context.Background())
	if err != nil {
		t.Fatalf("credits error: %v", err)
	}
	if acc.Credits.GetTotal() != 1240 || acc.Sandbox != false {
		t.Errorf("unexpected account status: %+v", acc)
	}

	job, err := c.Verify.SubmitJob(context.Background(), []string{"a@b.com", "c@d.com"})
	if err != nil {
		t.Fatalf("submit job error: %v", err)
	}
	if job.Id != "job-1" || job.Status != "processing" {
		t.Errorf("unexpected job: %+v", job)
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

func TestClientIDHeader(t *testing.T) {
	var got []string
	var present []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("X-Client-Id"))
		_, ok := r.Header["X-Client-Id"]
		present = append(present, ok)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"email":"a@b.com","result":"valid","mx":true,"disposable":false,"role":false,"catch_all":false,"relay":false,"parked":false,"spam_trap":false,"score":0.95,"freshness":"fresh","decision":{"recommendation":"allow","reasons":["mailbox_accepts"]}}`))
	}))
	defer srv.Close()
	ctx := context.Background()

	plain := New("k", WithBaseURL(srv.URL))
	if _, err := plain.Verify.Single(ctx, "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if present[0] || plain.ClientID() != 0 {
		t.Errorf("header must be absent when unset: %v %d", present, plain.ClientID())
	}

	opt := New("k", WithBaseURL(srv.URL), WithClientID(42))
	if _, err := opt.Verify.Single(ctx, "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if got[1] != "42" || opt.ClientID() != 42 {
		t.Errorf("option: header %q id %d", got[1], opt.ClientID())
	}

	tenant := plain.WithClientID(7)
	if _, err := tenant.Verify.Single(ctx, "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := plain.Verify.Single(ctx, "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if got[2] != "7" || tenant.ClientID() != 7 {
		t.Errorf("method: header %q id %d", got[2], tenant.ClientID())
	}
	if present[3] || plain.ClientID() != 0 {
		t.Errorf("original changed: %v %d", present, plain.ClientID())
	}
	if tenant.Verify.client == plain.Verify.client {
		t.Errorf("derived client must be separate")
	}
}

func TestClientIDNonPositive(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()

	for name, c := range map[string]*Client{
		"option zero":     New("k", WithBaseURL(srv.URL), WithClientID(0)),
		"option negative": New("k", WithBaseURL(srv.URL), WithClientID(-3)),
		"method zero":     New("k", WithBaseURL(srv.URL)).WithClientID(0),
	} {
		_, err := c.Verify.Single(context.Background(), "a@b.com")
		apiErr, ok := err.(*APIError)
		if !ok || apiErr.Code != "config_error" {
			t.Errorf("%s: want config_error, got %v", name, err)
		}
		if c.ClientID() != 0 {
			t.Errorf("%s: ClientID should be 0", name)
		}
	}
	if calls != 0 {
		t.Errorf("no request may be sent, got %d", calls)
	}
}

func TestClientNotFoundCode(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":{"code":"client_not_found","message":"Unknown client"}}`))
	})
	defer srv.Close()
	_, err := c.WithClientID(9).Verify.Single(context.Background(), "a@b.com")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != 404 || apiErr.Code != "client_not_found" {
		t.Errorf("got %v", err)
	}
}
