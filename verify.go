package emailsherlock

import (
	"context"

	"github.com/Emailsherlock1/go/genclient"
)

// Wire model aliases. The structs are generated from the OpenAPI spec in the
// genclient sub-package; these aliases keep the public names short and let
// callers import only this package.
type (
	// VerifyResult is the result for one address (was a hand-written struct in
	// v0.1.0; now the generated model, which also carries deliverable, reason,
	// mx_record, free_email, checked_at, domain and decision).
	VerifyResult = genclient.VerifyResultResponse
	// BatchResponse is the response of a batch call; Results holds one entry per
	// submitted address, in order.
	BatchResponse = genclient.VerifyBatchResponse
	// BatchItem is one batch entry: either a VerifyResultResponse or a
	// BatchItemError. Use IsVerifyResult to tell them apart.
	BatchItem = genclient.VerifyBatchResponseResultsInner
	// BatchItemError is a per-address failure inside a batch response.
	BatchItemError = genclient.BatchItemError
	// VerifyJob is an asynchronous verification job.
	VerifyJob = genclient.VerifyJobResponse
	// AccountStatus is the credit balance + rate-limit status behind a key.
	AccountStatus = genclient.AccountStatusResponse
	// DomainInfo is the domain-level intelligence attached to a result.
	DomainInfo = genclient.VerifyDomainResponse
	// Decision is the recommended action + reasons attached to a result.
	Decision = genclient.VerifyDecisionResponse
)

// IsVerifyResult reports whether a batch entry verified (true) or carries a
// per-address error (false). On true, read item.VerifyResultResponse; on false,
// item.BatchItemError.
func IsVerifyResult(item BatchItem) bool {
	return item.VerifyResultResponse != nil
}

// VerifyService groups the verify endpoints. Reached as Client.Verify.
type VerifyService struct {
	client *Client
}

// Single verifies one address.
func (s *VerifyService) Single(ctx context.Context, email string) (*VerifyResult, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.VerifyAPI.VerifySingle(ctx).
		VerifySingleRequest(*genclient.NewVerifySingleRequest(email)).Execute()
	return do(s.client, v, r, e)
}

// Batch verifies a batch of addresses in one call.
func (s *VerifyService) Batch(ctx context.Context, emails []string) (*BatchResponse, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.VerifyAPI.VerifyBatch(ctx).
		VerifyBatchRequest(*genclient.NewVerifyBatchRequest(emails)).Execute()
	return do(s.client, v, r, e)
}

// SubmitJob submits a list of addresses for asynchronous verification. Poll the
// returned job's ID with GetJob until its status is "completed".
func (s *VerifyService) SubmitJob(ctx context.Context, emails []string) (*VerifyJob, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.VerifyAPI.SubmitVerifyJob(ctx).
		VerifyJobRequest(*genclient.NewVerifyJobRequest(emails)).Execute()
	return do(s.client, v, r, e)
}

// GetJob reads the status and results of a verification job.
func (s *VerifyService) GetJob(ctx context.Context, id string) (*VerifyJob, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.VerifyAPI.GetVerifyJob(ctx, id).Execute()
	return do(s.client, v, r, e)
}

// Credits reads the credit balance and rate-limit status. Free: consumes no credits.
func (c *Client) Credits(ctx context.Context) (*AccountStatus, error) {
	if err := c.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := c.api.AccountAPI.GetCredits(ctx).Execute()
	return do(c, v, r, e)
}

// GuardService groups the Email-Guard endpoints. Reached as Client.Guard.
type GuardService struct {
	client *Client
}

// RecordEvents records a batch of Email-Guard decision events (free, no credits).
// The full email address is never sent, only the domain.
func (s *GuardService) RecordEvents(ctx context.Context, events []map[string]interface{}) error {
	if err := s.client.ensureKey(); err != nil {
		return err
	}
	resp, err := s.client.api.GuardAPI.RecordGuardEvents(ctx).
		GuardEventsRequest(*genclient.NewGuardEventsRequest(events)).Execute()
	if resp != nil {
		s.client.captureMeta(resp.Header)
	}
	if err != nil {
		return s.client.toAPIError(resp, err)
	}
	return nil
}
