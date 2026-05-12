package login

import (
	"os/exec"
	"runtime"
	"strings"
)

func pipeText(name string, args []string, text string) bool {
	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	_, _ = stdin.Write([]byte(text))
	_ = stdin.Close()
	return cmd.Wait() == nil
}

// CopyToClipboard places text on the OS clipboard using the platform-native
// CLI helper. Mirrors src/os-utils.ts: clip / pbcopy / wl-copy / xclip / xsel.
func CopyToClipboard(text string) bool {
	switch runtime.GOOS {
	case "windows":
		return pipeText("clip", nil, text)
	case "darwin":
		return pipeText("pbcopy", nil, text)
	default:
		// Linux/BSD fallback chain.
		if pipeText("wl-copy", nil, text) {
			return true
		}
		if pipeText("xclip", []string{"-selection", "clipboard"}, text) {
			return true
		}
		if pipeText("xsel", []string{"--clipboard", "--input"}, text) {
			return true
		}
		return false
	}
}

// errIsExitError reports whether a tool was launched but failed; used only
// for diagnostic logs (not currently surfaced).
func errIsExitError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "exit status")
}
