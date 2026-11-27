# Changelog

## 0.5.0

### Added

- `es.ProbeCreditsRemaining()` and `es.ValidationsRemaining()`: the two balances the API reports, read from `X-Probe-Credits-Remaining` and `X-Validations-Remaining` (falling back to the older header names). `ValidationsRemaining()` stays `nil` on a paid plan, where standard verify is unlimited.

### Changed

- `APIError.CreditsRemaining` on a 402 reads `X-Probe-Credits-Remaining` first.

### Deprecated

- `es.CreditsRemaining()`: returns the same value as `ProbeCreditsRemaining()`.

## 0.4.0

### Added

- `es.Deliverability`: `ListAddresses(ctx)`, `Latest(ctx, id)` (the pre-send gate), `ListChecks(ctx, id, params)`, `GetCheck(ctx, id)`, `SetWebhook(ctx, id, url)` (returns the signing secret once) and `DeleteWebhook(ctx, id)`.
- `es.DMARC`: `Start(ctx, domain)`, `Check(ctx, domain)`, `Summary(ctx, domain, params)`, `Reports(ctx, domain, params)` and `Sources(ctx, domain, params)`.
- `es.TLSRPT`: `Reports(ctx, domain, params)`.
- `es.Domains`: `Changes(ctx, domain, params)` and `NextSteps(ctx, domain)`.
- `es.Impersonation`: `Analyze(ctx, req)` scores whether a sender poses as one of your protected identities. It consumes credits; every other call added here is free.
- Optional query parameters go in a `*XxxParams` struct (`ListChecksParams`, `DMARCSummaryParams`, `DMARCReportsParams`, `DMARCSourcesParams`, `TLSRPTReportsParams`, `DomainChangesParams`); pass `nil` for the API defaults.
- `WithClientID(id)` option and `(*Client).WithClientID(id)` method send the `X-Client-Id` header, so a provider key can work inside one of its tenants. `(*Client).ClientID()` reads it back. A non-positive id makes every call return a `config_error` without a request. (Listed under 0.3.0 before, but 0.3.0 shipped without it.)

### Changed

- The raw client in `genclient` is regenerated from the current spec. Its doc comments now point at https://www.hostsherlock.com/api/docs.
- `VerifyResult` gains `Relay`, `Parked` and `SpamTrap` (required in the spec, so a response without them fails to decode) plus the optional `RelayProvider`, `SpamTrapType`, `SuggestedCorrection`, `SmtpDiagnosis` and `Degraded`.
- The User-Agent is `emailsherlock-go/0.4.0`.

## 0.3.0

The client and models are now generated from the EmailSherlock OpenAPI spec
(raw client in the `genclient` sub-package), with a thin hand-maintained sugar
layer (the root `emailsherlock` package). The v0.2.0 surface mostly carries
over; see the breaking notes below.

### Added

- `es.Credits(ctx)` reads the account status (credit buckets, rate limit, plan, sandbox flag).
- `es.Verify.SubmitJob(ctx, emails)` and `es.Verify.GetJob(ctx, id)` for asynchronous verification jobs.
- `es.Guard.RecordEvents(ctx, events)` records Email-Guard decision events.
- `Decision` result field. The v2 fields from 0.2.0 are read with `Get<Field>()` accessors now, see the breaking notes.
- `IsVerifyResult(item)` to tell batch entries apart.

### Changed (breaking, acceptable on 0.x)

- `VerifyResult` is now the generated `genclient.VerifyResultResponse`: optional fields are nullable and read with `Get<Field>()` accessors (e.g. `GetScore()` instead of `Score`).
- Batch entries are `VerifyBatchResponseResultsInner` (a oneOf wrapper); use `IsVerifyResult(item)` then read `item.VerifyResultResponse` / `item.BatchItemError`. The old `item.Failed()` / embedded fields are gone.
- One new dependency: `gopkg.in/validator.v2` (pulled in by the generated models; the client was previously dependency-free).
- `APIError` (+ `IsUnauthorized` / `IsForbidden` / `IsInsufficientCredits` / `IsRateLimited` / `IsUnavailable`), `CreditsRemaining()`, `LastRateLimit()`, `WithBaseURL` / `WithHTTPClient`, and the `ES_KEY` / `EMAILSHERLOCK_API_KEY` fallback are unchanged.
- The wire stays snake_case via json tags (no field renaming).

## 0.2.0

Verify response v2 fields on the hand-written client. `VerifyResult` gains `Deliverable`, `Reason`, `MXRecord`, `FreeEmail`, `CheckedAt` and a `Domain` (`*VerifyDomain`: types, trust score, SPF/DKIM/DMARC and other mail-security signals). They are pointers, `nil` when the server did not send the field or sent `null`.

## 0.1.0

Initial release: `Verify.Single` / `Verify.Batch`, `APIError` predicates, credit and rate-limit accessors. Dependency-free.
