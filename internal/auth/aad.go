package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

// AADScopes is the Azure DevOps resource scope used for all token requests.
// The GUID 499b84ac-... is the well-known ADO public application ID.
var AADScopes = []string{"499b84ac-1321-427f-aa17-267ca6975798/.default"}

// ErrLoginRequired is returned by aadProvider.Authorization when no usable
// account is available and a non-blocking interactive login window has been
// (or will be) spawned. Tool handlers detect this via errors.Is and surface
// a friendly "login window opened, retry" message to the user.
var ErrLoginRequired = errors.New("ado-slim: AAD login required; complete the login window then retry")

// AadConfig captures the per-tenant AAD configuration, sourced from env.
type AadConfig struct {
	Tenant   string
	ClientID string
}

// GetAADConfig mirrors `getAadConfig()` in TS:
//
//   - tenant defaults to "organizations"
//   - clientId defaults to the Azure CLI public client (04b07795-...)
func GetAADConfig() AadConfig {
	tenant := strings.TrimSpace(os.Getenv("ADO_AAD_TENANT"))
	if tenant == "" {
		tenant = "organizations"
	}
	clientID := strings.TrimSpace(os.Getenv("ADO_AAD_CLIENT_ID"))
	if clientID == "" {
		clientID = "04b07795-8ddb-461a-bbee-02f9e1bf7b46"
	}
	return AadConfig{Tenant: tenant, ClientID: clientID}
}

// NewMSALApp constructs the MSAL public client wired to the persistent
// on-disk file cache, scoped to the given org.
func NewMSALApp(org string) (public.Client, error) {
	cfg := GetAADConfig()
	authority := "https://login.microsoftonline.com/" + cfg.Tenant
	cacheAccessor, err := NewFileCache(org)
	if err != nil {
		return public.Client{}, fmt.Errorf("init token cache: %w", err)
	}
	app, err := public.New(cfg.ClientID,
		public.WithAuthority(authority),
		public.WithCache(cacheAccessor),
	)
	if err != nil {
		return public.Client{}, fmt.Errorf("init msal: %w", err)
	}
	return app, nil
}

// InitialTokenOptions controls the behavior of AcquireInitialToken.
type InitialTokenOptions struct {
	// Force, when true, skips the silent-from-cache path and always runs
	// a fresh device-code flow.
	Force bool
	// DeviceCodeCallback is invoked once the device-code flow has produced
	// the user-facing instructions (URL + code). If nil, the message is
	// written to stderr with the standard "[ado-slim] " prefix.
	DeviceCodeCallback func(message, userCode, verificationURL string)
}

