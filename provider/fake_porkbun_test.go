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
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	testAPIKey    = "pk1_test"
	testSecretKey = "sk1_test"
)

// fakePorkbun is an in-memory Porkbun API v3. It reproduces the quirks the
// provider has to handle: records come back with fully qualified names and
// numbers as strings, and writes may carry an Idempotency-Key.
type fakePorkbun struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	records  map[string]map[string]Record // domain -> id -> record
	ns       map[string][]string
	nextID   int
	requests []fakeRequest
	// throttle makes the next N requests fail with 429.
	throttle int
}

type fakeRequest struct {
	Path           string
	IdempotencyKey string
}

func newFakePorkbun(t *testing.T) *fakePorkbun {
	f := &fakePorkbun{
		t:       t,
		records: map[string]map[string]Record{},
		ns:      map[string][]string{},
		nextID:  100,
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakePorkbun) URL() string { return f.srv.URL }

func (f *fakePorkbun) seed(domain string, rec Record) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.records[domain] == nil {
		f.records[domain] = map[string]Record{}
	}
	f.records[domain][rec.ID] = rec
}

func (f *fakePorkbun) record(domain, id string) (Record, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.records[domain][id]
	return r, ok
}

func (f *fakePorkbun) count(domain string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records[domain])
}

func (f *fakePorkbun) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, fakeRequest{Path: r.URL.Path, IdempotencyKey: r.Header.Get("Idempotency-Key")})

	if f.throttle > 0 {
		f.throttle--
		w.Header().Set("Retry-After", "0")
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"status": "ERROR", "code": "RATE_LIMIT_EXCEEDED", "message": "slow down"})
		return
	}

	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body["apikey"] != testAPIKey || body["secretapikey"] != testSecretKey {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "ERROR", "code": "INVALID_API_KEYS", "message": "Invalid API key."})
		return
	}
	str := func(k string) string { s, _ := body[k].(string); return s }

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	ok := func(extra map[string]any) {
		out := map[string]any{"status": "SUCCESS", "requestId": "req-" + strconv.Itoa(len(f.requests))}
		for k, v := range extra {
			out[k] = v
		}
		writeJSON(w, http.StatusOK, out)
	}
	notFound := func() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "ERROR", "code": "RECORD_NOT_FOUND", "message": "Record not found."})
	}

	switch {
	case len(parts) == 1 && parts[0] == "ping":
		ok(map[string]any{"credentialsValid": true})

	case len(parts) == 3 && parts[0] == "dns" && parts[1] == "create":
		domain := parts[2]
		f.nextID++
		id := strconv.Itoa(f.nextID)
		if f.records[domain] == nil {
			f.records[domain] = map[string]Record{}
		}
		f.records[domain][id] = Record{
			ID: id, Name: fqdn(str("name"), domain), Type: str("type"), Content: str("content"),
			TTL: defaultString(str("ttl"), "600"), Prio: defaultString(str("prio"), "0"), Notes: str("notes"),
		}
		ok(map[string]any{"id": id})

	case len(parts) == 4 && parts[0] == "dns" && parts[1] == "edit":
		domain, id := parts[2], parts[3]
		rec, exists := f.records[domain][id]
		if !exists {
			notFound()
			return
		}
		if _, set := body["name"]; set {
			rec.Name = fqdn(str("name"), domain)
		}
		for k, dst := range map[string]*string{"type": &rec.Type, "content": &rec.Content, "ttl": &rec.TTL, "prio": &rec.Prio, "notes": &rec.Notes} {
			if _, set := body[k]; set {
				*dst = str(k)
			}
		}
		f.records[domain][id] = rec
		ok(nil)

	case len(parts) == 4 && parts[0] == "dns" && parts[1] == "delete":
		domain, id := parts[2], parts[3]
		if _, exists := f.records[domain][id]; !exists {
			notFound()
			return
		}
		delete(f.records[domain], id)
		ok(nil)

	case len(parts) >= 3 && parts[0] == "dns" && parts[1] == "retrieve":
		domain := parts[2]
		recs := []Record{}
		for id, rec := range f.records[domain] {
			if len(parts) == 4 && id != parts[3] {
				continue
			}
			recs = append(recs, rec)
		}
		// Porkbun answers a retrieve of a missing ID with an empty list.
		ok(map[string]any{"records": recs})

	case len(parts) == 3 && parts[0] == "domain" && parts[1] == "getNs":
		ok(map[string]any{"ns": f.ns[parts[2]]})

	case len(parts) == 3 && parts[0] == "domain" && parts[1] == "updateNs":
		var ns []string
		for _, v := range body["ns"].([]any) {
			ns = append(ns, v.(string))
		}
		f.ns[parts[2]] = ns
		ok(nil)

	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "ERROR", "message": "no such endpoint " + r.URL.Path})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func defaultString(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func (f *fakePorkbun) nameservers(domain string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ns[domain]
}
