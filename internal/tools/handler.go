// Package tools registers the 31 read-only Azure DevOps MCP tools.
//
// Each handler follows the pattern: extract args, build a REST URL via
// internal/ado, call GetJSON / PostJSON, transform via internal/slim, return
// JSON-encoded text. Errors from the HTTP layer are caught by withError and
// mapped to auth-mode-aware MCP error results (mirrors src/errors.ts).
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"ado-slim/internal/ado"
	"ado-slim/internal/auth"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// withError wraps a tool handler and converts any *ado.APIError or generic
// error into a CallToolResult with isError=true, mimicking handleApiError.
func withError(fn server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := fn(ctx, req)
		if err == nil {
			return res, nil
		}
		return mcp.NewToolResultError(mapErrorMessage(err)), nil
	}
}

func mapErrorMessage(err error) string {
	if errors.Is(err, auth.ErrLoginRequired) {
		return "Not authenticated yet. A login window has been opened in a separate console — please complete the device-code sign-in there, then ask me the same thing again. (If no window appeared, run `ado-slim-mcp --login` in a terminal.)"
	}
	var apiErr *ado.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == 401:
			if auth.GetActiveAuthMode() == auth.ModeAAD {
				return "Authentication failed - AAD token rejected; refresh tokens may have expired. " +
					"Delete the token cache and restart to re-authenticate."
			}
			return "Authentication failed - check ADO_PAT is valid and not expired"
		case apiErr.Status == 403:
			return fmt.Sprintf("Permission denied: %s", apiErr.Body)
		case apiErr.Status == 404:
			return fmt.Sprintf("Resource not found: %s", apiErr.Message)
		case apiErr.Status == 429:
			retry := "unknown"
			if v := apiErr.Headers.Get("Retry-After"); v != "" {
				retry = v
			}
			return fmt.Sprintf("Rate limited by Azure DevOps. Retry after: %ss", retry)
		case apiErr.Status >= 500:
			return fmt.Sprintf("Azure DevOps API error: %d %s", apiErr.Status, apiErr.Message)
		}
	}
	return fmt.Sprintf("Error: %s", err.Error())
}

// jsonResult marshals v and wraps it as a tool text result. Mirrors the TS
// pattern of `JSON.stringify(result)`.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// optInt extracts an optional integer argument, returning nil when absent.
func optInt(req mcp.CallToolRequest, key string) *int {
	args := req.GetArguments()
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch n := v.(type) {
	case float64:
		x := int(n)
		return &x
	case int:
		x := n
		return &x
	case int64:
		x := int(n)
		return &x
	case string:
		// best-effort
		if n == "" {
			return nil
		}
		x := 0
		neg := false
		i := 0
		if n[0] == '-' || n[0] == '+' {
			neg = n[0] == '-'
			i = 1
		}
		for ; i < len(n); i++ {
			c := n[i]
			if c < '0' || c > '9' {
				return nil
			}
			x = x*10 + int(c-'0')
		}
		if neg {
			x = -x
		}
		return &x
	}
	return nil
}

func optString(req mcp.CallToolRequest, key string) string {
	return req.GetString(key, "")
}

// reqInt extracts a required-or-defaulted int that may arrive as number or string.
func reqInt(req mcp.CallToolRequest, key string) (int, error) {
	if p := optInt(req, key); p != nil {
		return *p, nil
	}
	return 0, fmt.Errorf("missing required argument %q", key)
}

// reqString returns a required string or an error.
func reqString(req mcp.CallToolRequest, key string) (string, error) {
	return req.RequireString(key)
}
