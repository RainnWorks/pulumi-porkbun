---
title: Porkbun Installation & Configuration
meta_desc: Information on how to install and configure the Porkbun provider for Pulumi.
layout: package
---

## Installation

The Porkbun provider is available as a package in these languages:

* JavaScript/TypeScript: [`@rainnworks/porkbun`](https://www.npmjs.com/package/@rainnworks/porkbun)
* Python: [`rainnworks-porkbun`](https://pypi.org/project/rainnworks-porkbun/)
* Go: [`github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun`](https://pkg.go.dev/github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun)
* .NET: [`RainnWorks.Porkbun`](https://www.nuget.org/packages/RainnWorks.Porkbun)

The provider plugin is published on [GitHub Releases](https://github.com/RainnWorks/pulumi-porkbun/releases)
and is downloaded automatically. To install it by hand:

```sh
pulumi plugin install resource porkbun --server github://api.github.com/RainnWorks/pulumi-porkbun
```

## Setup

1. Create an API key pair at [porkbun.com/account/api](https://porkbun.com/account/api).
2. Turn on **API Access** for every domain the provider will manage: Domain
   Management, then the domain's Details, then API Access. Calls for a domain
   without it are refused.

## Configuration options

| Option | Environment variable | Required | Description |
|---|---|---|---|
| `porkbun:apiKey` | `PORKBUN_API_KEY` | Yes | API key, `pk1_…`. Secret. |
| `porkbun:secretApiKey` | `PORKBUN_SECRET_API_KEY` | Yes | Secret API key, `sk1_…`. Secret. |
| `porkbun:baseUrl` | `PORKBUN_BASE_URL` | No | API endpoint. Defaults to `https://api.porkbun.com/api/json/v3`. For testing only. |

Set them as encrypted stack configuration:

```sh
pulumi config set porkbun:apiKey pk1_... --secret
pulumi config set porkbun:secretApiKey sk1_... --secret
```

or in the environment:

```sh
export PORKBUN_API_KEY=pk1_...
export PORKBUN_SECRET_API_KEY=sk1_...
```

Stack configuration wins when both are set.

## Multiple Porkbun accounts

Create an explicit provider for each account and pass it to the resources:

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as porkbun from "@rainnworks/porkbun";

const config = new pulumi.Config();

const other = new porkbun.Provider("other-account", {
    apiKey: config.requireSecret("otherApiKey"),
    secretApiKey: config.requireSecret("otherSecretApiKey"),
});

new porkbun.DnsRecord("www", {
    domain: "example.org",
    name: "www",
    type: "A",
    content: "203.0.113.10",
}, { provider: other });
```
