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
	"slices"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// DomainNameservers manages the nameservers a Porkbun-registered domain is delegated to.
type DomainNameservers struct{}

// DomainNameserversArgs are the inputs to Nameservers.
type DomainNameserversArgs struct {
	Domain      string   `pulumi:"domain" provider:"replaceOnChanges"`
	Nameservers []string `pulumi:"nameservers"`
}

// DomainNameserversState is what's persisted in state.
type DomainNameserversState struct {
	DomainNameserversArgs
}

// Annotate describes the resource.
func (n *DomainNameservers) Annotate(a infer.Annotator) {
	a.Describe(&n, nameserversDescription)
}

// Annotate describes the inputs.
func (a *DomainNameserversArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "The domain, e.g. `example.com`. It must be registered with Porkbun. Changing it replaces the resource.")
	an.Describe(&a.Nameservers, "The nameservers to delegate the domain to, e.g. the two Cloudflare assigns a zone. Order does not matter.")
}

// Create sets the nameservers. The domain always has nameservers, so this
// takes over whatever is set now.
func (DomainNameservers) Create(ctx context.Context, req infer.CreateRequest[DomainNameserversArgs]) (infer.CreateResponse[DomainNameserversState], error) {
	in := req.Inputs
	if err := validateNameservers(in.Nameservers); err != nil {
		return infer.CreateResponse[DomainNameserversState]{}, err
	}
	state := DomainNameserversState{in}
	if req.DryRun {
		return infer.CreateResponse[DomainNameserversState]{ID: in.Domain, Output: state}, nil
	}
	if err := clientFrom(ctx).UpdateNameservers(ctx, in.Domain, in.Nameservers); err != nil {
		return infer.CreateResponse[DomainNameserversState]{}, fmt.Errorf("setting nameservers for %s: %w", in.Domain, err)
	}
	return infer.CreateResponse[DomainNameserversState]{ID: in.Domain, Output: state}, nil
}

// Update changes the nameservers.
func (DomainNameservers) Update(ctx context.Context, req infer.UpdateRequest[DomainNameserversArgs, DomainNameserversState]) (infer.UpdateResponse[DomainNameserversState], error) {
	in := req.Inputs
	if err := validateNameservers(in.Nameservers); err != nil {
		return infer.UpdateResponse[DomainNameserversState]{}, err
	}
	state := DomainNameserversState{in}
	if req.DryRun {
		return infer.UpdateResponse[DomainNameserversState]{Output: state}, nil
	}
	if err := clientFrom(ctx).UpdateNameservers(ctx, in.Domain, in.Nameservers); err != nil {
		return infer.UpdateResponse[DomainNameserversState]{}, fmt.Errorf("setting nameservers for %s: %w", in.Domain, err)
	}
	return infer.UpdateResponse[DomainNameserversState]{Output: state}, nil
}

