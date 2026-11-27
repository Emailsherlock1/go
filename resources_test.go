package emailsherlock

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// request is what the stub server saw for one call.
type request struct {
	method   string
	path     string // escaped, as sent on the wire
	query    string // url.Values.Encode() order (sorted by key)
	body     string
	apiKey   string
	clientID string
}

// newRecordingClient answers every request with status + body and records the
// last request it received.
func newRecordingClient(t *testing.T, status int, body string, header map[string]string) (*Client, *request, func()) {
	t.Helper()
	got := &request{}
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = request{
			method:   r.Method,
			path:     r.URL.EscapedPath(),
			query:    r.URL.Query().Encode(),
			body:     string(b),
			apiKey:   r.Header.Get("X-API-Key"),
			clientID: r.Header.Get("X-Client-Id"),
		}
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	})
	return c, got, srv.Close
}

func assertRequest(t *testing.T, got *request, method, path, query string) {
	t.Helper()
	if got.method != method {
		t.Errorf("method: want %s, got %s", method, got.method)
	}
	if got.path != path {
		t.Errorf("path: want %s, got %s", path, got.path)
	}
	if got.query != query {
		t.Errorf("query: want %q, got %q", query, got.query)
	}
}

func assertJSONBody(t *testing.T, got *request, want string) {
	t.Helper()
	var g, w interface{}
	if err := json.Unmarshal([]byte(got.body), &g); err != nil {
		t.Fatalf("body is not JSON: %q", got.body)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad want JSON: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("body: want %s, got %s", want, got.body)
	}
}

func assertNoBody(t *testing.T, got *request) {
	t.Helper()
	if got.body != "" {
		t.Errorf("want no body, got %q", got.body)
	}
}

// --- Deliverability ---

func TestDeliverabilityListAddresses(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"addresses":[{"id":7,"label":"Main","address":"check-7@in.example","active":true,"has_webhook":false,"latest_verdict":null,"created_at":"2026-11-01T10:00:00+00:00"}]}`, nil)
	defer done()

	res, err := c.Deliverability.ListAddresses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/deliverability/addresses", "")
	assertNoBody(t, got)
	if len(res.Addresses) != 1 || res.Addresses[0].Id != 7 || res.Addresses[0].Label != "Main" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDeliverabilityLatest(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"address_id":7,"has_result":true,"verdict":{"check_id":99,"score":82,"verdict_band":"good","received_at":"2026-11-02T10:00:00+00:00"}}`, nil)
	defer done()

	res, err := c.Deliverability.Latest(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/deliverability/addresses/7/latest", "")
	v := res.GetVerdict()
	if res.AddressId != 7 || !res.HasResult || v.GetScore() != 82 || v.GetVerdictBand() != "good" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDeliverabilityListChecks(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"address_id":7,"limit":10,"offset":20,"count":1,"checks":[{"id":99,"address_id":7,"address_label":"Main","address":"check-7@in.example","received_at":"2026-11-02T10:00:00+00:00","score":82,"verdict_band":"good","forwarded_inline":false}]}`, nil)
	defer done()

	res, err := c.Deliverability.ListChecks(context.Background(), 7, &ListChecksParams{Limit: 10, Offset: 20})
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/deliverability/addresses/7/checks", "limit=10&offset=20")
	if res.GetCount() != 1 || len(res.Checks) != 1 || res.Checks[0].Id != 99 {
		t.Errorf("unexpected result: %+v", res)
	}
}

// With nil params the generated client sends the spec defaults (limit 50,
// offset 0); this pins that behaviour so a regenerated client that changes it
// shows up here.
func TestDeliverabilityListChecksNilParams(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"checks":[]}`, nil)
	defer done()

	if _, err := c.Deliverability.ListChecks(context.Background(), 7, nil); err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/deliverability/addresses/7/checks", "limit=50&offset=0")
}

