# emailsherlock-go

Official Go client for the [EmailSherlock](https://www.hostsherlock.com) email-verification API. Verify one address or a batch over HTTPS with an API key. Get an API key at https://www.hostsherlock.com/api. Want to try a single address by hand first? The free [email verification](https://www.hostsherlock.com/verify) tool runs the same checks in the browser.

Go 1.21+. The client and models are generated from the OpenAPI spec (sub-package `genclient`), with a thin hand-maintained layer for the ergonomics below. One small dependency (`gopkg.in/validator.v2`, pulled in by the generated models).

## Install

```bash
go get github.com/Emailsherlock1/go
```

```go
import emailsherlock "github.com/Emailsherlock1/go"
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"os"

	emailsherlock "github.com/Emailsherlock1/go"
)

func main() {
	// reads the key from the environment, never hard-code it
	es := emailsherlock.New(os.Getenv("ES_KEY"))

	result, err := es.Verify.Single(context.Background(), "jane@acme.com")
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Result)     // "valid"
	fmt.Println(result.GetScore()) // 0.95 (Get* accessors on nullable fields)
}
```

With an empty first argument, `New("")` falls back to the `ES_KEY` (or
`EMAILSHERLOCK_API_KEY`) environment variable.

## Batch

Up to 100 addresses per call. Each item is either a result or a per-address error:

```go
batch, err := es.Verify.Batch(ctx, []string{"jane@acme.com", "sales@acme.com"})
if err != nil {
	return err
}
for _, item := range batch.Results {
	if emailsherlock.IsVerifyResult(item) {
		r := item.VerifyResultResponse
		fmt.Println(r.Email, r.Result)
	} else {
		e := item.BatchItemError
		fmt.Println(e.GetEmail(), "failed:", e.Error)
	}
}
```

## Async jobs

For large lists, submit a job and poll it. Every address runs the full pipeline
including the SMTP probe, so the results carry definitive inbox verdicts:

```go
job, err := es.Verify.SubmitJob(ctx, []string{"a@acme.com", "b@acme.com"})
for err == nil && job.Status != "completed" {
	time.Sleep(2 * time.Second)
	job, err = es.Verify.GetJob(ctx, job.Id)
}
```

## Account status

```go
acc, err := es.Credits(ctx)
acc.Credits.GetTotal() // spendable credits
acc.Sandbox            // true on an es_test_ key
```

## Email-Guard events

Record Email-Guard decision events (free, no credits). The full address is never
sent, only the domain:

```go
err := es.Guard.RecordEvents(ctx, []map[string]interface{}{
	{"domain": "mailinator.com", "verdict": "disposable", "action": "deny",
		"reasons": []string{"disposable_provider"}, "degraded": false, "source": "local"},
})
```

## Deliverability checks

Read the verdicts of your Deliverability check addresses (free, no credits).
`Latest` is the gate before a send: the latest cached verdict, no live probe.

```go
list, err := es.Deliverability.ListAddresses(ctx)
latest, err := es.Deliverability.Latest(ctx, list.Addresses[0].Id)
if latest.HasResult {
	v := latest.GetVerdict()
	fmt.Println(v.GetScore(), v.GetVerdictBand())
}

checks, err := es.Deliverability.ListChecks(ctx, 7, &emailsherlock.ListChecksParams{Limit: 20})
check, err := es.Deliverability.GetCheck(ctx, checks.Checks[0].Id) // check.Result: full audit

hook, err := es.Deliverability.SetWebhook(ctx, 7, "https://hooks.example.com/es")
if wh, ok := hook.GetWebhookOk(); ok {
	store(wh.GetSecret()) // the signing secret is shown only in this answer
}
_, err = es.Deliverability.DeleteWebhook(ctx, 7)
```

## DMARC

`Start` watches a domain and returns the DMARC record to publish; `Check` looks
the record up again (free, no credits). The reads (`Summary`, `Reports`,
`Sources`) need your account's report address in the domain's DMARC record;
watching a domain alone does not unlock its reports. Pass `nil` params for the
API defaults.

```go
setup, err := es.DMARC.Start(ctx, "acme.com")
if rec, ok := setup.GetRecordOk(); ok {
	fmt.Println(rec.GetAction(), rec.GetHost(), rec.GetValue()) // the change to make in DNS
}

sum, err := es.DMARC.Summary(ctx, "acme.com", &emailsherlock.DMARCSummaryParams{Range: 7})
reports, err := es.DMARC.Reports(ctx, "acme.com", &emailsherlock.DMARCReportsParams{From: "2026-11-01"})
sources, err := es.DMARC.Sources(ctx, "acme.com", &emailsherlock.DMARCSourcesParams{Result: "fail"})
```

## TLS-RPT

Free, no credits. Needs your account's report address in the domain's TLS-RPT
record:

```go
tls, err := es.TLSRPT.Reports(ctx, "acme.com", &emailsherlock.TLSRPTReportsParams{Range: 30})
```

## Domain timeline and next steps

Free, no credits. `Changes` reads the timeline of a domain you watch;
`NextSteps` needs your account's report address in the domain's DMARC record:

```go
changes, err := es.Domains.Changes(ctx, "acme.com", nil)
steps, err := es.Domains.NextSteps(ctx, "acme.com") // steps.UnalignedSources, steps.Findings
```

## Impersonation

Score whether a sender poses as one of your people or partners. This call
consumes credits. Only sender header fields are sent, no subject or body:

```go
req := emailsherlock.ImpersonationRequest{
	From: "ceo@acme-payments.com",
	Protected: []emailsherlock.ProtectedIdentity{
		{Name: "Jane Doe", Domains: []string{"acme.com"}},
	},
}
req.SetDisplayName("Jane Doe")

res, err := es.Impersonation.Analyze(ctx, req)
fmt.Println(res.Risk, res.Score, res.GetIdentity()) // "high" 75 "Jane Doe"
```

## The result object

`VerifyResult` is generated from the spec. Required fields are plain values;
optional ones are nullable, read with `Get<Field>()` accessors:

| field          | meaning                                                         |
|----------------|-----------------------------------------------------------------|
| `Email`        | the address you sent                                            |
| `Result`       | `valid` · `invalid` · `catch_all` · `disposable` · `role` · `unknown` |
| `MX`           | the domain has reachable MX records                             |
| `Disposable` · `Role` · `CatchAll` | throwaway / role / catch-all flags          |
| `GetScore()`   | 0–1 confidence, higher is safer to send to                      |
| `Freshness`    | `fresh` · `cached_recent` · `cached_stale_refreshed`            |
| `GetDeliverable()` | proven via SMTP (true accepted, false provably bad, unset = unproven) |
| `GetReason()`  | why the pipeline decided (`mailbox_accepts`, `greylisted`, …)   |
| `GetMxRecord()` · `GetFreeEmail()` · `GetCheckedAt()` | primary MX host · freemail flag · ISO 8601 check time |
| `Domain`       | domain-level intelligence (SPF, DKIM, DMARC, score, blacklists, …) |
| `Decision`     | `Recommendation` (allow · deny · review) + `Reasons`            |

Every field and model is generated from the [OpenAPI spec](https://www.hostsherlock.com/api/docs); `Domain` is a `*VerifyDomainResponse` with its own `Get*` accessors (SPF, DKIM, DMARC, score, blacklists, …).

## Credits and rate limits

After every call:

```go
es.CreditsRemaining() // *float64, e.g. 41
es.LastRateLimit()    // RateLimit{Limit, Remaining, Reset}
```

## Errors

Every non-2xx response returns an `*APIError`. Inspect it with a type assertion
and the helper predicates:

```go
result, err := es.Verify.Single(ctx, "jane@acme.com")
if err != nil {
	var apiErr *emailsherlock.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.IsRateLimited():
			fmt.Printf("retry after %v\n", apiErr.RetryAfter)
		case apiErr.IsInsufficientCredits():
			fmt.Printf("need %v credits\n", apiErr.CreditsRequired)
		case apiErr.IsUnauthorized():
			fmt.Println("bad key")
		}
	}
	return err
}
```

| predicate                       | HTTP | extras                                   |
|---------------------------------|------|------------------------------------------|
| `IsUnauthorized()`              | 401  | -                                        |
| `IsForbidden()`                 | 403  | `RequiredScope`                          |
| `IsInsufficientCredits()`       | 402  | `CreditsRequired`, `CreditsRemaining`    |
| `IsRateLimited()`               | 429  | `RetryAfter`, `RateLimit`                |
| `IsUnavailable()`               | 503  | credit auto-refunded                     |

Other statuses (400, 404, 409, 422, 500) have no predicate; read `StatusCode`
and `Code`.

## Working for a tenant (`X-Client-Id`)

A provider account's key can run a call inside one of its tenants by naming the
tenant's numeric id. Usage, verify jobs, deliverability addresses and Guard
events then land in that tenant. Without it, the key works in its own account.

```go
// As an option:
es := emailsherlock.New(key, emailsherlock.WithClientID(4711))

// Or derive a client for a tenant from an existing one:
tenant := es.WithClientID(4711) // new client, same key and settings; es is unchanged
tenant.ClientID()               // 4711 (0 means no tenant)
```

Poll a job with the same client you submitted it with, because the job belongs
to that tenant. The id must be positive; otherwise every call returns an
`*APIError` with code `config_error` and sends nothing.

The server answers with:

- 404 `client_not_found` for any id your key may not reach.
- 403 `forbidden` if that tenant is blocked from the API.
- 400 `client_id_not_allowed` if the key is not a provider key.

Read the code from `APIError.Code`.

## Options

```go
es := emailsherlock.New(key,
	emailsherlock.WithBaseURL("https://api.emailsherlock.com"),
	emailsherlock.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
)
```

## License

MIT. Full API reference: https://www.hostsherlock.com/api/docs
