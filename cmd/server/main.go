// Package main is the entry point for ado-slim-mcp's Go port.
//
// CLI surface:
//
//	ado-slim-mcp                       Start the MCP stdio server (default).
//	ado-slim-mcp --login [--force]     Run AAD device-code login interactively.
//	ado-slim-mcp --version             Print version and exit.
//	ado-slim-mcp --help                Print usage.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"ado-slim/internal/ado"
	"ado-slim/internal/auth"
	"ado-slim/internal/login"
	"ado-slim/internal/tools"

	"github.com/mark3labs/mcp-go/server"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage:
  ado-slim-mcp                       Start the MCP stdio server (default).
  ado-slim-mcp --login [--force]     Run AAD device-code login interactively in a terminal.
                                     Writes a token cache so the server starts silently afterwards.
  ado-slim-mcp --version             Print version and exit.
  ado-slim-mcp --help                Show this help.

Env vars:
  ADO_ORG               (required) Azure DevOps organization name.
  ADO_PAT               If set, server uses PAT auth and skips AAD entirely.
  ADO_AUTH_MODE         "pat" | "aad" - overrides auto-detection.
  ADO_AAD_TENANT        Override AAD tenant (default: organizations).
  ADO_AAD_CLIENT_ID     Override AAD public client id.
  ADO_AUTO_LOGIN        Set to "0" to disable auto-spawning a login window
                        when stdin is not a TTY (Windows only).
`

type cliMode int

const (
	modeServer cliMode = iota
	modeLogin
	modeVersion
	modeHelp
	modeUnknown
)

type cli struct {
	mode    cliMode
	force   bool
	unknown string
}

// parseArgs mirrors `parseArgs()` in src/index.ts: argv-set membership check
// for --help/-h and --login, then a per-arg whitelist when --login is present.
func parseArgs(argv []string) cli {
	set := make(map[string]bool, len(argv))
	for _, a := range argv {
		set[a] = true
	}
	if set["--help"] || set["-h"] {
		return cli{mode: modeHelp}
	}
	if set["--version"] {
		return cli{mode: modeVersion}
	}
	if set["--login"] {
		for _, a := range argv {
			if a != "--login" && a != "--force" {
				return cli{mode: modeUnknown, unknown: a}
			}
		}
		return cli{mode: modeLogin, force: set["--force"]}
	}
	if len(argv) == 0 {
		return cli{mode: modeServer}
	}
	return cli{mode: modeUnknown, unknown: argv[0]}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}

func main() {
	c := parseArgs(os.Args[1:])

	switch c.mode {
	case modeHelp:
		printUsage(os.Stdout)
		os.Exit(0)
	case modeVersion:
		fmt.Fprintln(os.Stdout, version)
		os.Exit(0)
	case modeUnknown:
		fmt.Fprintf(os.Stderr, "[ado-slim] Unknown argument: %s\n\n", c.unknown)
		printUsage(os.Stderr)
		os.Exit(1)
	case modeLogin:
		ctx := context.Background()
		if err := login.Run(ctx, login.Options{Force: c.force}); err != nil {
			fmt.Fprintf(os.Stderr, "[ado-slim] Login failed: %s\n", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	case modeServer:
		runServer()
	}
}

func runServer() {
	ctx := context.Background()

	// 1. Resolve org (required for all flows).
	org, err := auth.GetOrgFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ado-slim] %s\n", err.Error())
		os.Exit(1)
	}

	// 2. Resolve auth (PAT or AAD device-code).
	provider, mode, err := auth.ResolveAuth(ctx, org)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ado-slim] Auth resolution failed: %s\n", err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[ado-slim] Auth mode: %s\n", mode)

	// 3. Construct HTTP client.
	client := ado.NewClient(ado.OrgURL(org), ado.SearchOrgURL(org), provider)

	// 4. Build MCP server, register all 31 tools, serve over stdio.
	// Lazy auth: no startup token acquisition, no connection probe — the
	// first tool call validates auth and triggers a non-blocking interactive
	// login spawn if needed.
	s := server.NewMCPServer("ado-slim", "1.0.0",
		server.WithToolCapabilities(true),
	)
	tools.ConfigureAll(s, client)

	fmt.Fprintln(os.Stderr, "[ado-slim] Server started on stdio")
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "[ado-slim] Fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}
