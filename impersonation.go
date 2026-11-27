package emailsherlock

import (
	"context"

	"github.com/Emailsherlock1/go/genclient"
)

// Impersonation wire model aliases (generated in genclient).
type (
	// ImpersonationRequest holds the sender header fields of one message plus
	// the identities you protect. Set the optional DisplayName, ReplyTo and
	// Dmarc fields with their Set* methods.
	ImpersonationRequest = genclient.ImpersonationAnalyzeRequest
	// ProtectedIdentity is one person or organisation a sender may pose as.
	ProtectedIdentity = genclient.ImpersonationAnalyzeRequestProtectedInner
	// ImpersonationResult is the risk, score, matched identity and reasons.
	ImpersonationResult = genclient.ImpersonationAnalyzeResponse
)

// ImpersonationService groups the impersonation endpoint. Reached as
// Client.Impersonation.
type ImpersonationService struct {
	client *Client
}

// Analyze scores whether a sender poses as one of your protected identities.
// It consumes credits.
func (s *ImpersonationService) Analyze(ctx context.Context, req ImpersonationRequest) (*ImpersonationResult, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.ImpersonationAPI.AnalyzeImpersonation(ctx).
		ImpersonationAnalyzeRequest(req).Execute()
	return do(s.client, v, r, e)
}
