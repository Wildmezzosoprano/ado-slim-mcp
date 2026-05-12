package auth

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// fakeMarshaler returns a fixed byte slice from Marshal().
type fakeMarshaler struct{ data []byte }

func (f *fakeMarshaler) Marshal() ([]byte, error) { return f.data, nil }

// fakeUnmarshaler captures the bytes passed to Unmarshal().
type fakeUnmarshaler struct {
	got    []byte
	called bool
}

func (f *fakeUnmarshaler) Unmarshal(b []byte) error {
	f.got = append([]byte(nil), b...)
	f.called = true
	return nil
}

// Compile-time interface conformance.
var (
	_ cache.Marshaler   = (*fakeMarshaler)(nil)
	_ cache.Unmarshaler = (*fakeUnmarshaler)(nil)
)

func TestSanitizeOrg(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "default"},
		{"MyOrg", "myorg"},
		{"my org", "my-org"},
		{"---", "default"},
		{"a/b\\c", "a-b-c"},
		{"foo--bar", "foo-bar"},
		{"-leading-trailing-", "leading-trailing"},
		{"AAA BBB", "aaa-bbb"},
		{"!!!", "default"},
		{"123-abc", "123-abc"},
	}
	for _, tc := range cases {
		got := sanitizeOrg(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeOrg(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// minimalMSALJSON is a valid-shape empty MSAL unified-cache skeleton.
var minimalMSALJSON = []byte(`{"AccessToken":{},"Account":{},"IdToken":{},"RefreshToken":{},"AppMetadata":{}}`)

func TestReplaceLegacyPlaintext(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "token-cache-test.json")
	if err := os.WriteFile(p, minimalMSALJSON, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c := &fileCache{path: p}
	u := &fakeUnmarshaler{}
	if err := c.Replace(context.Background(), u, cache.ReplaceHints{}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if !u.called {
		t.Fatalf("Unmarshaler not called")
	}
	if !bytes.Equal(u.got, minimalMSALJSON) {
		t.Fatalf("plaintext mismatch: got %q want %q", u.got, minimalMSALJSON)
	}
}

func TestExportReplaceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "token-cache-test.json")
	c := &fileCache{path: p}
	m := &fakeMarshaler{data: minimalMSALJSON}
	if err := c.Export(context.Background(), m, cache.ExportHints{}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	u := &fakeUnmarshaler{}
	if err := c.Replace(context.Background(), u, cache.ReplaceHints{}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if !u.called {
		t.Fatalf("Unmarshaler not called")
	}
	if !bytes.Equal(u.got, minimalMSALJSON) {
		t.Fatalf("round-trip mismatch: got %q want %q", u.got, minimalMSALJSON)
	}
}

func TestReplaceMissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "does-not-exist.json")
	c := &fileCache{path: p}
	u := &fakeUnmarshaler{}
	if err := c.Replace(context.Background(), u, cache.ReplaceHints{}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if u.called {
		t.Fatalf("Unmarshaler should not be called for missing file")
	}
}
