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
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	integration "github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func newServer(t *testing.T, fake *fakePorkbun) integration.Server {
	t.Helper()
	server, err := integration.NewServer(t.Context(), Name, semver.Version{Minor: 1},
		integration.WithProvider(Provider()))
	require.NoError(t, err)
	require.NoError(t, server.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{
		"apiKey":       property.New(testAPIKey),
		"secretApiKey": property.New(testSecretKey),
		"baseUrl":      property.New(fake.URL()),
	})}))
	return server
}

func inputs(m map[string]any) property.Map {
	return presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(m))
}

func urn(typ string) presource.URN {
	return presource.NewURN("test", "provider", "", tokens.Type(typ), "test")
}

func TestDnsRecordLifecycle(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	server := newServer(t, fake)

	integration.LifeCycleTest{
		Resource: "porkbun:index:DnsRecord",
		Create: integration.Operation{
			Inputs: inputs(map[string]any{
				"domain": "example.com", "name": "www", "type": "A", "content": "203.0.113.10",
			}),
			Hook: func(_, out property.Map) {
				assert.Equal(t, "www.example.com", out.Get("fqdn").AsString())
				assert.Equal(t, 600.0, out.Get("ttl").AsNumber())
				rec, ok := fake.record("example.com", out.Get("recordId").AsString())
				require.True(t, ok, "record should exist in Porkbun")
				assert.Equal(t, "www.example.com", rec.Name)
				assert.Equal(t, "203.0.113.10", rec.Content)
				assert.Equal(t, "600", rec.TTL)
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: inputs(map[string]any{
					"domain": "example.com", "name": "www", "type": "A", "content": "203.0.113.20",
					"ttl": 3600, "notes": "moved",
				}),
				Hook: func(_, out property.Map) {
					rec, _ := fake.record("example.com", out.Get("recordId").AsString())
					assert.Equal(t, "203.0.113.20", rec.Content)
					assert.Equal(t, "3600", rec.TTL)
					assert.Equal(t, "moved", rec.Notes)
				},
			},
			{
				// Dropping the note must clear it in Porkbun, not leave it behind.
				Inputs: inputs(map[string]any{
					"domain": "example.com", "name": "www", "type": "A", "content": "203.0.113.20", "ttl": 3600,
				}),
				Hook: func(_, out property.Map) {
					rec, _ := fake.record("example.com", out.Get("recordId").AsString())
					assert.Equal(t, "", rec.Notes)
				},
			},
		},
	}.Run(t, server)

	assert.Equal(t, 0, fake.count("example.com"), "the record should be deleted at the end of the lifecycle")
}

func TestDnsRecordApexAndPriority(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	server := newServer(t, fake)

	integration.LifeCycleTest{
		Resource: "porkbun:index:DnsRecord",
		Create: integration.Operation{
			Inputs: inputs(map[string]any{
				"domain": "example.com", "type": "MX", "content": "mail.example.com", "prio": 10,
			}),
			Hook: func(_, out property.Map) {
				assert.Equal(t, "example.com", out.Get("fqdn").AsString())
				rec, _ := fake.record("example.com", out.Get("recordId").AsString())
				assert.Equal(t, "example.com", rec.Name)
				assert.Equal(t, "10", rec.Prio)
			},
		},
	}.Run(t, server)
}

