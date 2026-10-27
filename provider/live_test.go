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
	"os"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	integration "github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// TestLiveReadOnly runs against the real Porkbun API. It only reads: it lists
// a domain's records and nameservers and imports each record, which must
// round-trip without error. Set PORKBUN_API_KEY, PORKBUN_SECRET_API_KEY and
// PORKBUN_LIVE_DOMAIN to run it.
func TestLiveReadOnly(t *testing.T) {
	domain := os.Getenv("PORKBUN_LIVE_DOMAIN")
	if domain == "" || os.Getenv("PORKBUN_API_KEY") == "" || os.Getenv("PORKBUN_SECRET_API_KEY") == "" {
		t.Skip("set PORKBUN_API_KEY, PORKBUN_SECRET_API_KEY and PORKBUN_LIVE_DOMAIN to run against the real API")
	}

	server, err := integration.NewServer(t.Context(), Name, semver.Version{Minor: 1}, integration.WithProvider(Provider()))
	require.NoError(t, err)
	require.NoError(t, server.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{
		"apiKey":       property.New(os.Getenv("PORKBUN_API_KEY")),
		"secretApiKey": property.New(os.Getenv("PORKBUN_SECRET_API_KEY")),
	})}))

	ns, err := server.Invoke(p.InvokeRequest{Token: "porkbun:index:getNameservers", Args: inputs(map[string]any{"domain": domain})})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(ns.Return.Get("nameservers").AsArray().AsSlice()), 2)

	recs, err := server.Invoke(p.InvokeRequest{Token: "porkbun:index:getDnsRecords", Args: inputs(map[string]any{"domain": domain})})
	require.NoError(t, err)
	for _, v := range recs.Return.Get("records").AsArray().AsSlice() {
		rec := v.AsMap()
		id := domain + "/" + rec.Get("recordId").AsString()
		resp, err := server.Read(p.ReadRequest{ID: id, Urn: urn("porkbun:index:DnsRecord")})
		require.NoError(t, err, id)
		assert.Equal(t, id, resp.ID)
		assert.Equal(t, rec.Get("content").AsString(), resp.Inputs.Get("content").AsString(), id)
		t.Logf("%-6s %-40s ok", rec.Get("type").AsString(), rec.Get("fqdn").AsString())
	}
}
