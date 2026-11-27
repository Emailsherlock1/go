package emailsherlock

import (
	"context"

	"github.com/Emailsherlock1/go/genclient"
)

// DMARC, TLS-RPT and domain wire model aliases (generated in genclient).
type (
	// DMARCSetup is the answer of DMARC.Start and DMARC.Check: the record
	// status, whether the domain is confirmed as yours, and the record to publish.
	DMARCSetup = genclient.DmarcSetupResource
	// DMARCSummary holds the totals of a domain's DMARC reports over 7 or 30 days.
	DMARCSummary = genclient.DmarcSummaryResource
	// DMARCReportList is one page of a domain's DMARC aggregate reports.
	DMARCReportList = genclient.ListDmarcReports200Response
	// DMARCReport is one DMARC aggregate report.
	DMARCReport = genclient.DmarcReportResource
	// DMARCSourceList is one page of the senders of a domain.
	DMARCSourceList = genclient.ListDmarcSources200Response
	// DMARCSource is one sender row.
	DMARCSource = genclient.DmarcSourceResource
	// TLSRPTReportList is one page of a domain's TLS-RPT reports.
	TLSRPTReportList = genclient.ListTlsRptReports200Response
	// TLSRPTReport is one TLS-RPT report.
	TLSRPTReport = genclient.TlsRptReportResource
	// DomainChangeList is one page of a watched domain's timeline.
	DomainChangeList = genclient.ListDomainChanges200Response
	// DomainChange is one timeline entry.
	DomainChange = genclient.DomainChangeResource
	// NextSteps is what to change next on a domain you own.
	NextSteps = genclient.NextStepsResource
)

// DMARCSummaryParams holds the optional query parameters of DMARC.Summary.
// A zero field means the API default.
type DMARCSummaryParams struct {
	Range int32 // days: 7 or 30
}

// DMARCReportsParams holds the optional query parameters of DMARC.Reports.
// A zero field means the API default.
type DMARCReportsParams struct {
	From     string // date, YYYY-MM-DD
	To       string // date, YYYY-MM-DD
	Reporter string
	Limit    int32 // 1 to 100
	Offset   int32
}

// DMARCSourcesParams holds the optional query parameters of DMARC.Sources.
// A zero field means the API default.
type DMARCSourcesParams struct {
	Range  int32  // days: 7 or 30
	Result string // "pass" or "fail"
	Check  string // "dmarc", "spf" or "dkim"
	Limit  int32  // 1 to 100
	Offset int32
}

// TLSRPTReportsParams holds the optional query parameters of TLSRPT.Reports.
// A zero field means the API default.
type TLSRPTReportsParams struct {
	Range  int32 // days: 7 or 30
	Limit  int32 // 1 to 100
	Offset int32
}

// DomainChangesParams holds the optional query parameters of Domains.Changes.
// A zero field means the API default.
type DomainChangesParams struct {
	Limit  int32 // 1 to 100
	Offset int32
}

// DMARCService groups the DMARC setup and report endpoints. Reached as
// Client.DMARC. These calls consume no credits. Summary, Reports and Sources
// need your account's report address in the domain's DMARC record; watching a
// domain alone does not unlock its reports.
type DMARCService struct {
	client *Client
}

// Start adds domain to the domains your account watches and returns the DMARC
// record to publish so its reports reach your account.
func (s *DMARCService) Start(ctx context.Context, domain string) (*DMARCSetup, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.DomainSetupAPI.StartDmarcMonitoring(ctx, domain).Execute()
	return do(s.client, v, r, e)
}

// Check looks up a watched domain's DMARC record again and reports whether it
// carries your report address. Checks are limited per account and hour.
func (s *DMARCService) Check(ctx context.Context, domain string) (*DMARCSetup, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.DomainSetupAPI.CheckDmarcSetup(ctx, domain).Execute()
	return do(s.client, v, r, e)
}

// Summary reads the totals of a domain's DMARC reports. params may be nil.
func (s *DMARCService) Summary(ctx context.Context, domain string, params *DMARCSummaryParams) (*DMARCSummary, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	req := s.client.api.DomainReportsAPI.GetDmarcSummary(ctx, domain)
	if params != nil && params.Range != 0 {
		req = req.Range_(params.Range)
	}
	v, r, e := req.Execute()
	return do(s.client, v, r, e)
}

// Reports lists a domain's DMARC aggregate reports, newest first. params may be nil.
func (s *DMARCService) Reports(ctx context.Context, domain string, params *DMARCReportsParams) (*DMARCReportList, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	req := s.client.api.DomainReportsAPI.ListDmarcReports(ctx, domain)
	if params != nil {
		if params.From != "" {
			req = req.From(params.From)
		}
		if params.To != "" {
			req = req.To(params.To)
		}
		if params.Reporter != "" {
			req = req.Reporter(params.Reporter)
		}
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

// Sources lists the senders of a domain, in the order of the dashboard's
// senders table. params may be nil.
func (s *DMARCService) Sources(ctx context.Context, domain string, params *DMARCSourcesParams) (*DMARCSourceList, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	req := s.client.api.DomainReportsAPI.ListDmarcSources(ctx, domain)
	if params != nil {
		if params.Range != 0 {
			req = req.Range_(params.Range)
		}
		if params.Result != "" {
			req = req.Result(params.Result)
		}
		if params.Check != "" {
			req = req.Check(params.Check)
		}
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

// TLSRPTService groups the TLS-RPT report endpoint. Reached as Client.TLSRPT.
// It consumes no credits and needs your account's report address in the
// domain's TLS-RPT record.
type TLSRPTService struct {
	client *Client
}

// Reports lists the TLS-RPT reports your account received for a domain,
// newest first. params may be nil.
func (s *TLSRPTService) Reports(ctx context.Context, domain string, params *TLSRPTReportsParams) (*TLSRPTReportList, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	req := s.client.api.DomainReportsAPI.ListTlsRptReports(ctx, domain)
	if params != nil {
		if params.Range != 0 {
			req = req.Range_(params.Range)
		}
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

// DomainsService groups the endpoints about a domain you watch or own. Reached
// as Client.Domains. These calls consume no credits.
type DomainsService struct {
	client *Client
}

// Changes reads the timeline of a domain you watch (DNS, record and mail
// changes), newest first. params may be nil.
func (s *DomainsService) Changes(ctx context.Context, domain string, params *DomainChangesParams) (*DomainChangeList, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	req := s.client.api.DomainReportsAPI.ListDomainChanges(ctx, domain)
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

// NextSteps reads what to change next on a domain you own: the steps towards
// p=reject, the sources that still fail DMARC, and the open Fix Guide findings.
// It needs your account's report address in the domain's DMARC record.
func (s *DomainsService) NextSteps(ctx context.Context, domain string) (*NextSteps, error) {
	if err := s.client.ensureKey(); err != nil {
		return nil, err
	}
	v, r, e := s.client.api.DomainReportsAPI.GetDomainNextSteps(ctx, domain).Execute()
	return do(s.client, v, r, e)
}
