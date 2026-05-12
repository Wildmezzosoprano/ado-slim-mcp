package login

import (
	"context"
	"fmt"
	"os"
	"sync"

	"ado-slim/internal/auth"
)

// Options controls the --login subcommand.
type Options struct {
	Force bool
}

// Run performs the AAD device-code login flow interactively, opening a browser
// and copying the user code to the clipboard. Mirrors src/login-command.ts.
//
// All output goes to stdout (matching the TS impl which uses console.log here),
// since this command is not invoked while the stdio MCP transport is active.
func Run(ctx context.Context, opts Options) error {
	org, err := auth.GetOrgFromEnv()
	if err != nil {
		return fmt.Errorf(
			"ADO_ORG environment variable is required (e.g., 'CloudValueDelivery'). " +
				"Cache files are scoped per org so --login needs to know which org to write the cache for.")
	}

	fmt.Println("[ado-slim] Starting AAD device-code login...")
	if opts.Force {
		fmt.Println("[ado-slim] --force: ignoring cached account.")
	}

	app, err := auth.NewMSALApp(org)
	if err != nil {
		return fmt.Errorf("init msal app: %w", err)
	}

	cb := func(message, userCode, verificationURL string) {
		fmt.Println()
		fmt.Println(message)
		fmt.Println()

		var wg sync.WaitGroup
		var opened, copied bool
		wg.Add(2)
		go func() { defer wg.Done(); opened = OpenBrowser(verificationURL) }()
		go func() { defer wg.Done(); copied = CopyToClipboard(userCode) }()
		wg.Wait()

		if opened {
			fmt.Printf("[ado-slim] Browser opened to %s\n", verificationURL)
		} else {
			fmt.Printf("[ado-slim] Could not open browser; visit %s manually\n", verificationURL)
		}
		if copied {
			fmt.Printf("[ado-slim] Code %s copied to clipboard\n", userCode)
		} else {
			fmt.Printf("[ado-slim] Code: %s (clipboard unavailable - type it manually)\n", userCode)
		}
		fmt.Println()
		fmt.Println("[ado-slim] Waiting for sign-in to complete...")
	}

	res, err := auth.AcquireInitialToken(ctx, app, auth.InitialTokenOptions{
		Force:              opts.Force,
		DeviceCodeCallback: cb,
	}, org)
	if err != nil {
		return err
	}

	who := res.Account.PreferredUsername
	if who == "" {
		who = "<unknown account>"
	}
	cachePath, _ := auth.GetCachePath(org)
	fmt.Println()
	fmt.Printf("[ado-slim] Logged in as %s for org %s\n", who, org)
	fmt.Printf("[ado-slim] Token cache written to %s\n", cachePath)
	fmt.Println("[ado-slim] You can now launch the MCP server normally - auth will be silent.")
	_ = os.Stdout.Sync()
	return nil
}
