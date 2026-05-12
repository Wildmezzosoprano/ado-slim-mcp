package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ado-slim/internal/auth"
)

// ErrWriteAttempted is returned by the read-only guard when a non-allowlisted
// method/host combination is invoked. It safeguards the read-only invariant
// at the HTTP layer.
var ErrWriteAttempted = errors.New("ado-slim is read-only: write methods are not permitted")

// APIError is the typed error returned for any non-2xx response. Tools wrap
// this to produce auth-mode-aware MCP error results.
type APIError struct {
	Status  int
	Message string
	Body    string
	Headers http.Header
	URL     string
	Method  string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("ado api %s %s: %d %s", e.Method, e.URL, e.Status, e.Message)
	}
	return fmt.Sprintf("ado api %s %s: %d", e.Method, e.URL, e.Status)
}

// Client is the read-only Azure DevOps HTTP client.
type Client struct {
	httpClient    *http.Client
	tokenProvider auth.TokenProvider
	orgURL        string
	searchOrgURL  string
}

// NewClient constructs a Client bound to the given org base URL and provider.
// Default request timeout is 30s.
func NewClient(orgURL, searchOrgURL string, provider auth.TokenProvider) *Client {
	return &Client{
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		tokenProvider: provider,
		orgURL:        orgURL,
		searchOrgURL:  searchOrgURL,
	}
}

// OrgURL returns the configured base ADO org URL (no trailing slash).
func (c *Client) OrgURL() string { return c.orgURL }

// SearchOrgURL returns the configured Search-API base URL (no trailing slash).
func (c *Client) SearchOrgURL() string { return c.searchOrgURL }

// readOnlyAllowed returns nil when the (method, parsedURL) combination is
// permitted by the read-only invariant. The allowlist is tight by design;
// every entry is justified.
//
//	GET                                       — universally allowed (queries)
//	POST  almsearch.dev.azure.com/...         — Search API uses POST for queries
//	POST  dev.azure.com/.../_apis/wit/wiql    — WIQL execution (read-only by spec)
//	POST  dev.azure.com/.../_apis/wit/queries/...
//	                                          — saved-query execution by ID
//	      (also read-only by spec; ADO REST exposes these as POST)
//
// All other methods (PUT, PATCH, DELETE, OPTIONS, ...) and all non-allowed
// POST targets are rejected with ErrWriteAttempted.
func readOnlyAllowed(method string, u *url.URL) error {
	if method == http.MethodGet {
		return nil
	}
	if method != http.MethodPost {
		return fmt.Errorf("%w: method %s not allowed", ErrWriteAttempted, method)
	}
	host := strings.ToLower(u.Hostname())
	path := u.EscapedPath()
	switch host {
	case "almsearch.dev.azure.com":
		return nil
	case "dev.azure.com":
		// WIQL ad-hoc:   /<org>/<project>/_apis/wit/wiql            (and ?api-version=...)
		// WIQL by id:    /<org>/<project>/_apis/wit/wiql/<queryId>
		// Saved queries: /<org>/<project>/_apis/wit/queries/<...>
		if strings.Contains(path, "/_apis/wit/wiql") ||
			strings.Contains(path, "/_apis/wit/queries/") {
			return nil
		}
	}
	return fmt.Errorf("%w: POST to %s not in read-only allowlist", ErrWriteAttempted, u.String())
}

// Do executes a request with the read-only guard, auth header injection,
// and unified error mapping.
//
//   - method: GET or POST (POST only for the allowlisted endpoints).
//   - urlStr: absolute URL.
//   - body:   if non-nil, JSON-encoded into the request body.
//   - out:    if non-nil, the JSON response is decoded into it.
func (c *Client) Do(ctx context.Context, method, urlStr string, body any, out any) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if err := readOnlyAllowed(method, parsed); err != nil {
		return err
	}

	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	authHeader, err := c.tokenProvider.Authorization(ctx)
	if err != nil {
		return fmt.Errorf("acquire auth header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http %s %s: %w", method, urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Read at most 1KB of the response body to keep error blobs bounded.
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &APIError{
			Status:  resp.StatusCode,
			Message: resp.Status,
			Body:    string(excerpt),
			Headers: resp.Header,
			URL:     urlStr,
			Method:  method,
		}
	}

	if out == nil {
		// Drain so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// GetJSON is a convenience wrapper for the common case.
func (c *Client) GetJSON(ctx context.Context, url string, out any) error {
	return c.Do(ctx, http.MethodGet, url, nil, out)
}

// PostJSON is a convenience wrapper for allowlisted POSTs.
func (c *Client) PostJSON(ctx context.Context, url string, body, out any) error {
	return c.Do(ctx, http.MethodPost, url, body, out)
}

// GetRaw fetches a URL and returns the raw response bytes. Used by the
// `get_file_content` tool which must inspect octet-streams. Auth and the
// read-only guard apply identically.
func (c *Client) GetRaw(ctx context.Context, urlStr string, accept string) ([]byte, http.Header, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parse url: %w", err)
	}
	if err := readOnlyAllowed(http.MethodGet, parsed); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	if accept == "" {
		accept = "application/octet-stream"
	}
	req.Header.Set("Accept", accept)
	authHeader, err := c.tokenProvider.Authorization(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("acquire auth header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http GET %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, resp.Header, &APIError{
			Status:  resp.StatusCode,
			Message: resp.Status,
			Body:    string(excerpt),
			Headers: resp.Header,
			URL:     urlStr,
			Method:  http.MethodGet,
		}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, fmt.Errorf("read response body: %w", err)
	}
	return data, resp.Header, nil
}
