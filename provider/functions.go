// Copyright 2026, RainnWorks.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// GetDnsRecords lists every editable record on a domain.
type GetDnsRecords struct{}

// GetDnsRecordsArgs are the inputs to getDnsRecords.
type GetDnsRecordsArgs struct {
	Domain string `pulumi:"domain"`
}

// GetDnsRecordsResult is the output of getDnsRecords.
type GetDnsRecordsResult struct {
	Domain  string          `pulumi:"domain"`
	Records []DnsRecordInfo `pulumi:"records"`
}

// DnsRecordInfo is one record as getDnsRecords returns it.
type DnsRecordInfo struct {
	RecordID string `pulumi:"recordId"`
	Name     string `pulumi:"name"`
	Fqdn     string `pulumi:"fqdn"`
	Type     string `pulumi:"type"`
	Content  string `pulumi:"content"`
	TTL      int    `pulumi:"ttl"`
	Prio     int    `pulumi:"prio"`
	Notes    string `pulumi:"notes"`
}

// Annotate describes the function.
func (f *GetDnsRecords) Annotate(a infer.Annotator) {
	a.Describe(&f, "Lists every editable DNS record on a Porkbun domain. Porkbun's default NS and SOA records are not included. "+
		"Useful for finding record IDs to import.")
}

// Annotate describes the inputs.
func (a *GetDnsRecordsArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "The domain, e.g. `example.com`.")
}

// Annotate describes the outputs.
func (r *GetDnsRecordsResult) Annotate(a infer.Annotator) {
	a.Describe(&r.Domain, "The domain that was queried.")
	a.Describe(&r.Records, "The domain's records.")
}

// Annotate describes a record.
func (r *DnsRecordInfo) Annotate(a infer.Annotator) {
	a.Describe(&r.RecordID, "Porkbun's ID for the record. Import it as `<domain>/<recordId>`.")
	a.Describe(&r.Name, "The subdomain, empty for the apex.")
	a.Describe(&r.Fqdn, "The fully qualified name.")
	a.Describe(&r.Type, "The record type.")
	a.Describe(&r.Content, "The record's value.")
	a.Describe(&r.TTL, "Time to live in seconds.")
	a.Describe(&r.Prio, "Priority; 0 for types that have none.")
	a.Describe(&r.Notes, "The note stored with the record, if any.")
}

// Invoke lists the records.
func (GetDnsRecords) Invoke(ctx context.Context, req infer.FunctionRequest[GetDnsRecordsArgs]) (infer.FunctionResponse[GetDnsRecordsResult], error) {
	domain := req.Input.Domain
	recs, err := clientFrom(ctx).ListRecords(ctx, domain)
	if err != nil {
		return infer.FunctionResponse[GetDnsRecordsResult]{}, fmt.Errorf("listing records for %s: %w", domain, err)
	}
	out := GetDnsRecordsResult{Domain: domain, Records: make([]DnsRecordInfo, 0, len(recs))}
	for _, r := range recs {
		ttl, _ := strconv.Atoi(r.TTL)
		prio, _ := strconv.Atoi(r.Prio)
		name := subdomain(r.Name, domain)
		out.Records = append(out.Records, DnsRecordInfo{
			RecordID: r.ID, Name: name, Fqdn: fqdn(name, domain), Type: r.Type,
			Content: r.Content, TTL: ttl, Prio: prio, Notes: r.Notes,
		})
	}
	return infer.FunctionResponse[GetDnsRecordsResult]{Output: out}, nil
}

// GetNameservers returns a domain's current nameservers.
type GetNameservers struct{}

// GetNameserversArgs are the inputs to getNameservers.
type GetNameserversArgs struct {
	Domain string `pulumi:"domain"`
}

// GetNameserversResult is the output of getNameservers.
type GetNameserversResult struct {
	Domain      string   `pulumi:"domain"`
	Nameservers []string `pulumi:"nameservers"`
}

// Annotate describes the function.
func (f *GetNameservers) Annotate(a infer.Annotator) {
	a.Describe(&f, "Returns the nameservers a Porkbun-registered domain is delegated to.")
}

// Annotate describes the inputs.
func (a *GetNameserversArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "The domain, e.g. `example.com`.")
}

// Annotate describes the outputs.
func (r *GetNameserversResult) Annotate(a infer.Annotator) {
	a.Describe(&r.Domain, "The domain that was queried.")
	a.Describe(&r.Nameservers, "The domain's nameservers, as the registry has them.")
}

// Invoke reads the nameservers.
func (GetNameservers) Invoke(ctx context.Context, req infer.FunctionRequest[GetNameserversArgs]) (infer.FunctionResponse[GetNameserversResult], error) {
	ns, err := clientFrom(ctx).GetNameservers(ctx, req.Input.Domain)
	if err != nil {
		return infer.FunctionResponse[GetNameserversResult]{}, fmt.Errorf("reading nameservers for %s: %w", req.Input.Domain, err)
	}
	return infer.FunctionResponse[GetNameserversResult]{Output: GetNameserversResult{Domain: req.Input.Domain, Nameservers: ns}}, nil
}
