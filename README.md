# emailsherlock-go

Official Go client for the [EmailSherlock](https://emailsherlock.com) email-verification API. Verify one address or a batch over HTTPS with an API key. Get an API key at https://emailsherlock.com/api

No third-party dependencies (standard library only). Go 1.21+.

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

	fmt.Println(result.Result) // "valid"
	fmt.Println(result.Score)  // 0.95
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
	if item.Failed() {
		fmt.Println(item.Email, "failed:", item.Error)
	} else {
		fmt.Println(item.Email, item.Result)
	}
}
```

## The result object

`VerifyResult` mirrors the API JSON:

| field        | type    | meaning                                                         |
|--------------|---------|-----------------------------------------------------------------|
| `Email`      | string  | the address you sent                                            |
| `Result`     | string  | `valid` · `invalid` · `catch_all` · `disposable` · `role` · `unknown` |
| `MX`         | bool    | the domain has reachable MX records                             |
| `Disposable` | bool    | throwaway / temporary-mail provider                             |
| `Role`       | bool    | role address such as `info@` or `sales@`                        |
| `CatchAll`   | bool    | host accepts mail for any local part                            |
| `Score`      | float64 | 0–1 confidence, higher is safer to send to                      |
| `Freshness`  | string  | `fresh` · `cached_recent` · `cached_stale_refreshed`            |

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
