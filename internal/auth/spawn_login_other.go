//go:build !windows

package auth

import (
	"context"
	"errors"
	"time"
)

// ErrAutoLoginUnsupported is returned by SpawnInteractiveLogin on platforms
// where launching a separate console window for the login flow is not
// implemented.
var ErrAutoLoginUnsupported = errors.New("auto-login not supported on this platform")

// ErrAutoLoginTimedOut is returned when the spawned login child does not
// produce an updated account file within the configured timeout. Defined
// for symmetry with the Windows build.
var ErrAutoLoginTimedOut = errors.New("auto-login timed out waiting for token cache")

// SpawnInteractiveLogin is a no-op on non-Windows platforms. Real terminal
// users see the device-code instructions inline on stderr, so the auto-spawn
// path is unnecessary.
func SpawnInteractiveLogin(ctx context.Context, org string, timeout time.Duration) error {
	_ = ctx
	_ = org
	_ = timeout
	return ErrAutoLoginUnsupported
}
