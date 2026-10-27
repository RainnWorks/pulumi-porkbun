PACK            := porkbun
PROJECT         := github.com/RainnWorks/pulumi-porkbun
PROVIDER        := pulumi-resource-$(PACK)
SCHEMA_FILE     := provider/cmd/$(PROVIDER)/schema.json
WORKING_DIR     := $(shell pwd)

# Defaults to .version, the version the committed SDKs carry. `make release` changes it.
VERSION         ?= $(shell cat .version)
# SDKs carry the version without the "v" prefix and without build metadata.
SDK_VERSION     := $(patsubst v%,%,$(VERSION))

PULUMI          ?= pulumi
GO_TEST         := go test -count=1 -cover -timeout 30m

# Use the pinned language plugins, not whatever is on PATH.
export PULUMI_IGNORE_AMBIENT_PLUGINS = true

.PHONY: all
all: provider codegen

## provider: build the provider binary into bin/
.PHONY: provider
provider:
	go build -o $(WORKING_DIR)/bin/$(PROVIDER) \
		-ldflags "-X $(PROJECT)/provider.Version=$(SDK_VERSION)" \
		./provider/cmd/$(PROVIDER)

## schema: regenerate the Pulumi schema from the provider
.PHONY: schema
schema: provider
	$(PULUMI) package get-schema $(WORKING_DIR)/bin/$(PROVIDER) | \
		python3 -c 'import json,sys; s=json.load(sys.stdin); s.pop("version",None); print(json.dumps(s,indent=2,ensure_ascii=False))' \
		> $(SCHEMA_FILE)

## codegen: regenerate the schema and every SDK
.PHONY: codegen
codegen: schema sdk/nodejs sdk/python sdk/go sdk/dotnet

.PHONY: sdk/nodejs sdk/python sdk/go sdk/dotnet
sdk/nodejs: schema
	rm -rf sdk/nodejs
	$(PULUMI) package gen-sdk --language nodejs $(SCHEMA_FILE) --version "$(SDK_VERSION)"

sdk/dotnet: schema
	rm -rf sdk/dotnet
	$(PULUMI) package gen-sdk --language dotnet $(SCHEMA_FILE) --version "$(SDK_VERSION)"

sdk/python: schema
	rm -rf sdk/python
	$(PULUMI) package gen-sdk --language python $(SCHEMA_FILE) --version "$(SDK_VERSION)"
	cp README.md sdk/python/

# The Go SDK is its own module, github.com/RainnWorks/pulumi-porkbun/sdk, so
# that programs don't pull in the provider's dependencies. Tag it sdk/vX.Y.Z.
sdk/go: schema
	rm -rf sdk/go
	$(PULUMI) package gen-sdk --language go $(SCHEMA_FILE) --version "$(SDK_VERSION)"
	cd sdk && go mod tidy

## build_sdks: compile the generated SDKs (needs bun, python3, dotnet)
.PHONY: build_sdks build_nodejs build_python build_dotnet build_go
build_sdks: build_nodejs build_python build_dotnet build_go

build_nodejs: sdk/nodejs
	cd sdk/nodejs && bun install && bun run --bun build
	cp README.md LICENSE sdk/nodejs/package.json sdk/nodejs/bin/

build_python: sdk/python
	cd sdk/python && python3 -m venv venv && ./venv/bin/python -m pip install -q build && ./venv/bin/python -m build .

build_dotnet: sdk/dotnet
	cd sdk/dotnet && echo "$(SDK_VERSION)" > version.txt && dotnet build

build_go: sdk/go
	cd sdk && go build ./...

## release: set the version and regenerate everything for it (VERSION=X.Y.Z)
.PHONY: release
release:
	@echo "$(SDK_VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$' || (echo "VERSION must be X.Y.Z" && exit 1)
	echo "$(SDK_VERSION)" > .version
	$(MAKE) codegen VERSION=$(SDK_VERSION)
	@echo "Now: update CHANGELOG.md, commit, then: git tag v$(SDK_VERSION) && git tag sdk/v$(SDK_VERSION) && git push origin v$(SDK_VERSION) sdk/v$(SDK_VERSION)"

## test: unit tests against an in-memory Porkbun
.PHONY: test
test:
	$(GO_TEST) -race ./provider/...

## test_live: read-only test against the real API (needs PORKBUN_API_KEY, PORKBUN_SECRET_API_KEY, PORKBUN_LIVE_DOMAIN)
.PHONY: test_live
test_live:
	$(GO_TEST) -run TestLiveReadOnly -v ./provider/...

## lint: go vet and golangci-lint
.PHONY: lint
lint:
	go vet ./...
	golangci-lint run ./...

## install: put the provider binary on PATH for local programs
.PHONY: install
install: provider
	cp $(WORKING_DIR)/bin/$(PROVIDER) $(shell go env GOPATH)/bin

## check_clean: fail if codegen left changes (run after codegen in CI)
.PHONY: check_clean
check_clean:
	@git diff --exit-code -- $(SCHEMA_FILE) sdk/ || \
		(echo "Generated code is out of date: run 'make codegen' and commit the result." && exit 1)

.PHONY: clean
clean:
	rm -rf bin sdk/nodejs/bin sdk/python/venv sdk/python/dist sdk/dotnet/bin sdk/dotnet/obj

.PHONY: help
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