func TestDnsRecordImport(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	fake.seed("example.com", Record{ID: "42", Name: "*.example.com", Type: "A", Content: "203.0.113.10", TTL: "600", Prio: "0"})
	fake.seed("example.com", Record{ID: "43", Name: "example.com", Type: "MX", Content: "smtp.google.com", TTL: "600", Prio: "1"})
	server := newServer(t, fake)

	resp, err := server.Read(p.ReadRequest{ID: "example.com/42", Urn: urn("porkbun:index:DnsRecord")})
	require.NoError(t, err)
	assert.Equal(t, "example.com/42", resp.ID)
	assert.Equal(t, "*", resp.Inputs.Get("name").AsString())
	assert.Equal(t, "A", resp.Inputs.Get("type").AsString())
	assert.Equal(t, 600.0, resp.Inputs.Get("ttl").AsNumber())
	_, hasPrio := resp.Inputs.GetOk("prio")
	assert.False(t, hasPrio, "a zero priority should not appear as an input")
	_, hasNotes := resp.Inputs.GetOk("notes")
	assert.False(t, hasNotes, "an empty note should not appear as an input")

	resp, err = server.Read(p.ReadRequest{ID: "example.com/43", Urn: urn("porkbun:index:DnsRecord")})
	require.NoError(t, err)
	_, hasName := resp.Inputs.GetOk("name")
	assert.False(t, hasName, "an apex record has no name input, as in a program that leaves it out")
	assert.Equal(t, 1.0, resp.Inputs.Get("prio").AsNumber())
}

func TestDnsRecordReadMissing(t *testing.T) {
	t.Parallel()
	server := newServer(t, newFakePorkbun(t))

	resp, err := server.Read(p.ReadRequest{ID: "example.com/999", Urn: urn("porkbun:index:DnsRecord")})
	require.NoError(t, err)
	assert.Empty(t, resp.ID, "a record deleted outside Pulumi should drop out of state")
}

func TestDnsRecordBadImportID(t *testing.T) {
	t.Parallel()
	server := newServer(t, newFakePorkbun(t))

	_, err := server.Read(p.ReadRequest{ID: "123456", Urn: urn("porkbun:index:DnsRecord")})
	assert.ErrorContains(t, err, "<domain>/<recordId>")
}

func TestDomainNameserversLifecycle(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	fake.ns["example.com"] = []string{"curitiba.ns.porkbun.com", "fortaleza.ns.porkbun.com"}
	server := newServer(t, fake)

	integration.LifeCycleTest{
		Resource: "porkbun:index:DomainNameservers",
		Create: integration.Operation{
			Inputs: inputs(map[string]any{
				"domain": "example.com", "nameservers": []any{"ada.ns.cloudflare.com", "bob.ns.cloudflare.com"},
			}),
			Hook: func(_, _ property.Map) {
				assert.Equal(t, []string{"ada.ns.cloudflare.com", "bob.ns.cloudflare.com"}, fake.nameservers("example.com"))
			},
		},
		Updates: []integration.Operation{{
			Inputs: inputs(map[string]any{
				"domain": "example.com", "nameservers": []any{"cy.ns.cloudflare.com", "dee.ns.cloudflare.com"},
			}),
			Hook: func(_, _ property.Map) {
				assert.Equal(t, []string{"cy.ns.cloudflare.com", "dee.ns.cloudflare.com"}, fake.nameservers("example.com"))
			},
		}},
	}.Run(t, server)

	assert.Equal(t, []string{"cy.ns.cloudflare.com", "dee.ns.cloudflare.com"}, fake.nameservers("example.com"),
		"deleting the resource must leave the delegation alone")
}

func TestDomainNameserversValidation(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	server := newServer(t, fake)

	_, err := server.Create(p.CreateRequest{
		Urn:        urn("porkbun:index:DomainNameservers"),
		Properties: inputs(map[string]any{"domain": "example.com", "nameservers": []any{"only.one.example"}}),
	})
	assert.ErrorContains(t, err, "at least two")
	assert.Nil(t, fake.nameservers("example.com"), "nothing should be sent to Porkbun")
}

func TestDomainNameserversDiffIgnoresOrderAndCase(t *testing.T) {
	t.Parallel()
	server := newServer(t, newFakePorkbun(t))

	state := inputs(map[string]any{"domain": "example.com", "nameservers": []any{"a.ns.example", "b.ns.example"}})
	resp, err := server.Diff(p.DiffRequest{
		ID: "example.com", Urn: urn("porkbun:index:DomainNameservers"), State: state, OldInputs: state,
		Inputs: inputs(map[string]any{"domain": "example.com", "nameservers": []any{"B.ns.example.", "a.ns.example"}}),
	})
	require.NoError(t, err)
	assert.False(t, resp.HasChanges)
}

