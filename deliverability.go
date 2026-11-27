package emailsherlock

import (
	"context"

	"github.com/Emailsherlock1/go/genclient"
)

// Deliverability wire model aliases (generated in genclient).
type (
	// DeliverabilityAddressList is the response of Deliverability.ListAddresses.
	DeliverabilityAddressList = genclient.ListCheckAddresses200Response
	// DeliverabilityAddress is one check address with its latest verdict.
	DeliverabilityAddress = genclient.DeliverabilityAddressResource
	// LatestVerdict is the latest cached verdict for one check address.
	LatestVerdict = genclient.DeliverabilityLatestVerdictResource
	// DeliverabilityCheckList is one page of the check log of an address.
	DeliverabilityCheckList = genclient.ListChecks200Response
	// DeliverabilityCheckSummary is one row of DeliverabilityCheckList.
	DeliverabilityCheckSummary = genclient.DeliverabilityCheckResource
	// DeliverabilityCheck is one check with its full audit breakdown in Result.
	DeliverabilityCheck = genclient.GetCheck200Response
	// CheckWebhook is the check.completed webhook of an address. Webhook.Secret
	// is set only in the answer of Deliverability.SetWebhook.
	CheckWebhook = genclient.DeliverabilityWebhookResource
)

// ListChecksParams holds the optional query parameters of
// Deliverability.ListChecks. A zero field means the API default.
type ListChecksParams struct {
	Limit  int32 // 1 to 100
	Offset int32
}

// DeliverabilityService groups the Deliverability check endpoints. Reached as
// Client.Deliverability. These calls consume no credits.
type DeliverabilityService struct {
	client *Client
}

// ListAddresses lists your check addresses, each with its latest verdict.
func (s *DeliverabilityService) ListAddresses(ctx context.Context) (*DeliverabilityAddressList, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.DeliverabilityAPI.ListCheckAddresses(ctx).Execute()
	return do(s.client, v, r, e)
}

// Latest reads the latest cached verdict for an address, without a live probe.
// Use it as the gate before a send.
func (s *DeliverabilityService) Latest(ctx context.Context, id int32) (*LatestVerdict, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.DeliverabilityAPI.GetLatestVerdict(ctx, id).Execute()
	return do(s.client, v, r, e)
}

// ListChecks reads the check log of one address, newest first. params may be nil.
func (s *DeliverabilityService) ListChecks(ctx context.Context, id int32, params *ListChecksParams) (*DeliverabilityCheckList, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	req := s.client.api.DeliverabilityAPI.ListChecks(ctx, id)
	if params != nil {
		if params.Limit != 0 {
			req = req.Limit(params.Limit)
		}
		if params.Offset != 0 {
			req = req.Offset(params.Offset)
		}
	}
	v, r, e := req.Execute()
	return do(s.client, v, r, e)
}

// GetCheck reads one check with its full audit breakdown.
func (s *DeliverabilityService) GetCheck(ctx context.Context, id int32) (*DeliverabilityCheck, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.DeliverabilityAPI.GetCheck(ctx, id).Execute()
	return do(s.client, v, r, e)
}

// SetWebhook wires (or replaces) the check.completed webhook of an address to
// url. The answer carries the signing secret, which the API shows only here.
func (s *DeliverabilityService) SetWebhook(ctx context.Context, id int32, url string) (*CheckWebhook, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	body := genclient.NewSetCheckWebhookRequest()
	body.SetUrl(url)
	v, r, e := s.client.api.DeliverabilityAPI.SetCheckWebhook(ctx, id).
		SetCheckWebhookRequest(*body).Execute()
	return do(s.client, v, r, e)
}

// DeleteWebhook removes the check.completed webhook of an address.
func (s *DeliverabilityService) DeleteWebhook(ctx context.Context, id int32) (*CheckWebhook, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.DeliverabilityAPI.DeleteCheckWebhook(ctx, id).Execute()
	return do(s.client, v, r, e)
}
