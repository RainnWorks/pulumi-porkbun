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
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// DefaultTTL is Porkbun's minimum, and its default, TTL in seconds.
const DefaultTTL = 600

// DnsRecord manages one DNS record on a Porkbun domain.
type DnsRecord struct{}

// DnsRecordArgs are the inputs to a DnsRecord.
type DnsRecordArgs struct {
	Domain  string  `pulumi:"domain" provider:"replaceOnChanges"`
	Name    string  `pulumi:"name,optional"`
	Type    string  `pulumi:"type"`
	Content string  `pulumi:"content"`
	TTL     *int    `pulumi:"ttl,optional"`
	Prio    *int    `pulumi:"prio,optional"`
	Notes   *string `pulumi:"notes,optional"`
}

// DnsRecordState is what's persisted in state.
type DnsRecordState struct {
	DnsRecordArgs
	RecordID string `pulumi:"recordId"`
	Fqdn     string `pulumi:"fqdn"`
}

// Annotate describes the resource.
func (r *DnsRecord) Annotate(a infer.Annotator) {
	a.Describe(&r, dnsRecordDescription)
}

// Annotate describes the inputs.
func (a *DnsRecordArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "The domain the record belongs to, e.g. `example.com`. Changing it replaces the record.")
	an.Describe(&a.Name, "The subdomain, without the domain: `www`, `*` for a wildcard, or empty (the default) for the apex.")
	an.Describe(&a.Type, "The record type: `A`, `AAAA`, `CNAME`, `ALIAS`, `MX`, `TXT`, `NS`, `SRV`, `TLSA`, `CAA`, `HTTPS` or `SVCB`.")
	an.Describe(&a.Content, "The record's value. For `TXT`, give the text without surrounding quotes.")
	an.Describe(&a.TTL, "Time to live in seconds. Porkbun's minimum is 600, which is also the default.")
	an.SetDefault(&a.TTL, DefaultTTL)
	an.Describe(&a.Prio, "Priority, for `MX` and `SRV` records.")
	an.Describe(&a.Notes, "A note stored with the record in Porkbun. It is not published in DNS.")
}

// Annotate describes the outputs.
func (s *DnsRecordState) Annotate(a infer.Annotator) {
	a.Describe(&s.RecordID, "Porkbun's ID for the record.")
	a.Describe(&s.Fqdn, "The record's fully qualified name, e.g. `www.example.com`.")
}

// Create creates the record.
func (DnsRecord) Create(ctx context.Context, req infer.CreateRequest[DnsRecordArgs]) (infer.CreateResponse[DnsRecordState], error) {
	in := req.Inputs
	state := DnsRecordState{DnsRecordArgs: in, Fqdn: fqdn(in.Name, in.Domain)}
	if req.DryRun {
		return infer.CreateResponse[DnsRecordState]{Output: state}, nil
	}

	id, warnings, err := clientFrom(ctx).CreateRecord(ctx, in.Domain, recordInput(in))
	if err != nil {
		return infer.CreateResponse[DnsRecordState]{}, fmt.Errorf("creating %s record %q: %w", in.Type, state.Fqdn, err)
	}
	logWarnings(ctx, warnings)
	state.RecordID = id
	return infer.CreateResponse[DnsRecordState]{ID: recordResourceID(in.Domain, id), Output: state}, nil
}

// Update edits the record in place.
func (DnsRecord) Update(ctx context.Context, req infer.UpdateRequest[DnsRecordArgs, DnsRecordState]) (infer.UpdateResponse[DnsRecordState], error) {
	in := req.Inputs
	state := DnsRecordState{DnsRecordArgs: in, RecordID: req.State.RecordID, Fqdn: fqdn(in.Name, in.Domain)}
	if req.DryRun {
		return infer.UpdateResponse[DnsRecordState]{Output: state}, nil
	}

	warnings, err := clientFrom(ctx).EditRecord(ctx, in.Domain, req.State.RecordID, recordInput(in))
	if err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, fmt.Errorf("editing %s record %q: %w", in.Type, state.Fqdn, err)
	}
	logWarnings(ctx, warnings)
	return infer.UpdateResponse[DnsRecordState]{Output: state}, nil
}