// AcquireInitialToken performs the first-run authentication: try silent
// from cache (if a home-account is persisted and Force is false), and
// otherwise run device-code interactively (or auto-spawn a login window
// when stdin is not a TTY on Windows).
//
// Mirrors `acquireInitialToken()` in src/aad-auth.ts plus the auto-spawn
// behavior added for the MCP launcher case.
func AcquireInitialToken(ctx context.Context, app public.Client, opts InitialTokenOptions, org string) (public.AuthResult, error) {
	// 1. Try silent re-auth from persisted home account (unless forced).
	if !opts.Force {
		if id, ok := LoadHomeAccountID(org); ok {
			// MSAL Go's public.Client does not expose a lookup-by-id, so we
			// iterate cached accounts (typically 0-1 entries) and match the
			// stored home account id.
			if accounts, err := app.Accounts(ctx); err == nil {
				for _, account := range accounts {
					if account.HomeAccountID != id {
						continue
					}
					result, err := app.AcquireTokenSilent(ctx, AADScopes, public.WithSilentAccount(account))
					if err == nil {
						return result, nil
					}
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return public.AuthResult{}, err
					}
					// Match TS: log and fall through to device-code.
					fmt.Fprintf(os.Stderr,
						"[ado-slim] silent token acquisition failed, falling back to device-code: %v\n",
						err)
					break
				}
			}
		}
	}

	// 2. Auto-spawn an interactive login window when stdin is not a TTY
	//    (typical for the MCP stdio launcher) and ADO_AUTO_LOGIN is not "0".
	//    This sidesteps the silent-hang where device-code instructions are
	//    written to stderr that the launcher swallows.
	if !opts.Force {
		autoOff := os.Getenv("ADO_AUTO_LOGIN") == "0"
		// Recursion guard: SpawnInteractiveLogin sets this env var on the
		// child so the child never tries to auto-spawn its own grandchild.
		// Required because Go redirects nil Stdin to NUL, so the child's
		// IsStdinTerminal() returns false despite the new console.
		spawnedChild := os.Getenv("ADO_SLIM_NO_AUTO_SPAWN") == "1"
		switch {
		case spawnedChild:
			// We're the spawned --login child. Fall through to inline device-code.
		case IsStdinTerminal():
			// Real terminal — fall through to inline device-code below.
		case autoOff:
			fmt.Fprintln(os.Stderr,
				"[ado-slim] Stdin is not a TTY but ADO_AUTO_LOGIN=0; printing device code to stderr (likely invisible under MCP launcher)")
		default:
			err := SpawnInteractiveLogin(ctx, org, 5*time.Minute)
			switch {
			case err == nil:
				// Re-run silent acquisition once. The child wrote the cache.
				if id, ok := LoadHomeAccountID(org); ok {
					if accounts, aerr := app.Accounts(ctx); aerr == nil {
						for _, account := range accounts {
							if account.HomeAccountID != id {
								continue
							}
							result, serr := app.AcquireTokenSilent(ctx, AADScopes, public.WithSilentAccount(account))
							if serr == nil {
								return result, nil
							}
							return public.AuthResult{}, fmt.Errorf("auto-login completed but silent retry failed: %w", serr)
						}
					}
				}
				return public.AuthResult{}, fmt.Errorf("auto-login completed but no matching account in cache")
			case errors.Is(err, ErrAutoLoginUnsupported):
				fmt.Fprintln(os.Stderr,
					"[ado-slim] Auto-login not supported on this platform; falling through to inline device-code")
			default:
				return public.AuthResult{}, fmt.Errorf("auto-login flow failed: %w", err)
			}
		}
	}

	// 3. Device-code flow (inline).
	dc, err := app.AcquireTokenByDeviceCode(ctx, AADScopes)
	if err != nil {
		return public.AuthResult{}, fmt.Errorf("device-code initiation failed: %w", err)
	}

	// Surface the user-facing instructions before the polling call blocks.
	if opts.DeviceCodeCallback != nil {
		opts.DeviceCodeCallback(dc.Result.Message, dc.Result.UserCode, dc.Result.VerificationURL)
	} else {
		fmt.Fprintf(os.Stderr, "[ado-slim] %s\n", dc.Result.Message)
	}

	// Persist the pending message so a parallel viewer (e.g. an editor watching
	// for the file) can render it without scraping stderr. Best-effort.
	if path, err := GetPendingDeviceCodePath(org); err == nil {
		_ = os.WriteFile(path, []byte(dc.Result.Message), 0o600)
	}

	result, err := dc.AuthenticationResult(ctx)
	if err != nil {
		return public.AuthResult{}, fmt.Errorf("device-code authentication failed: %w", err)
	}
	if result.Account.HomeAccountID != "" {
		if err := SaveHomeAccountID(org, result.Account.HomeAccountID); err != nil {
			fmt.Fprintf(os.Stderr, "[ado-slim] failed to persist account id: %v\n", err)
		}
	}
	ClearPendingDeviceCode(org)
	return result, nil
}

// aadProvider is lazy: account may be zero-valued at construction; the first
// Authorization call will resolve it from the on-disk cache or, if no cache
// exists, trigger a non-blocking interactive login spawn and return
// ErrLoginRequired. Subsequent calls succeed silently once the spawned login
// completes — no MCP reconnect required.
type aadProvider struct {
	app public.Client
	org string

	mu            sync.Mutex
	account       public.Account
	spawnInFlight bool
}

// NewAADProvider constructs a lazy TokenProvider. account may be the zero
// value; in that case the first Authorization call resolves it from the
// on-disk cache or spawns an interactive login window.
func NewAADProvider(app public.Client, org string, account public.Account) TokenProvider {
	return &aadProvider{app: app, org: org, account: account}
}

func (p *aadProvider) Mode() AuthMode { return ModeAAD }

