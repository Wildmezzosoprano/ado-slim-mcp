// Package login implements the `--login` subcommand: an interactive AAD
// device-code flow with cross-platform browser and clipboard helpers.
package login

import (
	"os/exec"
	"runtime"
)

// OpenBrowser attempts to open the given URL in the user's default browser.
// Returns true on best-effort spawn success. Mirrors src/os-utils.ts.
func OpenBrowser(url string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// `cmd /c start "" <url>` — empty title arg avoids `start` parsing issues.
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	// Don't block on the child; treat a successful spawn as success.
	go func() { _ = cmd.Wait() }()
	return true
}
