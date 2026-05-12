package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// sanitizeOrg returns a filesystem-safe slug for the given ADO org name.
// Lowercases the input, replaces every byte outside [a-z0-9-] with '-',
// collapses runs of '-', trims leading/trailing '-'. Returns "default"
// if the resulting slug is empty.
func sanitizeOrg(org string) string {
	org = strings.ToLower(org)
	var b strings.Builder
	b.Grow(len(org))
	for i := 0; i < len(org); i++ {
		c := org[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteByte(c)
		} else {
			b.WriteByte('-')
		}
	}
	// Collapse runs of '-'
	collapsed := b.String()
	for strings.Contains(collapsed, "--") {
		collapsed = strings.ReplaceAll(collapsed, "--", "-")
	}
	collapsed = strings.Trim(collapsed, "-")
	if collapsed == "" {
		return "default"
	}
	return collapsed
}

// GetCacheDir resolves the per-user config directory used for the MSAL cache,
// the persisted home account ID, and the pending device-code file.
//
// Path resolution mirrors `getCacheDir()` in `src/token-cache.ts` byte-for-byte:
//
//   - Windows: %APPDATA%\ado-slim-mcp (fallback ~/AppData/Roaming/ado-slim-mcp)
//   - other:   $XDG_CONFIG_HOME/ado-slim-mcp or ~/.config/ado-slim-mcp
//
// The directory is created (mode 0700 on POSIX) on first call.
func GetCacheDir() (string, error) {
	var dir string
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home dir: %w", err)
			}
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dir = filepath.Join(appData, "ado-slim-mcp")
	} else {
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home dir: %w", err)
			}
			xdg = filepath.Join(home, ".config")
		}
		dir = filepath.Join(xdg, "ado-slim-mcp")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// GetCachePath returns the full path to the MSAL serialized token cache,
// scoped to the given org.
func GetCachePath(org string) (string, error) {
	d, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "token-cache-"+sanitizeOrg(org)+".json"), nil
}

// GetAccountPath returns the path to the persisted home-account-ID JSON,
// scoped to the given org.
func GetAccountPath(org string) (string, error) {
	d, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "account-"+sanitizeOrg(org)+".json"), nil
}

// GetPendingDeviceCodePath returns the path to the transient device-code
// instructions file written during an in-progress login, scoped to the given org.
func GetPendingDeviceCodePath(org string) (string, error) {
	d, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "pending-device-code-"+sanitizeOrg(org)+".txt"), nil
}

// fileCache is an MSAL Go cache.ExportReplace backed by a single file on
// disk. On non-Windows the on-disk byte format is the unified MSAL JSON
// cache format (plaintext, mode 0600) — interchangeable with caches written
// by the TypeScript implementation. On Windows the file content is binary:
// a 10-byte ASCII magic prefix "ADOSLIMV1\n" followed by raw DPAPI
// ciphertext (CurrentUser scope, app-bound via fixed entropy). The file
// extension stays ".json" for historical reasons even though Windows
// content is binary; legacy plaintext caches written by an earlier version
// are loaded transparently and silently upgraded on the next Export.
//
// (Empirically validated in plan step 22; if the formats diverge, the
// only impact is users re-login once.)
type fileCache struct {
	path string
}

// NewFileCache constructs a cache.ExportReplace bound to the per-org
// token-cache-<org>.json path resolved by GetCachePath.
func NewFileCache(org string) (cache.ExportReplace, error) {
	p, err := GetCachePath(org)
	if err != nil {
		return nil, err
	}
	return &fileCache{path: p}, nil
}

// Replace loads the on-disk cache into MSAL's in-memory representation.
// Missing files are treated as an empty cache (not an error).
func (c *fileCache) Replace(_ context.Context, u cache.Unmarshaler, _ cache.ReplaceHints) error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		// Best-effort: log via stderr but do not abort — empty cache is recoverable.
		fmt.Fprintf(os.Stderr, "[ado-slim] token cache read failed: %v\n", err)
		return nil
	}
	plain, wasEncrypted, err := decryptFromStorage(data)
	if err != nil {
		if wasEncrypted {
			return fmt.Errorf("token cache decryption failed for %q (likely written by a different Windows user or machine); re-run `ado-slim-mcp.exe --login --force` to recover: %w", c.path, err)
		}
		// Defensive: should not occur with current storage impls.
		return err
	}
	return u.Unmarshal(plain)
}

// Export writes the in-memory MSAL cache to disk atomically (write to .tmp
// then rename) and applies 0600 perms on POSIX.
func (c *fileCache) Export(_ context.Context, m cache.Marshaler, _ cache.ExportHints) error {
	data, err := m.Marshal()
	if err != nil {
		return fmt.Errorf("msal marshal: %w", err)
	}
	data, err = encryptForStorage(data)
	if err != nil {
		return fmt.Errorf("encrypt token cache: %w", err)
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, c.path); err != nil {
		// best-effort cleanup of the partial file
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, c.path, err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(c.path, 0o600)
	}
	return nil
}

// LoadHomeAccountID reads the persisted home-account ID written by a prior
// login for the given org, or returns ("", false) if none exists or the
// file is malformed.
func LoadHomeAccountID(org string) (string, bool) {
	p, err := GetAccountPath(org)
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	var parsed struct {
		HomeAccountID string `json:"homeAccountId"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", false
	}
	if parsed.HomeAccountID == "" {
		return "", false
	}
	return parsed.HomeAccountID, true
}

// SaveHomeAccountID persists the resolved home-account ID for silent re-auth
// on the next process start, scoped to the given org.
func SaveHomeAccountID(org, id string) error {
	p, err := GetAccountPath(org)
	if err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		HomeAccountID string `json:"homeAccountId"`
	}{HomeAccountID: id})
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(p, 0o600)
	}
	return nil
}

// ClearPendingDeviceCode removes the transient device-code file for the
// given org. Best-effort.
func ClearPendingDeviceCode(org string) {
	p, err := GetPendingDeviceCodePath(org)
	if err != nil {
		return
	}
	_ = os.Remove(p)
}
