// Package client is the HTTP transport for the Anthropic Managed Agents
// REST API. It hand-rolls retries, typed errors, and request-ID capture on
// top of net/http via retryablehttp, and exposes one Go method per API
// operation (CreateAgent, GetAgent, …). It deliberately depends on nothing
// from terraform-plugin-framework so it could be lifted into a standalone
// SDK if needed.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	defaultBaseURL    = "https://api.anthropic.com"
	apiVersionHeader  = "2023-06-01"
	managedAgentsBeta = "managed-agents-2026-04-01"
	skillsBeta        = "skills-2025-10-02"
	logSubsystem      = "client"
)

// headerOverride lets callers replace or augment the default request headers
// for a single call. Used to swap the `anthropic-beta` value on the skill
// endpoints (which speak `skills-2025-10-02` rather than the managed-agents
// beta). The override is applied after the defaults via `Set`, so it always
// wins.
type headerOverride struct {
	name  string
	value string
}

func withHeader(name, value string) headerOverride {
	return headerOverride{name: name, value: value}
}

// Client is the HTTP transport for the Managed Agents API.
type Client struct {
	httpClient *retryablehttp.Client
	apiKey     string
	baseURL    string
	userAgent  string
}

// Config configures a Client.
type Config struct {
	APIKey     string
	BaseURL    string // optional; defaults to https://api.anthropic.com
	UserAgent  string // optional
	MaxRetries int    // optional; defaults to 3
}

// New returns a configured Client. APIKey must be non-empty.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("client: APIKey is required")
	}

	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	if _, err := url.Parse(base); err != nil {
		return nil, fmt.Errorf("client: invalid BaseURL %q: %w", base, err)
	}
	base = strings.TrimRight(base, "/")

	retries := cfg.MaxRetries
	if retries == 0 {
		retries = 3
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = retries
	rc.Logger = nil // tflog handles our logging
	rc.CheckRetry = retryablehttp.DefaultRetryPolicy

	return &Client{
		httpClient: rc,
		apiKey:     cfg.APIKey,
		baseURL:    base,
		userAgent:  cfg.UserAgent,
	}, nil
}

// BaseURL returns the configured base URL (no trailing slash). Useful for tests.
func (c *Client) BaseURL() string { return c.baseURL }

// do executes an HTTP request, retrying transient failures. body may be nil
// for requests without a payload. out may be nil for endpoints that return no
// content of interest. extra header overrides are applied after the default
// header set via `Set`, so they win on conflict.
func (c *Client) do(ctx context.Context, method, path string, body, out any, extra ...headerOverride) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("client: marshal request body: %w", err)
		}
	}
	return c.doRaw(ctx, method, path, bodyBytes, "application/json", redactJSON(bodyBytes), out, extra...)
}

// doMultipart executes a multipart/form-data POST. The body is already
// encoded by the caller (see buildSkillMultipart). The body is intentionally
// NOT routed through redactJSON — multipart payloads contain raw file bytes
// that should never be logged in full. A structured summary is emitted
// instead (size + content type).
func (c *Client) doMultipart(ctx context.Context, method, path string, body []byte, contentType string, summary map[string]any, out any, extra ...headerOverride) error {
	// Emit a separate structured summary so operators can correlate the
	// multipart upload with the response log without seeing the file bytes.
	logFields := map[string]any{
		"method":                 method,
		"endpoint":               c.baseURL + path,
		"multipart_size_bytes":   len(body),
		"multipart_content_type": contentType,
	}
	for k, v := range summary {
		logFields[k] = v
	}
	tflog.SubsystemDebug(ctx, logSubsystem, "multipart request", logFields)
	return c.doRaw(ctx, method, path, body, contentType, "<multipart redacted>", out, extra...)
}

// doRaw is the shared low-level path. It is not called directly by API
// methods; use `do` (JSON) or `doMultipart` (multipart) instead.
func (c *Client) doRaw(ctx context.Context, method, path string, bodyBytes []byte, contentType, redactedBody string, out any, extra ...headerOverride) error {
	endpoint := c.baseURL + path

	// For JSON requests we log the redacted body inline; multipart requests
	// already emitted a separate summary record, so we skip the body field
	// here to avoid noise.
	if contentType == "application/json" {
		tflog.SubsystemDebug(ctx, logSubsystem, "request", map[string]any{
			"method":   method,
			"endpoint": endpoint,
			"body":     redactedBody,
		})
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersionHeader)
	req.Header.Set("anthropic-beta", managedAgentsBeta)
	req.Header.Set("Content-Type", contentType)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	for _, h := range extra {
		req.Header.Set(h.name, h.value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("client: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("client: read response: %w", err)
	}

	reqID := resp.Header.Get("request-id")

	tflog.SubsystemDebug(ctx, logSubsystem, "response", map[string]any{
		"status":     resp.StatusCode,
		"request_id": reqID,
		"body":       redactJSON(respBody),
	})

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, reqID, respBody)
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("client: decode response: %w", err)
	}
	return nil
}

// redactJSON masks known-sensitive keys in a JSON blob so that tflog output is
// safe to share. The output is best-effort; on parse failure the original
// bytes are returned as a string with the api key masked.
func redactJSON(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return string(b)
	}
	redactMap(raw)
	out, err := json.Marshal(raw)
	if err != nil {
		return string(b)
	}
	return string(out)
}

// nonSecretKeySuffixes lists field names that contain a secret-like substring
// ("token", "secret") but are themselves not secrets — typically OAuth
// metadata or write-only-version counters. Keeping them in clear in debug
// logs makes triage useful.
var nonSecretKeySuffixes = []string{
	"_endpoint",      // e.g. token_endpoint
	"_endpoint_auth", // e.g. token_endpoint_auth
	"_wo_version",    // e.g. token_wo_version (TF write-only rotation counter)
}

func isSecretKey(k string) bool {
	lower := strings.ToLower(k)
	for _, suf := range nonSecretKeySuffixes {
		if strings.HasSuffix(lower, suf) {
			return false
		}
	}
	if strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "passphrase") ||
		lower == "api_key" ||
		lower == "x-api-key" {
		return true
	}
	return false
}

func redactMap(m map[string]any) {
	for k, v := range m {
		if isSecretKey(k) {
			m[k] = "***"
			continue
		}
		switch vv := v.(type) {
		case map[string]any:
			redactMap(vv)
		case []any:
			for _, item := range vv {
				if mm, ok := item.(map[string]any); ok {
					redactMap(mm)
				}
			}
		}
	}
}
