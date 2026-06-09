package emailsherlock

import "context"

// VerifyResult is the result for one address. It mirrors the API JSON.
//
// The pointer fields were added with the v2 response. They are absent on
// older servers and nullable afterwards, so a nil pointer means "not
// provided" rather than a zero value.
type VerifyResult struct {
	Email      string `json:"email"`
	Result     string `json:"result"` // valid | invalid | catch_all | disposable | role | unknown
	MX         bool   `json:"mx"`
	Disposable bool   `json:"disposable"`
	Role       bool   `json:"role"`
	CatchAll   bool   `json:"catch_all"`
	// Score is the 0-1 confidence. The API sends score:null on unknown
	// results, which decodes to 0 here: 0 can mean a null score on
	// unknown results.
	Score     float64 `json:"score"`
	Freshness string  `json:"freshness"` // fresh | cached_recent | cached_stale_refreshed

	// Deliverable reports whether the mailbox accepts mail. Nil when the
	// server could not decide (or predates the v2 response).
	Deliverable *bool `json:"deliverable"`
	// Reason explains the result. One of: bad_syntax, no_mx,
	// mailbox_accepts, mailbox_not_found, disposable_provider,
	// role_address, catch_all_domain, greylisted, smtp_timeout,
	// smtp_unreachable, verification_pending.
	Reason *string `json:"reason"`
	// MXRecord is the hostname of the best-priority MX record.
	MXRecord *string `json:"mx_record"`
	// FreeEmail reports whether the domain is a free-mail provider.
	FreeEmail *bool `json:"free_email"`
	// CheckedAt is the ISO 8601 timestamp of the underlying check.
	CheckedAt *string `json:"checked_at"`
	// Domain carries domain-level reputation and mail-security details.
	Domain *VerifyDomain `json:"domain"`
}

// VerifyDomain is the domain object of a v2 verify response. Pointer fields
// are nullable: nil means the signal was not measured.
type VerifyDomain struct {
	Name string `json:"name"`
	// Types classifies the domain. Values: freemail, disposable, custom,
	// company, government, education, public, isp.
	Types []string `json:"types"`
	// Score is the 0-100 domain reputation score.
	Score *float64 `json:"score"`
	SPF   *bool    `json:"spf"`
	DKIM  *bool    `json:"dkim"`
	DMARC *bool    `json:"dmarc"`
	// DMARCPolicy is one of: none, quarantine, reject.
	DMARCPolicy *string `json:"dmarc_policy"`
	MTASTS      *bool   `json:"mta_sts"`
	TLSRPT      *bool   `json:"tls_rpt"`
	BIMI        *bool   `json:"bimi"`
	DANE        *bool   `json:"dane"`
	// Blacklists is the number of DNS blacklists currently listing the
	// domain's mail infrastructure.
	Blacklists *int `json:"blacklists"`
	// DNSSEC is one of: secure, insecure, bogus.
	DNSSEC *string `json:"dnssec"`
	CAA    *bool   `json:"caa"`
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
