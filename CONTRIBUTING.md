# Contributing

Issues and pull requests are welcome. For anything larger than a fix, open an
issue first so we can agree the shape before you write it.

## Layout

| Path | What |
|---|---|
| `provider/` | The provider, in Go, built on [pulumi-go-provider](https://github.com/pulumi/pulumi-go-provider)'s `infer` package |
| `provider/client.go` | The Porkbun API client |
| `provider/cmd/pulumi-resource-porkbun/schema.json` | The Pulumi schema. **Generated** |
| `sdk/` | The SDKs for every language. **Generated**, and committed so that the Go SDK can be fetched straight from this repo |
| `docs/` | The Pulumi Registry pages |
| `examples/` | One program per language |

Never edit generated files by hand. Change the provider and run `make codegen`.

## Prerequisites

- Go (the version in `go.mod`)
- The [Pulumi CLI](https://www.pulumi.com/docs/install/)
- To build the SDKs: Node.js, Python 3 and the .NET 8 SDK

## Making a change

```sh
make provider   # build bin/pulumi-resource-porkbun
make test       # unit tests, against an in-memory fake of the Porkbun API
make codegen    # regenerate the schema and the SDKs; commit the result
```

CI fails if the generated files don't match the provider, so run `make codegen`
whenever you change a resource's inputs, outputs or descriptions.

The unit tests never touch the real API. `provider/fake_porkbun_test.go` is the
fake; when Porkbun behaves in a way the provider has to handle, teach the fake
the same behaviour and add a test.

### Against the real API

`make test_live` runs a read-only test against a real domain: it lists the
records and nameservers and imports each record, and changes nothing.

```sh
PORKBUN_API_KEY=pk1_... PORKBUN_SECRET_API_KEY=sk1_... PORKBUN_LIVE_DOMAIN=example.com make test_live
```

### Trying it in a program

```sh
make provider
export PATH="$PWD/bin:$PATH"   # Pulumi finds pulumi-resource-porkbun on PATH
cd examples/yaml && pulumi preview
```

## Releasing

1. `make release VERSION=X.Y.Z`. This writes `.version` and regenerates the
   schema and SDKs at that version.
2. Move the `[Unreleased]` notes in `CHANGELOG.md` under a new `## [X.Y.Z]`
   heading. They become the GitHub release notes.
3. Commit, open a PR, merge.
4. Tag the merge commit `vX.Y.Z` and push the tag.

The `release` workflow then checks that the tag matches `.version`, publishes
the provider plugin to GitHub Releases, tags the Go SDK `sdk/vX.Y.Z`, and
publishes to npm, PyPI and NuGet.

### Registry credentials (maintainers)

| Registry | Set up | Until then |
|---|---|---|
| npm | Repository secret `NPM_TOKEN`: an automation token that can publish `@rainnworks/porkbun` | Skipped with a warning |
| PyPI | A [trusted publisher](https://docs.pypi.org/trusted-publishers/) for `rainnworks-porkbun`: this repo, workflow `release.yml`, environment `pypi`. Then repository variable `PYPI_ENABLED=true` | Skipped |
| NuGet | Repository secret `NUGET_API_KEY`, scoped to push `RainnWorks.Porkbun` | Skipped with a warning |
| Go | Nothing: the workflow pushes the `sdk/vX.Y.Z` tag | |

## Code of conduct

Be kind, assume good faith, and keep discussion about the work.