// Diff ignores order and letter case: the registry treats nameservers as a set.
func (DomainNameservers) Diff(_ context.Context, req infer.DiffRequest[DomainNameserversArgs, DomainNameserversState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if req.Inputs.Domain != req.State.Domain {
		diff["domain"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
	}
	if !sameNameservers(req.Inputs.Nameservers, req.State.Nameservers) {
		diff["nameservers"] = p.PropertyDiff{Kind: p.Update, InputDiff: true}
	}
	return infer.DiffResponse{HasChanges: len(diff) > 0, DetailedDiff: diff}, nil
}

// Delete leaves the nameservers as they are. Reverting a domain's delegation
// would take it offline, so removing the resource only stops managing it.
func (DomainNameservers) Delete(ctx context.Context, req infer.DeleteRequest[DomainNameserversState]) (infer.DeleteResponse, error) {
	p.GetLogger(ctx).Info(fmt.Sprintf(
		"porkbun: %s keeps its current nameservers; deleting the resource only stops Pulumi managing them", req.ID))
	return infer.DeleteResponse{}, nil
}

// Read refreshes the nameservers from Porkbun. It also implements
// `pulumi import`, with the domain as the ID.
func (DomainNameservers) Read(ctx context.Context, req infer.ReadRequest[DomainNameserversArgs, DomainNameserversState]) (infer.ReadResponse[DomainNameserversArgs, DomainNameserversState], error) {
	domain := req.ID
	ns, err := clientFrom(ctx).GetNameservers(ctx, domain)
	if err != nil {
		return infer.ReadResponse[DomainNameserversArgs, DomainNameserversState]{}, fmt.Errorf("reading nameservers for %s: %w", domain, err)
	}
	if sameNameservers(ns, req.Inputs.Nameservers) {
		ns = req.Inputs.Nameservers
	}
	args := DomainNameserversArgs{Domain: domain, Nameservers: ns}
	return infer.ReadResponse[DomainNameserversArgs, DomainNameserversState]{ID: domain, Inputs: args, State: DomainNameserversState{args}}, nil
}

func validateNameservers(ns []string) error {
	if len(ns) < 2 {
		return fmt.Errorf("nameservers: give at least two (got %d); registries require it", len(ns))
	}
	for _, n := range ns {
		if strings.TrimSpace(n) == "" || strings.ContainsAny(n, " /:") {
			return fmt.Errorf("nameservers: %q is not a hostname", n)
		}
	}
	return nil
}

func sameNameservers(a, b []string) bool {
	norm := func(in []string) []string {
		out := make([]string, len(in))
		for i, s := range in {
			out[i] = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "."))
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(norm(a), norm(b))
}

const nameserversDescription = `Manages the nameservers a Porkbun-registered domain is delegated to.

Use it to move a domain's DNS to another provider, such as Cloudflare, while
keeping Porkbun as the registrar. A domain always has nameservers, so creating
this resource takes over the current set rather than adding to it.

**Deleting this resource does not change the nameservers.** Reverting a domain's
delegation would take it offline, so removing the resource only stops Pulumi
managing it. To go back to Porkbun's own DNS, set Porkbun's nameservers
explicitly before removing it.

{{% examples %}}
## Example Usage
{{% example %}}
### Delegate a domain to Cloudflare

` + "```typescript" + `
import * as cloudflare from "@pulumi/cloudflare";
import * as porkbun from "@rainnworks/porkbun";

const zone = new cloudflare.Zone("example", {
    account: { id: "<account-id>" },
    name: "example.com",
});

new porkbun.DomainNameservers("example", {
    domain: "example.com",
    nameservers: zone.nameServers,
});
` + "```" + `
` + "```python" + `
import pulumi_cloudflare as cloudflare
import rainnworks_porkbun as porkbun

zone = cloudflare.Zone("example",
    account={"id": "<account-id>"},
    name="example.com")

porkbun.DomainNameservers("example",
    domain="example.com",
    nameservers=zone.name_servers)
` + "```" + `
` + "```go" + `
package main

import (
	"github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := porkbun.NewDomainNameservers(ctx, "example", &porkbun.DomainNameserversArgs{
			Domain: pulumi.String("example.com"),
			Nameservers: pulumi.StringArray{
				pulumi.String("ada.ns.cloudflare.com"),
				pulumi.String("bob.ns.cloudflare.com"),
			},
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
    var ns = new DomainNameservers("example", new()
    {
        Domain = "example.com",
        Nameservers = new[] { "ada.ns.cloudflare.com", "bob.ns.cloudflare.com" },
    });
});
` + "```" + `
` + "```yaml" + `
resources:
  example:
    type: porkbun:DomainNameservers
    properties:
      domain: example.com
      nameservers:
        - ada.ns.cloudflare.com
        - bob.ns.cloudflare.com
` + "```" + `
{{% /example %}}
{{% /examples %}}

## Import

Import with the domain name. Importing changes nothing; it only records the
current nameservers.

` + "```sh" + `
$ pulumi import porkbun:index:DomainNameservers example example.com
` + "```" + `
`
