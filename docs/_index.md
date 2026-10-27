---
title: Porkbun
meta_desc: Provides an overview of the Porkbun Provider for Pulumi.
layout: package
---

The Porkbun provider manages resources on [Porkbun](https://porkbun.com), the domain
registrar: DNS records, and the nameservers a domain is delegated to. Configure it
with an API key pair before use; see [Installation & Configuration](./installation-configuration).

## Example

{{< chooser language "typescript,python,go,csharp,yaml" >}}
{{% choosable language typescript %}}

```typescript
import * as porkbun from "@rainnworks/porkbun";

const domain = "example.com";

const apex = new porkbun.DnsRecord("apex", {
    domain,
    type: "A",
    content: "203.0.113.10",
});

const www = new porkbun.DnsRecord("www", {
    domain,
    name: "www",
    type: "CNAME",
    content: domain,
});

const mx = new porkbun.DnsRecord("mx", {
    domain,
    type: "MX",
    content: "mail.example.com",
    prio: 10,
});

export const wwwName = www.fqdn;
```

{{% /choosable %}}
{{% choosable language python %}}

```python
import pulumi
import rainnworks_porkbun as porkbun

domain = "example.com"

apex = porkbun.DnsRecord("apex", domain=domain, type="A", content="203.0.113.10")

www = porkbun.DnsRecord("www", domain=domain, name="www", type="CNAME", content=domain)

mx = porkbun.DnsRecord("mx", domain=domain, type="MX", content="mail.example.com", prio=10)

pulumi.export("wwwName", www.fqdn)
```

{{% /choosable %}}
{{% choosable language go %}}

```go
package main

import (
	"github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		domain := pulumi.String("example.com")

		if _, err := porkbun.NewDnsRecord(ctx, "apex", &porkbun.DnsRecordArgs{
			Domain:  domain,
			Type:    pulumi.String("A"),
			Content: pulumi.String("203.0.113.10"),
		}); err != nil {
			return err
		}

		www, err := porkbun.NewDnsRecord(ctx, "www", &porkbun.DnsRecordArgs{
			Domain:  domain,
			Name:    pulumi.String("www"),
			Type:    pulumi.String("CNAME"),
			Content: domain,
		})
		if err != nil {
			return err
		}

		if _, err := porkbun.NewDnsRecord(ctx, "mx", &porkbun.DnsRecordArgs{
			Domain:  domain,
			Type:    pulumi.String("MX"),
			Content: pulumi.String("mail.example.com"),
			Prio:    pulumi.Int(10),
		}); err != nil {
			return err
		}

		ctx.Export("wwwName", www.Fqdn)
		return nil
	})
}
```

{{% /choosable %}}
{{% choosable language csharp %}}

```csharp
using System.Collections.Generic;
using Pulumi;
using RainnWorks.Porkbun;

return await Deployment.RunAsync(() =>
{
    const string domain = "example.com";

    var apex = new DnsRecord("apex", new()
    {
        Domain = domain,
        Type = "A",
        Content = "203.0.113.10",
    });

    var www = new DnsRecord("www", new()
    {
        Domain = domain,
        Name = "www",
        Type = "CNAME",
        Content = domain,
    });

    var mx = new DnsRecord("mx", new()
    {
        Domain = domain,
        Type = "MX",
        Content = "mail.example.com",
        Prio = 10,
    });

    return new Dictionary<string, object?>
    {
        ["wwwName"] = www.Fqdn,
    };
});
```

{{% /choosable %}}
{{% choosable language yaml %}}

```yaml
name: porkbun-example
runtime: yaml

variables:
  domain: example.com

resources:
  apex:
    type: porkbun:DnsRecord
    properties:
      domain: ${domain}
      type: A
      content: 203.0.113.10
  www:
    type: porkbun:DnsRecord
    properties:
      domain: ${domain}
      name: www
      type: CNAME
      content: ${domain}
  mx:
    type: porkbun:DnsRecord
    properties:
      domain: ${domain}
      type: MX
      content: mail.example.com
      prio: 10

outputs:
  wwwName: ${www.fqdn}
```

{{% /choosable %}}
{{< /chooser >}}

## Moving DNS to another provider

`DomainNameservers` delegates a domain to other nameservers, such as the two
Cloudflare assigns a zone, while Porkbun stays the registrar. Copy the records
into the new zone before changing the nameservers.

Deleting a `DomainNameservers` resource leaves the domain's nameservers as they
are: reverting a delegation would take the domain offline.

## Importing existing records

`getDnsRecords` lists a domain's records with their IDs. Import a record as
`<domain>/<recordId>`, and a domain's nameservers by the domain:

```sh
pulumi import porkbun:index:DnsRecord www example.com/123456789
pulumi import porkbun:index:DomainNameservers example example.com
```