func TestDeliverabilityGetCheck(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"id":99,"address_id":7,"address_label":"Main","address":"check-7@in.example","received_at":"2026-11-02T10:00:00+00:00","from":"news@shop.example","score":82,"verdict_band":"good","forwarded_inline":false,"result":{"spf":"pass"}}`, nil)
	defer done()

	res, err := c.Deliverability.GetCheck(context.Background(), 99)
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/deliverability/checks/99", "")
	if res.Id != 99 || res.GetFrom() != "news@shop.example" || res.Result["spf"] != "pass" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDeliverabilitySetWebhook(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"address_id":7,"webhook":{"url":"https://hooks.example.com/es","secret":"whsec_abc"}}`, nil)
	defer done()

	res, err := c.Deliverability.SetWebhook(context.Background(), 7, "https://hooks.example.com/es")
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "PUT", "/v1/deliverability/addresses/7/webhook", "")
	assertJSONBody(t, got, `{"url":"https://hooks.example.com/es"}`)
	w := res.GetWebhook()
	if res.AddressId != 7 || w.GetSecret() != "whsec_abc" || w.GetUrl() != "https://hooks.example.com/es" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDeliverabilityDeleteWebhook(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"address_id":7,"webhook":null}`, nil)
	defer done()

	res, err := c.Deliverability.DeleteWebhook(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "DELETE", "/v1/deliverability/addresses/7/webhook", "")
	assertNoBody(t, got)
	if res.AddressId != 7 || res.Webhook.Get() != nil {
		t.Errorf("unexpected result: %+v", res)
	}
}

// --- DMARC ---

const dmarcSetupJSON = `{"domain":"bücher.example","status":"not_found","confirmed":false,"record":{"name":"_dmarc.bücher.example","type":"TXT","value":"v=DMARC1; p=none; rua=mailto:r@in.example"},"watch_slots":{"used":1,"max":5}}`

func TestDMARCStart(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, dmarcSetupJSON, nil)
	defer done()

	res, err := c.DMARC.Start(context.Background(), "bücher.example")
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "POST", "/v1/domains/b%C3%BCcher.example/dmarc", "")
	if res.Domain != "bücher.example" || res.Status != "not_found" || res.Confirmed || res.WatchSlots.GetUsed() != 1 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDMARCCheck(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"domain":"a/b.example","status":"found","confirmed":true,"watch_slots":{"used":1,"max":null}}`, nil)
	defer done()

	res, err := c.DMARC.Check(context.Background(), "a/b.example")
	if err != nil {
		t.Fatal(err)
	}
	// a slash inside the domain stays one path segment
	assertRequest(t, got, "POST", "/v1/domains/a%2Fb.example/dmarc/check", "")
	if res.Status != "found" || !res.Confirmed {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDMARCSummary(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"domain":"bücher.example","range_days":7,"message_count":120,"aligned_messages":100,"aligned_pct":83.3,"source_count":4,"known_source_count":3,"unknown_source_count":1,"new_source_count":0,"failing_source_count":1}`, nil)
	defer done()

	res, err := c.DMARC.Summary(context.Background(), "bücher.example", &DMARCSummaryParams{Range: 7})
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/dmarc/b%C3%BCcher.example/summary", "range=7")
	if res.RangeDays != 7 || res.MessageCount != 120 || res.FailingSourceCount != 1 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDMARCReports(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"domain":"bücher.example","limit":10,"offset":0,"count":0,"next_offset":null,"reports":[]}`, nil)
	defer done()

	res, err := c.DMARC.Reports(context.Background(), "bücher.example", &DMARCReportsParams{
		From: "2026-11-01", To: "2026-11-20", Reporter: "google.com", Limit: 10, Offset: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/dmarc/b%C3%BCcher.example/reports",
		"from=2026-11-01&limit=10&offset=30&reporter=google.com&to=2026-11-20")
	if res.GetDomain() != "bücher.example" || res.GetLimit() != 10 || res.NextOffset.Get() != nil {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDMARCSources(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"domain":"bücher.example","range_days":7,"limit":5,"offset":5,"count":0,"next_offset":10,"sources":[]}`, nil)
	defer done()

	res, err := c.DMARC.Sources(context.Background(), "bücher.example", &DMARCSourcesParams{
		Range: 7, Result: "fail", Check: "spf", Limit: 5, Offset: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/dmarc/b%C3%BCcher.example/sources",
		"check=spf&limit=5&offset=5&range=7&result=fail")
	if res.GetRangeDays() != 7 || res.GetNextOffset() != 10 {
		t.Errorf("unexpected result: %+v", res)
	}
}

// --- TLS-RPT ---

func TestTLSRPTReports(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"domain":"bücher.example","range_days":30,"limit":20,"offset":40,"count":0,"next_offset":null,"reports":[]}`, nil)
	defer done()

	res, err := c.TLSRPT.Reports(context.Background(), "bücher.example", &TLSRPTReportsParams{Range: 30, Limit: 20, Offset: 40})
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/tls-rpt/b%C3%BCcher.example/reports", "limit=20&offset=40&range=30")
	if res.GetRangeDays() != 30 || res.GetOffset() != 40 {
		t.Errorf("unexpected result: %+v", res)
	}
}

