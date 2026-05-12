package auth

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// activeAuthMode tracks the resolved mode so error messages elsewhere can be
// auth-mode aware (mirrors `getActiveAuthMode()` in TS).
var activeAuthMode atomic.Value // holds AuthMode (or empty string when unset)

// SetActiveAuthMode records the resolved mode for diagnostic use.
func SetActiveAuthMode(m AuthMode) {
	activeAuthMode.Store(m)
}

// GetActiveAuthMode returns the resolved mode, or "" if not yet set.
func GetActiveAuthMode() AuthMode {
	v := activeAuthMode.Load()
	if v == nil {
		return ""
	}
	if s, ok := v.(AuthMode); ok {
		return s
	}
	return ""
}

// GetPATFromEnv returns the trimmed ADO_PAT, or "" if unset/empty.
func GetPATFromEnv() string {
	pat := strings.TrimSpace(os.Getenv("ADO_PAT"))
	return pat
}

// GetOrgFromEnv returns ADO_ORG or an error mirroring the TS message.
func GetOrgFromEnv() (string, error) {
	org := strings.TrimSpace(os.Getenv("ADO_ORG"))
	if org == "" {
		return "", fmt.Errorf("ADO_ORG environment variable is required (e.g., 'CloudValueDelivery')")
	}
	return org, nil
}

// ResolveAuthMode mirrors `resolveAuthMode()` in TS:
//
//   - if ADO_AUTH_MODE is set, it must be "pat" or "aad" (else error)
//   - else, presence of ADO_PAT selects "pat", absence selects "aad"
func ResolveAuthMode() (AuthMode, error) {
	explicit := strings.ToLower(strings.TrimSpace(os.Getenv("ADO_AUTH_MODE")))
	if explicit != "" {
		switch explicit {
		case "pat":
			return ModePAT, nil
		case "aad":
			return ModeAAD, nil
		default:
			return "", fmt.Errorf("Invalid ADO_AUTH_MODE='%s'. Expected 'pat' or 'aad'.", os.Getenv("ADO_AUTH_MODE"))
		}
	}
	if GetPATFromEnv() != "" {
		return ModePAT, nil
	}
	return ModeAAD, nil
}