func TestDomainNameserversImport(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	fake.ns["example.com"] = []string{"curitiba.ns.porkbun.com", "fortaleza.ns.porkbun.com"}
	server := newServer(t, fake)

	resp, err := server.Read(p.ReadRequest{ID: "example.com", Urn: urn("porkbun:index:DomainNameservers")})
	require.NoError(t, err)
	assert.Equal(t, "example.com", resp.Inputs.Get("domain").AsString())
	assert.Len(t, resp.Inputs.Get("nameservers").AsArray().AsSlice(), 2)
}

func TestGetDnsRecords(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	fake.seed("example.com", Record{ID: "42", Name: "www.example.com", Type: "CNAME", Content: "example.com", TTL: "600", Prio: "0"})
	server := newServer(t, fake)

	resp, err := server.Invoke(p.InvokeRequest{
		Token: "porkbun:index:getDnsRecords",
		Args:  inputs(map[string]any{"domain": "example.com"}),
	})
	require.NoError(t, err)
	recs := resp.Return.Get("records").AsArray().AsSlice()
	require.Len(t, recs, 1)
	rec := recs[0].AsMap()
	assert.Equal(t, "42", rec.Get("recordId").AsString())
	assert.Equal(t, "www", rec.Get("name").AsString())
	assert.Equal(t, "www.example.com", rec.Get("fqdn").AsString())
}

func TestRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	fake.throttle = 2
	server := newServer(t, fake)

	resp, err := server.Create(p.CreateRequest{
		Urn:        urn("porkbun:index:DnsRecord"),
		Properties: inputs(map[string]any{"domain": "example.com", "type": "TXT", "content": "hello", "ttl": 600}),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, 1, fake.count("example.com"), "a retried create must not make two records")

	require.Len(t, fake.requests, 3)
	key := fake.requests[0].IdempotencyKey
	assert.NotEmpty(t, key)
	for _, r := range fake.requests {
		assert.Equal(t, key, r.IdempotencyKey, "retries must reuse the idempotency key")
	}
}

func TestBadCredentials(t *testing.T) {
	t.Parallel()
	fake := newFakePorkbun(t)
	server, err := integration.NewServer(t.Context(), Name, semver.Version{Minor: 1}, integration.WithProvider(Provider()))
	require.NoError(t, err)
	require.NoError(t, server.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{
		"apiKey": property.New("pk1_wrong"), "secretApiKey": property.New("sk1_wrong"), "baseUrl": property.New(fake.URL()),
	})}))

	_, err = server.Invoke(p.InvokeRequest{
		Token: "porkbun:index:getNameservers",
		Args:  inputs(map[string]any{"domain": "example.com"}),
	})
	assert.ErrorContains(t, err, "INVALID_API_KEYS")
}

func TestSubdomain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, domain, want string }{
		{"example.com", "example.com", ""},
		{"www.example.com", "example.com", "www"},
		{"*.example.com", "example.com", "*"},
		{"a.b.example.com.", "example.com", "a.b"},
		{"WWW.Example.com", "example.com", "WWW"},
		{"notexample.com", "example.com", "notexample.com"},
	} {
		assert.Equal(t, tc.want, subdomain(tc.name, tc.domain), tc.name)
	}
}

func TestCreateIDAcceptsNumberOrString(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"id":"123"}`, `{"id":123}`} {
		var out struct {
			ID json.Number `json:"id"`
		}
		require.NoError(t, json.Unmarshal([]byte(raw), &out))
		assert.Equal(t, "123", out.ID.String())
	}
}
