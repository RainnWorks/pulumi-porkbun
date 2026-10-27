# Porkbun Resource Provider

[![Build](https://github.com/RainnWorks/pulumi-porkbun/actions/workflows/build.yml/badge.svg)](https://github.com/RainnWorks/pulumi-porkbun/actions/workflows/build.yml)
[![npm](https://img.shields.io/npm/v/@rainnworks/porkbun.svg)](https://www.npmjs.com/package/@rainnworks/porkbun)
[![PyPI](https://img.shields.io/pypi/v/rainnworks-porkbun.svg)](https://pypi.org/project/rainnworks-porkbun)
[![NuGet](https://img.shields.io/nuget/v/RainnWorks.Porkbun.svg)](https://www.nuget.org/packages/RainnWorks.Porkbun)
[![Go Reference](https://pkg.go.dev/badge/github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun.svg)](https://pkg.go.dev/github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun)
[![License](https://img.shields.io/github/license/RainnWorks/pulumi-porkbun)](LICENSE)

A [Pulumi](https://www.pulumi.com) provider for [Porkbun](https://porkbun.com): manage
DNS records and the nameservers your domains are delegated to, in TypeScript,
Python, Go, C# or YAML.

It is a native provider built on the [Porkbun API v3](https://porkbun.com/api/json/v3/documentation)
with [pulumi-go-provider](https://github.com/pulumi/pulumi-go-provider). It is not
affiliated with Porkbun.

## What it manages

| | |
|---|---|
| `porkbun.DnsRecord` | One DNS record: `A`, `AAAA`, `CNAME`, `ALIAS`, `MX`, `TXT`, `NS`, `SRV`, `TLSA`, `CAA`, `HTTPS`, `SVCB` |
| `porkbun.DomainNameservers` | The nameservers a domain is delegated to, e.g. to move its DNS to Cloudflare while Porkbun stays the registrar |
| `porkbun.getDnsRecords` | Every record on a domain, with the IDs you need to import them |
| `porkbun.getNameservers` | A domain's current nameservers |

Existing records and nameservers can be [imported](#importing-what-you-already-have) without
changing them.

## Installing

The provider plugin downloads automatically the first time a program uses it.
Install the SDK for your language:

**Node.js** (TypeScript or JavaScript)

```sh
npm install @rainnworks/porkbun
```

**Python**

```sh
pip install rainnworks-porkbun
```

**Go**

```sh
go get github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun
```

**.NET**

```sh
dotnet add package RainnWorks.Porkbun
```

**YAML**: nothing to install. Refer to the resources as `porkbun:DnsRecord` and
`porkbun:DomainNameservers`, and add the plugin to `Pulumi.yaml`:

```yaml
plugins:
  providers:
    - name: porkbun
      version: 0.1.0
```

or install it by hand with
`pulumi plugin install resource porkbun --server github://api.github.com/RainnWorks/pulumi-porkbun`.

## Configuring

You need an API key pair from [porkbun.com/account/api](https://porkbun.com/account/api),
and **API access turned on for each domain** you manage (Domain Management → the
domain → Details → API Access). Without that, every call for the domain fails.

Set the keys as stack configuration, as secrets:

```sh
pulumi config set porkbun:apiKey pk1_... --secret
pulumi config set porkbun:secretApiKey sk1_... --secret
```

or in the environment, which keeps them out of the stack entirely. When both are
set, stack configuration wins.

```sh
export PORKBUN_API_KEY=pk1_...
export PORKBUN_SECRET_API_KEY=sk1_...
```

| Option | Environment variable | |
|---|---|---|
| `porkbun:apiKey` | `PORKBUN_API_KEY` | Required. Secret |
| `porkbun:secretApiKey` | `PORKBUN_SECRET_API_KEY` | Required. Secret |
| `porkbun:baseUrl` | `PORKBUN_BASE_URL` | Optional. Only for testing against another endpoint |

## Example

```typescript
import * as porkbun from "@rainnworks/porkbun";

const domain = "example.com";

new porkbun.DnsRecord("apex", { domain, type: "A", content: "203.0.113.10" });
new porkbun.DnsRecord("www", { domain, name: "www", type: "CNAME", content: domain });
new porkbun.DnsRecord("mx", { domain, type: "MX", content: "mail.example.com", prio: 10 });
new porkbun.DnsRecord("spf", { domain, type: "TXT", content: "v=spf1 mx ~all" });
```

The same program in every language is in [`examples/`](examples/).

### Moving a domain's DNS to Cloudflare

Keep Porkbun as the registrar and hand DNS to Cloudflare: create the zone, then
point the domain at the nameservers Cloudflare assigned.

```typescript
import * as cloudflare from "@pulumi/cloudflare";
import * as porkbun from "@rainnworks/porkbun";

const zone = new cloudflare.Zone("example", {
    account: { id: accountId },
    name: "example.com",
});

new porkbun.DomainNameservers("example", {
    domain: "example.com",
    nameservers: zone.nameServers,
});
```

Copy every record into the new zone *before* the nameservers change, and check
the copy, or the domain stops resolving for as long as the gap lasts.

**Deleting a `DomainNameservers` resource does not change the domain's
nameservers.** Reverting a delegation would take the domain offline, so removing
the resource only stops Pulumi managing it.

## Importing what you already have

List a domain's records and their IDs:

```typescript
const records = porkbun.getDnsRecordsOutput({ domain: "example.com" });
export const ids = records.records.apply(rs => rs.map(r => `${r.fqdn} ${r.type} ${r.recordId}`));
```

then import each one as `<domain>/<recordId>`, and the nameservers by domain:

```sh
pulumi import porkbun:index:DnsRecord www example.com/123456789
pulumi import porkbun:index:DomainNameservers example example.com
```

`pulumi import` prints the code for what it imported. A `pulumi preview` straight
after should show no changes.

## How it behaves

- **Names are subdomains.** `name: "www"` on `example.com` is `www.example.com`;
  leave `name` out for the apex and use `*` for a wildcard. The `fqdn` output has
  the full name.
- **TTL** defaults to 600 seconds, Porkbun's minimum.
- **Edits happen in place.** Only changing `domain` replaces a record.
- **Removing an input clears it.** Drop `notes` or `prio` from a record and the
  next update clears it in Porkbun, rather than leaving the old value behind.
- **Refresh is quiet about cosmetic differences.** Letter case, an empty note and
  a zero priority read back the way your program wrote them.
- **Writes are safe to retry.** Each write carries an `Idempotency-Key`, so a
  retry after a dropped connection cannot create a second record. Rate-limited
  calls wait for `Retry-After` and try again.
- **Porkbun's warnings are passed on.** For example, Porkbun accepts records for
  a domain delegated elsewhere, where they won't resolve, and says so.

## Reference

Full documentation for every resource, function and property is on the
[Pulumi Registry](https://www.pulumi.com/registry/packages/porkbun/) once
published, and in [`docs/`](docs/).

## Developing

See [CONTRIBUTING.md](CONTRIBUTING.md). In short:

```sh
make provider   # build bin/pulumi-resource-porkbun
make test       # unit tests against an in-memory Porkbun
make codegen    # regenerate the schema and the SDKs after changing the provider
```

## License

[Apache 2.0](LICENSE).