// --- Domains ---

func TestDomainsChanges(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"domain":"bücher.example","history_from":"2026-10-28T00:00:00+00:00","limit":25,"offset":0,"count":1,"next_offset":null,"changes":[{"type":"dns_change","tag":"dns","title":"MX changed","description":"mx1 replaced","created_at":"2026-11-20T08:00:00+00:00"}]}`, nil)
	defer done()

	res, err := c.Domains.Changes(context.Background(), "bücher.example", &DomainChangesParams{Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/domains/b%C3%BCcher.example/changes", "limit=25&offset=0")
	if len(res.Changes) != 1 || res.Changes[0].Title != "MX changed" || res.GetHistoryFrom() == "" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestDomainsNextSteps(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"domain":"bücher.example","policy":{"tag":"p","value":"none"},"ladder":null,"unaligned_sources":[],"findings":[]}`, nil)
	defer done()

	res, err := c.Domains.NextSteps(context.Background(), "bücher.example")
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "GET", "/v1/domains/b%C3%BCcher.example/next-steps", "")
	if res.Domain != "bücher.example" || res.Policy.Value.Get() == nil || *res.Policy.Value.Get() != "none" {
		t.Errorf("unexpected result: %+v", res)
	}
}

// --- Impersonation ---

func impersonationRequest() ImpersonationRequest {
	req := ImpersonationRequest{
		From: "ceo@acme-payments.example",
		Protected: []ProtectedIdentity{
			{Name: "Jane Doe", Domains: []string{"acme.example"}},
		},
	}
	req.SetDisplayName("Jane Doe")
	return req
}

func TestImpersonationAnalyze(t *testing.T) {
	c, got, done := newRecordingClient(t, 200,
		`{"risk":"high","score":75,"identity":"Jane Doe","reasons":[{"reason":"display_name_match","weight":40}]}`,
		map[string]string{"X-Credits-Remaining": "12"})
	defer done()

	res, err := c.Impersonation.Analyze(context.Background(), impersonationRequest())
	if err != nil {
		t.Fatal(err)
	}
	assertRequest(t, got, "POST", "/v1/impersonation/analyze", "")
	assertJSONBody(t, got, `{"from":"ceo@acme-payments.example","display_name":"Jane Doe","protected":[{"name":"Jane Doe","domains":["acme.example"]}]}`)
	if res.Risk != "high" || res.Score != 75 || res.GetIdentity() != "Jane Doe" || len(res.Reasons) != 1 {
		t.Errorf("unexpected result: %+v", res)
	}
	if cr := c.CreditsRemaining(); cr == nil || *cr != 12 {
		t.Errorf("credits not captured: %v", cr)
	}
}

// --- shared behaviour on the new endpoints ---

