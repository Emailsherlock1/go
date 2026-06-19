# emailsherlock-go

Official Go client for the [EmailSherlock](https://emailsherlock.com) email-verification API. Verify one address or a batch over HTTPS with an API key. Get an API key at https://emailsherlock.com/api. Want to try a single address by hand first? The free [email verification](https://emailsherlock.com/verify) tool runs the same checks in the browser.

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

Note on `Score`: the API sends `score: null` on `unknown` results, which decodes
to `0` in Go. A score of `0` can therefore mean "no score".

### v2 response fields

Newer servers add these fields. They are pointers: `nil` means the field was
absent (older server) or `null` (not measured), so you can tell that apart
from a real `false`/`0`.

| field         | type            | meaning                                                          |
|---------------|-----------------|------------------------------------------------------------------|
| `Deliverable` | `*bool`         | the mailbox accepts mail                                         |
| `Reason`      | `*string`       | why you got this result, e.g. `mailbox_accepts`, `no_mx`, `catch_all_domain`, `verification_pending` |
| `MXRecord`    | `*string`       | hostname of the best-priority MX record                          |
| `FreeEmail`   | `*bool`         | the domain is a free-mail provider                               |
| `CheckedAt`   | `*string`       | ISO 8601 timestamp of the underlying check                       |
| `Domain`      | `*VerifyDomain` | domain-level reputation and mail-security details                |

`VerifyDomain` carries the domain object:

| field         | type       | meaning                                                              |
|---------------|------------|----------------------------------------------------------------------|
| `Name`        | `string`   | the domain                                                           |
| `Types`       | `[]string` | `freemail` · `disposable` · `custom` · `company` · `government` · `education` · `public` · `isp` |
| `Score`       | `*float64` | 0–100 domain reputation score                                        |
| `SPF` / `DKIM` / `DMARC` | `*bool` | the record is present and valid                            |
| `DMARCPolicy` | `*string`  | `none` · `quarantine` · `reject`                                     |
| `MTASTS` / `TLSRPT` / `BIMI` / `DANE` | `*bool` | mail-security signals                          |
| `Blacklists`  | `*int`     | number of DNS blacklists listing the domain's mail infrastructure    |
| `DNSSEC`      | `*string`  | `secure` · `insecure` · `bogus`                                      |
| `CAA`         | `*bool`    | a CAA record is present                                              |

```go
if result.Deliverable != nil && *result.Deliverable {
	fmt.Println("safe to send")
}
if d := result.Domain; d != nil && d.DMARCPolicy != nil {
	fmt.Println("DMARC policy:", *d.DMARCPolicy)
}
```

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

## Options

```go
es := emailsherlock.New(key,
	emailsherlock.WithBaseURL("https://api.emailsherlock.com"),
	emailsherlock.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
)
```

## License

MIT. Full API reference: https://emailsherlock.com/api/docs
