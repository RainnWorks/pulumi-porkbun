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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the Porkbun API v3 endpoint.
const DefaultBaseURL = "https://api.porkbun.com/api/json/v3"

// maxAttempts bounds retries of rate-limited (429) and transient (5xx) responses.
const maxAttempts = 5

// Client is a minimal Porkbun API v3 client covering DNS records and nameservers.
type Client struct {
	baseURL      string
	apiKey       string
	secretAPIKey string
	http         *http.Client
	userAgent    string
}

// NewClient returns a client for the given credentials. An empty baseURL means DefaultBaseURL.
func NewClient(baseURL, apiKey, secretAPIKey, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		secretAPIKey: secretAPIKey,
		http:         &http.Client{Timeout: 60 * time.Second},
		userAgent:    userAgent,
	}
}

// APIError is an error returned by the Porkbun API.
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.HTTPStatus)
	}
	s := "porkbun: " + msg
	if e.Code != "" {
		s += " (" + e.Code + ")"
	}
	if e.RequestID != "" {
		s += " [request " + e.RequestID + "]"
	}
	return s
}

// IsNotFound reports whether err means the record or domain does not exist.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.HTTPStatus == http.StatusNotFound || strings.Contains(strings.ToUpper(apiErr.Code), "NOT_FOUND") {
		return true
	}
	// Older responses carry no code; retrieving a missing record ID returns an
	// empty record list instead, which callers handle separately.
	return strings.Contains(strings.ToLower(apiErr.Message), "not found")
}

// envelope holds the fields every Porkbun response shares.
type envelope struct {
	Status    string   `json:"status"`
	Message   string   `json:"message"`
	Code      string   `json:"code"`
	RequestID string   `json:"requestId"`
	Warnings  []string `json:"warnings"`
}

// Record is a DNS record as the API returns it. Porkbun encodes numbers as strings.
type Record struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     string `json:"ttl"`
	Prio    string `json:"prio"`
	Notes   string `json:"notes"`
}

// RecordInput is the body for creating or editing a record.
type RecordInput struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Content string  `json:"content"`
	TTL     string  `json:"ttl,omitempty"`
	Prio    string  `json:"prio,omitempty"`
	Notes   *string `json:"notes,omitempty"`
}

// do sends a request and decodes the response into out (which may be nil).
// Writes carry an Idempotency-Key, reused across retries of the same call, so
// a retry after a lost response cannot create a second record.
func (c *Client) do(ctx context.Context, path string, body map[string]any, write bool, out any) ([]string, error) {
	if body == nil {
		body = map[string]any{}
	}
	body["apikey"] = c.apiKey
	body["secretapikey"] = c.secretAPIKey
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	idempotencyKey := ""
	if write {
		idempotencyKey = newIdempotencyKey()
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if !sleep(ctx, backoff(attempt, "")) {
				return nil, ctx.Err()
			}
			continue
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = decodeError(resp.StatusCode, raw)
			if !sleep(ctx, backoff(attempt, resp.Header.Get("Retry-After"))) {
				return nil, ctx.Err()
			}
			continue
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("porkbun: unreadable response (HTTP %d): %w", resp.StatusCode, err)
		}
		if resp.StatusCode >= 400 || !strings.EqualFold(env.Status, "SUCCESS") {
			return nil, decodeError(resp.StatusCode, raw)
		}
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				return nil, fmt.Errorf("porkbun: unexpected response shape: %w", err)
			}
		}
		return env.Warnings, nil
	}
	return nil, fmt.Errorf("porkbun: giving up after %d attempts: %w", maxAttempts, lastErr)
}

func decodeError(status int, raw []byte) error {
	var env envelope
	_ = json.Unmarshal(raw, &env)
	return &APIError{HTTPStatus: status, Code: env.Code, Message: env.Message, RequestID: env.RequestID}
}

func backoff(attempt int, retryAfter string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && s >= 0 {
		return time.Duration(s) * time.Second
	}
	return time.Duration(1<<(attempt-1)) * time.Second
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func newIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func esc(s string) string { return url.PathEscape(s) }

// Ping checks the credentials.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, "/ping", nil, false, nil)
	return err
}

// CreateRecord creates a record and returns its ID and any API warnings.
func (c *Client) CreateRecord(ctx context.Context, domain string, in RecordInput) (string, []string, error) {
	var out struct {
		ID json.Number `json:"id"`
	}
	warnings, err := c.do(ctx, "/dns/create/"+esc(domain), recordBody(in), true, &out)
	if err != nil {
		return "", nil, err
	}
	if out.ID.String() == "" {
		return "", warnings, errors.New("porkbun: create succeeded but returned no record ID")
	}
	return out.ID.String(), warnings, nil
}

// EditRecord replaces a record's fields.
func (c *Client) EditRecord(ctx context.Context, domain, id string, in RecordInput) ([]string, error) {
	return c.do(ctx, "/dns/edit/"+esc(domain)+"/"+esc(id), recordBody(in), true, nil)
}

// DeleteRecord deletes a record.
func (c *Client) DeleteRecord(ctx context.Context, domain, id string) error {
	_, err := c.do(ctx, "/dns/delete/"+esc(domain)+"/"+esc(id), nil, true, nil)
	return err
}

// GetRecord returns one record, or nil if it no longer exists.
func (c *Client) GetRecord(ctx context.Context, domain, id string) (*Record, error) {
	var out struct {
		Records []Record `json:"records"`
	}
	if _, err := c.do(ctx, "/dns/retrieve/"+esc(domain)+"/"+esc(id), nil, false, &out); err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	for i := range out.Records {
		if out.Records[i].ID == id {
			return &out.Records[i], nil
		}
	}
	return nil, nil
}

// ListRecords returns every editable record on a domain. Porkbun's default NS
// and SOA records are not included.
func (c *Client) ListRecords(ctx context.Context, domain string) ([]Record, error) {
	var out struct {
		Records []Record `json:"records"`
	}
	if _, err := c.do(ctx, "/dns/retrieve/"+esc(domain), nil, false, &out); err != nil {
		return nil, err
	}
	return out.Records, nil
}

// GetNameservers returns the domain's delegated nameservers at the registry.
func (c *Client) GetNameservers(ctx context.Context, domain string) ([]string, error) {
	var out struct {
		NS []string `json:"ns"`
	}
	if _, err := c.do(ctx, "/domain/getNs/"+esc(domain), nil, false, &out); err != nil {
		return nil, err
	}
	return out.NS, nil
}

// UpdateNameservers sets the domain's delegated nameservers at the registry.
func (c *Client) UpdateNameservers(ctx context.Context, domain string, ns []string) error {
	_, err := c.do(ctx, "/domain/updateNs/"+esc(domain), map[string]any{"ns": ns}, true, nil)
	return err
}

func recordBody(in RecordInput) map[string]any {
	b := map[string]any{"name": in.Name, "type": in.Type, "content": in.Content}
	if in.TTL != "" {
		b["ttl"] = in.TTL
	}
	if in.Prio != "" {
		b["prio"] = in.Prio
	}
	if in.Notes != nil {
		b["notes"] = *in.Notes
	}
	return b
}
