package emailsherlock

import "context"

// VerifyResult is the result for one address. It mirrors the API JSON.
type VerifyResult struct {
	Email      string  `json:"email"`
	Result     string  `json:"result"` // valid | invalid | catch_all | disposable | role | unknown
	MX         bool    `json:"mx"`
	Disposable bool    `json:"disposable"`
	Role       bool    `json:"role"`
	CatchAll   bool    `json:"catch_all"`
	Score      float64 `json:"score"`
	Freshness  string  `json:"freshness"` // fresh | cached_recent | cached_stale_refreshed
}

// BatchItem is one entry of a batch response. It is either a verified result or
// a per-address error. Call Failed to tell them apart.
type BatchItem struct {
	VerifyResult
	// Error is non-empty when this address could not be processed:
	// invalid_email | insufficient_credits | verify_unavailable.
	Error string `json:"error,omitempty"`
}

// Failed reports whether this item carries a per-address error.
func (b BatchItem) Failed() bool { return b.Error != "" }

// BatchResponse is the response of a batch call.
type BatchResponse struct {
	Results []BatchItem `json:"results"`
}

// VerifyService groups the verify endpoints. Reached as Client.Verify.
type VerifyService struct {
	client *Client
}

type singleRequest struct {
	Email string `json:"email"`
}

type batchRequest struct {
	Emails []string `json:"emails"`
}

// Single verifies one address.
func (s *VerifyService) Single(ctx context.Context, email string) (*VerifyResult, error) {
	out := &VerifyResult{}
	if err := s.client.do(ctx, "/v1/verify/single", singleRequest{Email: email}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Batch verifies up to 100 addresses in one call.
func (s *VerifyService) Batch(ctx context.Context, emails []string) (*BatchResponse, error) {
	out := &BatchResponse{}
	if err := s.client.do(ctx, "/v1/verify/batch", batchRequest{Emails: emails}, out); err != nil {
		return nil, err
	}
	return out, nil
}
