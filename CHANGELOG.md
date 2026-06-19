# Changelog

## 0.2.0

The client and models are now generated from the EmailSherlock OpenAPI spec
(raw client in the `genclient` sub-package), with a thin hand-maintained sugar
layer (the root `emailsherlock` package). The v0.1.0 surface mostly carries
over; see the breaking notes below.

### Added

- `es.Credits(ctx)` reads the account status (credit buckets, rate limit, plan, sandbox flag).
- `es.Verify.SubmitJob(ctx, emails)` and `es.Verify.GetJob(ctx, id)` for asynchronous verification jobs.
- `es.Guard.RecordEvents(ctx, events)` records Email-Guard decision events.
- Richer result fields: `GetDeliverable()`, `GetReason()`, `GetMxRecord()`, `GetFreeEmail()`, `GetCheckedAt()`, `Domain` (full domain intelligence), and `Decision`.
- `IsVerifyResult(item)` to tell batch entries apart.

### Changed (breaking, acceptable on 0.x)

- `VerifyResult` is now the generated `genclient.VerifyResultResponse`: optional fields are nullable and read with `Get<Field>()` accessors (e.g. `GetScore()` instead of `Score`).
- Batch entries are `VerifyBatchResponseResultsInner` (a oneOf wrapper); use `IsVerifyResult(item)` then read `item.VerifyResultResponse` / `item.BatchItemError`. The old `item.Failed()` / embedded fields are gone.
- One new dependency: `gopkg.in/validator.v2` (pulled in by the generated models; the client was previously dependency-free).
- `APIError` (+ `IsUnauthorized` / `IsForbidden` / `IsInsufficientCredits` / `IsRateLimited` / `IsUnavailable`), `CreditsRemaining()`, `LastRateLimit()`, `WithBaseURL` / `WithHTTPClient`, and the `ES_KEY` / `EMAILSHERLOCK_API_KEY` fallback are unchanged.
- The wire stays snake_case via json tags (no field renaming).

## 0.1.0

Initial release: `Verify.Single` / `Verify.Batch`, `APIError` predicates, credit and rate-limit accessors. Dependency-free.
