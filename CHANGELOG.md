# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2026-10-27

The first release on NuGet. No change to the provider.

### Changed

- Every registry now publishes by trusted publishing (OIDC): npm, PyPI and
  NuGet. No registry tokens are stored.
- The Go SDK tag `sdk/vX.Y.Z` is pushed with the release tag; the release
  workflow checks it.

## [0.1.1] - 2026-10-27

The first release published to npm, PyPI and NuGet. No change to the provider.

### Fixed

- Release pipeline: the npm, PyPI and NuGet jobs. v0.1.0 reached GitHub Releases
  and the Go module proxy only.

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

[Unreleased]: https://github.com/RainnWorks/pulumi-porkbun/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/RainnWorks/pulumi-porkbun/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/RainnWorks/pulumi-porkbun/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/RainnWorks/pulumi-porkbun/releases/tag/v0.1.0
