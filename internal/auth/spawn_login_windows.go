//go:build windows

package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ErrAutoLoginUnsupported is returned by SpawnInteractiveLogin on platforms
// where launching a separate console window for the login flow is not
// implemented.
var ErrAutoLoginUnsupported = errors.New("auto-login not supported on this platform")

// ErrAutoLoginTimedOut is returned when the spawned login child does not
// produce an updated account file within the configured timeout.
var ErrAutoLoginTimedOut = errors.New("auto-login timed out waiting for token cache")

// SpawnInteractiveLogin launches a new instance of this executable with
// `--login`, attached to a brand-new console window so the device-code
// instructions are visible to the user. It then watches the per-org
// account file for an mtime change, indicating the child has written a
// fresh token cache, and returns nil on success.
//
// The function blocks until one of:
//   - the account file appears or its mtime advances → success (nil)
//   - the child exits without updating the account file → error
//   - the configured timeout elapses → ErrAutoLoginTimedOut
//   - ctx is cancelled → ctx.Err()
func SpawnInteractiveLogin(ctx context.Context, org string, timeout time.Duration) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve self exe: %w", err)
	}
	acctPath, err := GetAccountPath(org)
	if err != nil {
		return fmt.Errorf("resolve account path: %w", err)
	}

	// Capture baseline (existence + mtime).
	var baselineExists bool
	var baselineMtime time.Time
	if st, err := os.Stat(acctPath); err == nil {
		baselineExists = true
		baselineMtime = st.ModTime()
	}

	// Spawn via `cmd.exe /c start /wait` rather than CreateProcess with
	// CREATE_NEW_CONSOLE directly. Reason: Go's os/exec always assigns
	// stdin/stdout/stderr handles (falling back to NUL when the fields are
	// nil), which means a CreateProcess child sees a new console window
	// but its prints still go to NUL — the window stays blank. The Windows
	// `start` command launches the target with proper stdio attachment to
	// the freshly created console, so device-code output is visible.
	//
	// /wait makes start block until the launched program exits, so
	// cmd.Wait() in this process tracks the child's lifetime.
	cmd := exec.CommandContext(ctx, "cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`cmd.exe /c start "ado-slim login" /wait "%s" --login`, selfPath),
	}
	// ADO_SLIM_NO_AUTO_SPAWN=1 prevents the child from itself trying to
	// auto-spawn another login window — without this guard the child sees
	// non-TTY stdin and would fork a grandchild, ad infinitum.
	cmd.Env = append(os.Environ(), "ADO_SLIM_NO_AUTO_SPAWN=1")

	fmt.Fprintf(os.Stderr,
		"[ado-slim] No cached login for org '%s'; spawning interactive login window. (Set ADO_AUTO_LOGIN=0 to disable.)\n",
		org)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn --login: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	deadline := time.After(timeout)

	cacheUpdated := func() bool {
		st, err := os.Stat(acctPath)
		if err != nil {
			return false
		}
		if !baselineExists {
			return true
		}
		return st.ModTime().After(baselineMtime)
	}

	drain := func() {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}

	for {
		select {
		case <-ticker.C:
			if cacheUpdated() {
				return nil
			}
		case werr := <-done:
			// Child exited. Give the filesystem a 1s grace window in case
			// the rename hasn't been observed yet, then re-check.
			time.Sleep(1 * time.Second)
			if cacheUpdated() {
				return nil
			}
			if werr != nil {
				return fmt.Errorf("auto-login child exited without writing cache: %w", werr)
			}
			return errors.New("auto-login child exited without writing cache")
		case <-deadline:
			_ = cmd.Process.Kill()
			drain()
			return ErrAutoLoginTimedOut
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			drain()
			return ctx.Err()
		}
	}
}
