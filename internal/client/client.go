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
	logSubsystem      = "client"
)

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
// content of interest.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("client: marshal request body: %w", err)
		}
	}

	endpoint := c.baseURL + path

	tflog.SubsystemDebug(ctx, logSubsystem, "request", map[string]any{
		"method":   method,
		"endpoint": endpoint,
		"body":     redactJSON(bodyBytes),
	})

	req, err := retryablehttp.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersionHeader)
	req.Header.Set("anthropic-beta", managedAgentsBeta)
	req.Header.Set("Content-Type", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
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

func redactMap(m map[string]any) {
	for k, v := range m {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "token") ||
			strings.Contains(lower, "secret") ||
			lower == "api_key" ||
			lower == "x-api-key" {
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