// Delete deletes the record. A record that is already gone is not an error.
func (DnsRecord) Delete(ctx context.Context, req infer.DeleteRequest[DnsRecordState]) (infer.DeleteResponse, error) {
	domain, id, err := parseRecordResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := clientFrom(ctx).DeleteRecord(ctx, domain, id); err != nil && !IsNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("deleting record %s: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// Read refreshes the record from Porkbun. It also implements `pulumi import`,
// with an ID of the form `<domain>/<recordId>`.
func (DnsRecord) Read(ctx context.Context, req infer.ReadRequest[DnsRecordArgs, DnsRecordState]) (infer.ReadResponse[DnsRecordArgs, DnsRecordState], error) {
	domain, id, err := parseRecordResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{}, err
	}
	rec, err := clientFrom(ctx).GetRecord(ctx, domain, id)
	if err != nil {
		return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{}, fmt.Errorf("reading record %s: %w", req.ID, err)
	}
	if rec == nil {
		// Gone: an empty ID tells the engine to drop it from state.
		return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{}, nil
	}

	args := argsFromRecord(domain, rec, req.Inputs)
	state := DnsRecordState{DnsRecordArgs: args, RecordID: rec.ID, Fqdn: fqdn(args.Name, domain)}
	return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{ID: req.ID, Inputs: args, State: state}, nil
}

// argsFromRecord maps an API record back to inputs. Where Porkbun's form differs
// only cosmetically from what the program said (letter case, a zero priority,
// an empty note), the program's form is kept so that refresh shows no diff.
func argsFromRecord(domain string, rec *Record, prev DnsRecordArgs) DnsRecordArgs {
	args := DnsRecordArgs{
		Domain:  domain,
		Name:    subdomain(rec.Name, domain),
		Type:    rec.Type,
		Content: rec.Content,
	}
	if strings.EqualFold(prev.Name, args.Name) {
		args.Name = prev.Name
	}
	if strings.EqualFold(prev.Type, args.Type) {
		args.Type = prev.Type
	}

	ttl := DefaultTTL
	if n, err := strconv.Atoi(rec.TTL); err == nil {
		ttl = n
	}
	args.TTL = &ttl

	if n, err := strconv.Atoi(rec.Prio); err == nil && (n != 0 || prev.Prio != nil) {
		args.Prio = &n
	}
	if rec.Notes != "" || prev.Notes != nil {
		notes := rec.Notes
		args.Notes = &notes
	}
	return args
}

// recordInput is the full desired record. Every field is sent, so that removing
// an input (a note, a priority) clears it in Porkbun rather than leaving it.
func recordInput(in DnsRecordArgs) RecordInput {
	ttl := DefaultTTL
	if in.TTL != nil {
		ttl = *in.TTL
	}
	prio := 0
	if in.Prio != nil {
		prio = *in.Prio
	}
	notes := ""
	if in.Notes != nil {
		notes = *in.Notes
	}
	return RecordInput{
		Name:    in.Name,
		Type:    strings.ToUpper(in.Type),
		Content: in.Content,
		TTL:     strconv.Itoa(ttl),
		Prio:    strconv.Itoa(prio),
		Notes:   &notes,
	}
}

func recordResourceID(domain, recordID string) string { return domain + "/" + recordID }

func parseRecordResourceID(id string) (domain, recordID string, err error) {
	domain, recordID, ok := strings.Cut(id, "/")
	if !ok || domain == "" || recordID == "" {
		return "", "", fmt.Errorf("invalid DnsRecord ID %q: expected <domain>/<recordId>, e.g. example.com/123456789", id)
	}
	return domain, recordID, nil
}

// fqdn joins a subdomain and its domain: ("www", "example.com") -> "www.example.com".
func fqdn(name, domain string) string {
	if name == "" {
		return domain
	}
	return name + "." + domain
}

// subdomain is the inverse of fqdn: ("www.example.com", "example.com") -> "www".
func subdomain(name, domain string) string {
	name = strings.TrimSuffix(name, ".")
	if strings.EqualFold(name, domain) {
		return ""
	}
	if len(name) > len(domain)+1 && strings.EqualFold(name[len(name)-len(domain)-1:], "."+domain) {
		return name[:len(name)-len(domain)-1]
	}
	return name
}

func logWarnings(ctx context.Context, warnings []string) {
	for _, w := range warnings {
		p.GetLogger(ctx).Warning("Porkbun: " + w)
	}
}

const dnsRecordDescription = `Manages a DNS record on a domain registered with Porkbun.

The domain must use Porkbun's own nameservers for the record to resolve. Porkbun
still stores a record when the domain is delegated elsewhere, and reports that as
a warning, which this provider passes on.

{{% examples %}}
## Example Usage
{{% example %}}
### An A record and a mail exchanger

` + "```typescript" + `
import * as porkbun from "@rainnworks/porkbun";

const www = new porkbun.DnsRecord("www", {
    domain: "example.com",
    name: "www",
    type: "A",
    content: "203.0.113.10",
});

const mx = new porkbun.DnsRecord("mx", {
    domain: "example.com",
    type: "MX",
    content: "mail.example.com",
    prio: 10,
});
` + "```" + `
` + "```python" + `
import rainnworks_porkbun as porkbun

www = porkbun.DnsRecord("www",
    domain="example.com",
    name="www",
    type="A",
    content="203.0.113.10")

mx = porkbun.DnsRecord("mx",
    domain="example.com",
    type="MX",
    content="mail.example.com",
    prio=10)
` + "```" + `
` + "```go" + `
package main

import (
	"github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := porkbun.NewDnsRecord(ctx, "www", &porkbun.DnsRecordArgs{
			Domain:  pulumi.String("example.com"),
			Name:    pulumi.String("www"),
			Type:    pulumi.String("A"),
			Content: pulumi.String("203.0.113.10"),
		})
		if err != nil {
			return err
		}
		_, err = porkbun.NewDnsRecord(ctx, "mx", &porkbun.DnsRecordArgs{
			Domain:  pulumi.String("example.com"),
			Type:    pulumi.String("MX"),
			Content: pulumi.String("mail.example.com"),
			Prio:    pulumi.Int(10),
		})
		return err
	})
}
` + "```" + `
` + "```csharp" + `
using Pulumi;
using RainnWorks.Porkbun;

return await Deployment.RunAsync(() =>
{
    var www = new DnsRecord("www", new()
    {
        Domain = "example.com",
        Name = "www",
        Type = "A",
        Content = "203.0.113.10",
    });

    var mx = new DnsRecord("mx", new()
    {
        Domain = "example.com",
        Type = "MX",
        Content = "mail.example.com",
        Prio = 10,
    });
});
` + "```" + `
` + "```yaml" + `
resources:
  www:
    type: porkbun:DnsRecord
    properties:
      domain: example.com
      name: www
      type: A
      content: 203.0.113.10
  mx:
    type: porkbun:DnsRecord
    properties:
      domain: example.com
      type: MX
      content: mail.example.com
      prio: 10
` + "```" + `
{{% /example %}}
{{% /examples %}}

## Import

Import an existing record with its domain and Porkbun record ID. List the IDs with
the ` + "`getDnsRecords`" + ` function, or from the Porkbun API.

` + "```sh" + `
$ pulumi import porkbun:index:DnsRecord www example.com/123456789
` + "```" + `
`
