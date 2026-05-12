// Package auth implements the dual PAT / AAD device-code authentication
// stack used by ado-slim-mcp. Mirrors `src/auth.ts`, `src/aad-auth.ts`, and
// `src/token-cache.ts` from the TypeScript implementation.
package auth

import "context"

// AuthMode is one of "pat" or "aad".
type AuthMode string

const (
	ModePAT AuthMode = "pat"
	ModeAAD AuthMode = "aad"
)

// TokenProvider returns the full HTTP `Authorization` header value (e.g.
// "Basic dXNlcjpwYXQ=" or "Bearer eyJ...") for each outgoing ADO request.
type TokenProvider interface {
	// Authorization returns a header value to attach to a request. Implementations
	// must be safe for concurrent use.
	Authorization(ctx context.Context) (string, error)
	// Mode reports which auth mode is active.
	Mode() AuthMode
}
