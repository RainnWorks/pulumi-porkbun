# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-10-27

First release.

### Added

- `DnsRecord`: create, update in place, delete and import DNS records on a
  Porkbun domain. Import IDs are `<domain>/<recordId>`.
- `DomainNameservers`: set the nameservers a domain is delegated to. Deleting
  the resource leaves the nameservers unchanged.
- `getDnsRecords` and `getNameservers` functions.
- Provider configuration through `porkbun:apiKey` / `porkbun:secretApiKey` or
  `PORKBUN_API_KEY` / `PORKBUN_SECRET_API_KEY`.
- Idempotency keys on every write, and retries on rate limits (`Retry-After`)
  and server errors.
- SDKs for Node.js, Python, Go and .NET.

[Unreleased]: https://github.com/RainnWorks/pulumi-porkbun/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/RainnWorks/pulumi-porkbun/releases/tag/v0.1.0
