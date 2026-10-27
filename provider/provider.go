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

// Package provider implements the Porkbun provider for Pulumi.
package provider

import (
	"context"
	"errors"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Version is initialized by the Go linker to contain the semver of this build.
var Version string

// Name controls how this provider is referenced in package names and elsewhere.
const Name string = "porkbun"

// Provider creates a new instance of the provider.
func Provider() p.Provider {
	prov, err := infer.NewProviderBuilder().
		WithDisplayName("Porkbun").
		WithDescription("A Pulumi provider for Porkbun: DNS records and domain nameservers.").
		WithKeywords("porkbun", "dns", "domain", "nameservers", "category/network", "kind/native").
		WithHomepage("https://github.com/RainnWorks/pulumi-porkbun").
		WithRepository("https://github.com/RainnWorks/pulumi-porkbun").
		WithPublisher("RainnWorks").
		WithLicense("Apache-2.0").
		WithNamespace("rainnworks").
		WithPluginDownloadURL("github://api.github.com/RainnWorks/pulumi-porkbun").
		WithGoImportPath("github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun").
		WithLanguageMap(map[string]any{
			"nodejs": map[string]any{
				"packageName":          "@rainnworks/porkbun",
				"packageDescription":   "A Pulumi provider for Porkbun: DNS records and domain nameservers.",
				"respectSchemaVersion": true,
			},
			"python": map[string]any{
				"packageName":          "rainnworks_porkbun",
				"respectSchemaVersion": true,
				"pyproject":            map[string]any{"enabled": true},
			},
			"csharp": map[string]any{
				"rootNamespace":        "RainnWorks",
				"respectSchemaVersion": true,
			},
			"go": map[string]any{
				"importBasePath":                 "github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun",
				"generateResourceContainerTypes": true,
				"respectSchemaVersion":           true,
			},
		}).
		WithResources(
			infer.Resource(DnsRecord{}),
			infer.Resource(DomainNameservers{}),
		).
		WithFunctions(
			infer.Function(GetDnsRecords{}),
			infer.Function(GetNameservers{}),
		).
		WithConfig(infer.Config(&Config{})).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		}).Build()
	if err != nil {
		panic(fmt.Errorf("unable to build provider: %w", err))
	}
	return prov
}

// Config is the provider-level configuration.
type Config struct {
	// Optional in the schema so that the environment variables alone are enough;
	// Configure insists on both.
	APIKey       string `pulumi:"apiKey,optional" provider:"secret"`
	SecretAPIKey string `pulumi:"secretApiKey,optional" provider:"secret"`
	BaseURL      string `pulumi:"baseUrl,optional"`

	client *Client
}

// Annotate describes the configuration and wires its environment variables.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.APIKey, "Porkbun API key (`pk1_…`). Can also be set with the `PORKBUN_API_KEY` environment variable.")
	a.SetDefault(&c.APIKey, nil, "PORKBUN_API_KEY")
	a.Describe(&c.SecretAPIKey, "Porkbun secret API key (`sk1_…`). Can also be set with the `PORKBUN_SECRET_API_KEY` environment variable.")
	a.SetDefault(&c.SecretAPIKey, nil, "PORKBUN_SECRET_API_KEY")
	a.Describe(&c.BaseURL, "Override the Porkbun API endpoint. Defaults to `"+DefaultBaseURL+"`; only useful for testing.")
	a.SetDefault(&c.BaseURL, nil, "PORKBUN_BASE_URL")
}

// Configure builds the API client once per provider process.
func (c *Config) Configure(_ context.Context) error {
	if c.APIKey == "" || c.SecretAPIKey == "" {
		return errors.New("porkbun: apiKey and secretApiKey are required; set them in the stack's " +
			"`porkbun:` config or with PORKBUN_API_KEY and PORKBUN_SECRET_API_KEY")
	}
	c.client = NewClient(c.BaseURL, c.APIKey, c.SecretAPIKey, "pulumi-porkbun/"+Version)
	return nil
}

func clientFrom(ctx context.Context) *Client {
	return infer.GetConfig[Config](ctx).client
}
