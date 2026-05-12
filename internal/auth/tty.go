package auth

import (
	"os"

	"golang.org/x/term"
)

// IsStdinTerminal returns true when standard input is attached to an
// interactive terminal (TTY). When the server is launched via the MCP
// stdio transport, stdin is a pipe and this returns false — which is
// the signal AcquireInitialToken uses to decide whether to spawn a
// separate console window for the device-code flow.
func IsStdinTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