func TestImpersonationInsufficientCredits(t *testing.T) {
	c, _, done := newRecordingClient(t, 402,
		`{"error":{"code":"insufficient_credits","message":"Not enough."}}`,
		map[string]string{"X-Credits-Required": "1", "X-Credits-Remaining": "0"})
	defer done()

	_, err := c.Impersonation.Analyze(context.Background(), impersonationRequest())
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if !apiErr.IsInsufficientCredits() || apiErr.Code != "insufficient_credits" ||
		apiErr.CreditsRequired == nil || *apiErr.CreditsRequired != 1 {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestDMARCForbidden(t *testing.T) {
	c, _, done := newRecordingClient(t, 403,
		`{"error":{"code":"forbidden","message":"Reports are locked for this domain."}}`, nil)
	defer done()

	_, err := c.DMARC.Summary(context.Background(), "bücher.example", nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if !apiErr.IsForbidden() || apiErr.Code != "forbidden" || apiErr.Message != "Reports are locked for this domain." {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestNewEndpointRateLimited(t *testing.T) {
	c, _, done := newRecordingClient(t, 429,
		`{"error":{"code":"rate_limit_exceeded","message":"Slow down."}}`,
		map[string]string{"Retry-After": "60", "X-RateLimit-Limit": "10", "X-RateLimit-Remaining": "0"})
	defer done()

	_, err := c.DMARC.Check(context.Background(), "bücher.example")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if !apiErr.IsRateLimited() || apiErr.RetryAfter == nil || *apiErr.RetryAfter != 60 {
		t.Errorf("unexpected error: %+v", apiErr)
	}
	if rl := c.LastRateLimit(); rl.Limit == nil || *rl.Limit != 10 {
		t.Errorf("rate limit not captured: %+v", rl)
	}
}

// 409 has no IsXxx predicate; it comes back as a plain *APIError.
func TestDMARCStartConflict(t *testing.T) {
	c, _, done := newRecordingClient(t, 409,
		`{"error":{"code":"watch_limit_reached","message":"No watch slot left."}}`, nil)
	defer done()

	_, err := c.DMARC.Start(context.Background(), "bücher.example")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.StatusCode != 409 || apiErr.Code != "watch_limit_reached" {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestClientIDReachesNewEndpoint(t *testing.T) {
	c, got, done := newRecordingClient(t, 200, `{"addresses":[]}`, nil)
	defer done()

	if _, err := c.WithClientID(4711).Deliverability.ListAddresses(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.clientID != "4711" {
		t.Errorf("X-Client-Id: want 4711, got %q", got.clientID)
	}
	if _, err := c.Deliverability.ListAddresses(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.clientID != "" {
		t.Errorf("X-Client-Id must be absent on the original client, got %q", got.clientID)
	}
}

func TestEnvKeyReachesNewEndpoint(t *testing.T) {
	t.Setenv("ES_KEY", "env-key")
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"addresses":[]}`))
	}))
	defer srv.Close()

	if _, err := New("", WithBaseURL(srv.URL)).Deliverability.ListAddresses(context.Background()); err != nil {
		t.Fatal(err)
	}
	if key != "env-key" {
		t.Errorf("X-API-Key: want env-key, got %q", key)
	}
}

func TestNewEndpointsMissingKey(t *testing.T) {
	t.Setenv("ES_KEY", "")
	t.Setenv("EMAILSHERLOCK_API_KEY", "")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	c := New("", WithBaseURL(srv.URL))
	ctx := context.Background()

	for name, call := range map[string]func() error{
		"Deliverability.ListAddresses": func() error { _, err := c.Deliverability.ListAddresses(ctx); return err },
		"Deliverability.Latest":        func() error { _, err := c.Deliverability.Latest(ctx, 1); return err },
		"Deliverability.ListChecks":    func() error { _, err := c.Deliverability.ListChecks(ctx, 1, nil); return err },
		"Deliverability.GetCheck":      func() error { _, err := c.Deliverability.GetCheck(ctx, 1); return err },
		"Deliverability.SetWebhook":    func() error { _, err := c.Deliverability.SetWebhook(ctx, 1, "https://h.example"); return err },
		"Deliverability.DeleteWebhook": func() error { _, err := c.Deliverability.DeleteWebhook(ctx, 1); return err },
		"DMARC.Start":                  func() error { _, err := c.DMARC.Start(ctx, "d.example"); return err },
		"DMARC.Check":                  func() error { _, err := c.DMARC.Check(ctx, "d.example"); return err },
		"DMARC.Summary":                func() error { _, err := c.DMARC.Summary(ctx, "d.example", nil); return err },
		"DMARC.Reports":                func() error { _, err := c.DMARC.Reports(ctx, "d.example", nil); return err },
		"DMARC.Sources":                func() error { _, err := c.DMARC.Sources(ctx, "d.example", nil); return err },
		"TLSRPT.Reports":               func() error { _, err := c.TLSRPT.Reports(ctx, "d.example", nil); return err },
		"Domains.Changes":              func() error { _, err := c.Domains.Changes(ctx, "d.example", nil); return err },
		"Domains.NextSteps":            func() error { _, err := c.Domains.NextSteps(ctx, "d.example"); return err },
		"Impersonation.Analyze":        func() error { _, err := c.Impersonation.Analyze(ctx, impersonationRequest()); return err },
	} {
		apiErr, ok := call().(*APIError)
		if !ok || apiErr.Code != "config_error" {
			t.Errorf("%s: want config_error", name)
		}
	}
	if calls != 0 {
		t.Errorf("no request may be sent without a key, got %d", calls)
	}
}
