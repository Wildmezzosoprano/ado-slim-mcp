package auth

import (
	"context"
	"encoding/base64"
)

// patProvider is a static-Bearer^WBasic provider. ADO PAT auth is HTTP Basic
// with empty username and the PAT as the password (matches `getPersonalAccessTokenHandler`).
type patProvider struct {
	header string
}

// NewPATProvider constructs a TokenProvider that emits a fixed
// "Basic base64(:pat)" header on every request.
func NewPATProvider(pat string) TokenProvider {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + pat))
	return &patProvider{header: "Basic " + encoded}
}

func (p *patProvider) Authorization(_ context.Context) (string, error) {
	return p.header, nil
}

func (p *patProvider) Mode() AuthMode { return ModePAT }