// resolveAccount returns the bound account if any, otherwise looks it up from
// the on-disk cache (LoadHomeAccountID + app.Accounts). The latter is a free
// in-memory lookup — MSAL Go's Accounts() reads the cache only.
func (p *aadProvider) resolveAccount(ctx context.Context) (public.Account, bool) {
	p.mu.Lock()
	acct := p.account
	p.mu.Unlock()

	if acct.HomeAccountID != "" {
		return acct, true
	}

	id, ok := LoadHomeAccountID(p.org)
	if !ok {
		return public.Account{}, false
	}
	accounts, err := p.app.Accounts(ctx)
	if err != nil {
		return public.Account{}, false
	}
	for _, a := range accounts {
		if a.HomeAccountID == id {
			p.mu.Lock()
			p.account = a
			p.mu.Unlock()
			return a, true
		}
	}
	return public.Account{}, false
}

// triggerSpawn launches a non-blocking interactive login goroutine if one is
// not already in flight. Concurrent failed Authorization calls coalesce onto
// a single spawn.
func (p *aadProvider) triggerSpawn() {
	p.mu.Lock()
	if p.spawnInFlight {
		p.mu.Unlock()
		return
	}
	p.spawnInFlight = true
	p.mu.Unlock()
	go p.runSpawn()
}

// runSpawn is the goroutine that owns the spawned --login child process.
// It uses context.Background() — never the per-request ctx — because the
// per-request ctx is cancelled when the failed tool call returns, which
// would kill the spawned login.
func (p *aadProvider) runSpawn() {
	defer func() {
		p.mu.Lock()
		p.spawnInFlight = false
		p.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := SpawnInteractiveLogin(ctx, p.org, 5*time.Minute); err != nil {
		fmt.Fprintf(os.Stderr, "[ado-slim] auto-login spawn failed: %v\n", err)
		return
	}

	// Best-effort: bind the freshly written account so the next Authorization
	// call hits silent-refresh fast path. Failure here is fine — resolveAccount
	// will retry on the next call.
	id, ok := LoadHomeAccountID(p.org)
	if !ok {
		return
	}
	accounts, err := p.app.Accounts(ctx)
	if err != nil {
		return
	}
	for _, a := range accounts {
		if a.HomeAccountID == id {
			p.mu.Lock()
			p.account = a
			p.mu.Unlock()
			return
		}
	}
}

func (p *aadProvider) Authorization(ctx context.Context) (string, error) {
	acct, ok := p.resolveAccount(ctx)
	if !ok {
		p.triggerSpawn()
		return "", ErrLoginRequired
	}

	res, err := p.app.AcquireTokenSilent(ctx, AADScopes, public.WithSilentAccount(acct))
	if err == nil {
		return "Bearer " + res.AccessToken, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}

	fmt.Fprintf(os.Stderr,
		"[ado-slim] AAD silent refresh failed; spawning login window: %v\n", err)
	p.triggerSpawn()
	return "", ErrLoginRequired
}

// ResolveAuth picks the active TokenProvider based on env, sets
// activeAuthMode, and returns the provider plus the resolved mode.
//
// Mirrors `resolveAuth()` in src/auth.ts.
func ResolveAuth(ctx context.Context, org string) (TokenProvider, AuthMode, error) {
	mode, err := ResolveAuthMode()
	if err != nil {
		return nil, "", err
	}
	switch mode {
	case ModePAT:
		pat := GetPATFromEnv()
		if pat == "" {
			return nil, "", fmt.Errorf("ADO_PAT environment variable is required when auth mode is 'pat'. " +
				"Create a PAT at https://dev.azure.com/{org}/_usersSettings/tokens")
		}
		SetActiveAuthMode(ModePAT)
		return NewPATProvider(pat), ModePAT, nil
	case ModeAAD:
		app, err := NewMSALApp(org)
		if err != nil {
			return nil, "", err
		}
		// Best-effort in-memory pre-bind: load the home-account-id from disk
		// and ask MSAL for accounts. app.Accounts only reads the on-disk cache
		// (no network), so this is essentially free and lets the first tool
		// call go straight to silent-refresh when a cache exists. If pre-bind
		// fails, the lazy provider resolves the account on first call.
		var acct public.Account
		if id, ok := LoadHomeAccountID(org); ok {
			if accs, aerr := app.Accounts(ctx); aerr == nil {
				for _, a := range accs {
					if a.HomeAccountID == id {
						acct = a
						break
					}
				}
			}
		}
		SetActiveAuthMode(ModeAAD)
		return NewAADProvider(app, org, acct), ModeAAD, nil
	default:
		return nil, "", fmt.Errorf("unsupported auth mode: %s", mode)
	}
}
