# ado-slim-mcp

A read-only MCP server for Azure DevOps that returns **slim DTOs** instead
of raw API payloads — typically 70-85% smaller, so your LLM session
context isn't burned on `_links`, audit metadata, and HTML-wrapped fields
you didn't ask for.

## Why this exists

Microsoft ships an official Azure DevOps MCP (`@azure-devops/mcp`), but
every tool returns `JSON.stringify(apiResponse)` — the full raw payload.
On a typical work-item or PR call, ~92% of the bytes are metadata the
model never uses, and they evict useful context from the window.

This server exposes the same surface area Microsoft's does (minus all
writes — see below), but every response passes through a hand-written
transform that strips `_links`, flattens identity objects, maps numeric
enums to readable strings, and removes HTML wrappers. Same questions
answerable; a fraction of the tokens.

It started life as a TypeScript reimplementation; it's now distributed
as a single ~9 MB Go binary so you don't need Node on the machine that
runs it.

## Build

Requires Go ≥ 1.22.

```sh
# Native build (Windows produces .exe, others produce a bare binary).
make build               # or: ./build.sh / ./build.ps1

# Cross-compile to all supported targets (windows/amd64, linux/amd64, darwin/arm64).
make build-all           # or: ./build.sh --all / ./build.ps1 -All
```

The default build flags are `CGO_ENABLED=0 -ldflags="-s -w" -trimpath`, yielding
a fully static binary with debug info stripped (~9 MB on Windows).

`UPX` compression is supported but **off by default** — pass `-Upx` to
`build.ps1` to enable. Tradeoffs:

- **+** Halves the binary (~9 MB → ~4 MB).
- **-** Triggers AV false positives on some Windows machines.
- **-** Adds a small decompression stub to startup time.

## Configuration

| Env var               | Required? | Default          | Notes                                             |
| --------------------- | --------- | ---------------- | ------------------------------------------------- |
| `ADO_ORG`             | yes       | -                | Azure DevOps organization name.                   |
| `ADO_PAT`             | one of    | -                | If set, selects PAT auth and skips AAD.           |
| `ADO_AUTH_MODE`       | no        | auto             | `"pat"` or `"aad"` — overrides auto-detect.       |
| `ADO_AAD_TENANT`      | no        | `organizations`  | AAD tenant for device-code auth.                  |
| `ADO_AAD_CLIENT_ID`   | no        | Azure CLI client | AAD public client to authorize against.           |
| `ADO_AUTO_LOGIN`      | no        | on               | Set to `0` to disable auto-spawning a login window when stdin is not a TTY (Windows). |

When using AAD, run `ado-slim-mcp --login` once **per org** to populate the
token cache. `--login` requires `ADO_ORG` to be set, since cache filenames
are now scoped per org. The server then starts silently on subsequent runs.

### First-run AAD login (Windows)

When the server is launched by an MCP client (e.g. Claude Code) over stdio
and there is no usable token cache for the configured `ADO_ORG`, the server
will automatically spawn `ado-slim-mcp.exe --login` in a **new console
window**. The user completes the device-code flow in that window; the
parent server polls the per-org account file and continues silently once
the cache has been written.

Notes:

- Closing the spawned window before completing login causes the parent
  server to fail (the MCP client will report a startup error). Restart and
  retry.
- Default timeout is 5 minutes; after that the parent gives up.
- To opt out of auto-spawn (e.g. CI / scripted contexts), set
  `ADO_AUTO_LOGIN=0`. The server will then write the device code to
  stderr inline as before — note that this output is typically invisible
  under MCP launchers.

### Per-org token caches

Cache filenames are scoped per `ADO_ORG` so two simultaneous sessions
targeting different orgs (especially in different AAD tenants) do not
collide:

- `token-cache-<org>.json` — MSAL serialized cache (mode 0600 on POSIX).
- `account-<org>.json` — persisted home-account ID for silent re-auth.
- `pending-device-code-<org>.txt` — transient, written during device-code login.

`<org>` is lowercased and reduced to `[a-z0-9-]` (other characters become
`-`, runs collapsed, leading/trailing trimmed).

Any legacy `token-cache.json` / `account.json` from older builds is left
in place but ignored. Run `--login` once per org to populate the new
files.

### Cache locations

- **Windows**: `%APPDATA%\ado-slim-mcp\`
- **Other**:   `${XDG_CONFIG_HOME:-~/.config}/ado-slim-mcp/`

On **Windows**, the token cache file (`token-cache-<org>.json`) is
encrypted at rest using **DPAPI** (`CryptProtectData`, `CurrentUser`
scope, app-bound via a fixed entropy value). The `.json` extension is
retained for historical reasons — the on-disk bytes are an opaque
DPAPI blob prefixed with a small magic header, not JSON. Legacy
plaintext caches written by older builds are still loaded
transparently and are **silently upgraded** to the encrypted format
on the next write.

On **macOS** and **Linux**, the cache file remains plaintext JSON with
file mode `0600`. Encryption-at-rest on those platforms is a future
follow-up.

## CLI

```text
ado-slim-mcp                       Start the MCP stdio server (default).
ado-slim-mcp --login [--force]     Run AAD device-code login interactively.
ado-slim-mcp --help                Show usage.
```

`--force` skips the silent-from-cache path and runs a fresh device-code flow.

## MCP client config

`ado-slim` is a local stdio MCP server. Register it with your client using one of the snippets below. All clients require `ADO_ORG` (your Azure DevOps organization name); optionally set `ADO_PAT` for PAT auth, or omit it to use AAD device-code auth on first call.

**First-time AAD setup:** Before wiring `ado-slim` into a client, run this once in a terminal to seed the token cache and avoid startup timeouts:

```bash
ado-slim-mcp --login        # macOS/Linux
ado-slim-mcp.exe --login    # Windows
```

### Claude Desktop

| OS      | Config file path                                                  |
| ------- | ----------------------------------------------------------------- |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json`                     |
| macOS   | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Linux   | `~/.config/Claude/claude_desktop_config.json`                     |

PAT auth:

```json
{
  "mcpServers": {
    "ado-slim": {
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "YourOrgName",
        "ADO_PAT": "your-pat-here"
      }
    }
  }
}
```

AAD auth — omit `ADO_PAT`:

```json
{
  "mcpServers": {
    "ado-slim": {
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "YourOrgName"
      }
    }
  }
}
```

On macOS/Linux use `/path/to/ado-slim-mcp` (no `.exe`, no escaping). After editing, fully quit Claude (not just close the window) and restart.

### Claude Code CLI

| Scope               | Path                                |
| ------------------- | ----------------------------------- |
| Project (committed) | `<repo>/.mcp.json`                  |
| User                | `~/.claude.json`                    |
| Local (uncommitted) | `<repo>/.claude/settings.local.json` |

CLI shortcut (recommended — sidesteps Windows path-escaping):

```bash
# Project scope
claude mcp add ado-slim -s project \
  --env ADO_ORG=YourOrgName \
  -- C:\path\to\ado-slim-mcp.exe

# User scope
claude mcp add ado-slim -s user \
  --env ADO_ORG=YourOrgName \
  -- C:\path\to\ado-slim-mcp.exe
```

The `--` is required — it separates `claude` flags from the server command.

Equivalent JSON for `.mcp.json`:

```json
{
  "mcpServers": {
    "ado-slim": {
      "type": "stdio",
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "YourOrgName",
        "ADO_PAT": "your-pat-here"
      }
    }
  }
}
```

For AAD auth, omit `ADO_PAT`.

### VS Code (Copilot Agent Mode)

| Scope     | Path                                                                                                                                                                  |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Workspace | `<repo>/.vscode/mcp.json`                                                                                                                                             |
| User      | `%APPDATA%\Code\User\mcp.json` (Windows) <br> `~/Library/Application Support/Code/User/mcp.json` (macOS) <br> `~/.config/Code/User/mcp.json` (Linux) |

> **⚠️ Key difference:** VS Code uses `"servers"`, not `"mcpServers"`. This is the most common copy-paste error when porting a snippet from Claude or Cursor.

PAT auth:

```json
{
  "servers": {
    "ado-slim": {
      "type": "stdio",
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "YourOrgName",
        "ADO_PAT": "your-pat-here"
      }
    }
  }
}
```

AAD auth with a prompted org name:

```json
{
  "inputs": [
    { "id": "adoOrg", "type": "promptString", "description": "Azure DevOps org" }
  ],
  "servers": {
    "ado-slim": {
      "type": "stdio",
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": { "ADO_ORG": "${input:adoOrg}" }
    }
  }
}
```

Requires VS Code 1.99+ with Copilot agent mode. Accept the workspace-trust prompt on first run.

### Cursor

| Scope   | Path                                                                                       |
| ------- | ------------------------------------------------------------------------------------------ |
| Project | `<repo>/.cursor/mcp.json`                                                                  |
| Global  | `%USERPROFILE%\.cursor\mcp.json` (Windows) <br> `~/.cursor/mcp.json` (macOS/Linux) |

PAT auth:

```json
{
  "mcpServers": {
    "ado-slim": {
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "YourOrgName",
        "ADO_PAT": "your-pat-here"
      }
    }
  }
}
```

AAD auth with env passthrough:

```json
{
  "mcpServers": {
    "ado-slim": {
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "${env:ADO_ORG}"
      }
    }
  }
}
```

Restart Cursor after editing. Settings → Tools & MCP shows a green dot when the server is up.

### Codex CLI & IDE

The Codex CLI, IDE extension, and desktop app all share one config file. There is no separate "Codex Desktop" config.

| OS              | Path                                |
| --------------- | ----------------------------------- |
| Windows         | `%USERPROFILE%\.codex\config.toml`  |
| macOS / Linux   | `~/.codex/config.toml`              |

CLI shortcut:

```bash
codex mcp add ado-slim \
  --env ADO_ORG=YourOrgName \
  -- C:\path\to\ado-slim-mcp.exe
```

TOML (PAT auth):

```toml
[mcp_servers.ado-slim]
command = "C:\\path\\to\\ado-slim-mcp.exe"

[mcp_servers.ado-slim.env]
ADO_ORG = "YourOrgName"
ADO_PAT = "your-pat-here"
```

TOML (AAD auth — extend startup timeout for first-run device-code flow):

```toml
[mcp_servers.ado-slim]
command = "C:\\path\\to\\ado-slim-mcp.exe"
startup_timeout_sec = 60

[mcp_servers.ado-slim.env]
ADO_ORG = "YourOrgName"
```

Codex defaults to a 10-second startup timeout, which AAD device-code login can exceed. Running `ado-slim-mcp --login` once in a terminal beforehand avoids this.

### GitHub Copilot CLI

The standalone `copilot` CLI (distinct from `gh copilot`).

| OS              | Path                                       |
| --------------- | ------------------------------------------ |
| Windows         | `%USERPROFILE%\.copilot\mcp-config.json`   |
| macOS / Linux   | `~/.copilot/mcp-config.json`               |

In-CLI:

```
/mcp add
/mcp show ado-slim
```

JSON:

```json
{
  "mcpServers": {
    "ado-slim": {
      "type": "stdio",
      "command": "C:\\path\\to\\ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "YourOrgName",
        "ADO_PAT": "your-pat-here"
      }
    }
  }
}
```

For AAD auth, omit `ADO_PAT`.

### ChatGPT Desktop (not supported)

ChatGPT Desktop's "connectors" / Developer Mode supports only remote HTTP MCP servers via its UI; it cannot register a local stdio binary like `ado-slim`. Use Claude Desktop, VS Code, Cursor, or one of the CLIs instead.

## Tools

31 read-only tools, grouped by domain:

- **core (3)**: `list_projects`, `list_project_teams`, `get_identity_ids`
- **work-items (10)**: `get_work_item`, `get_work_items_batch`,
  `list_work_item_comments`, `list_work_item_revisions`, `my_work_items`,
  `get_work_items_for_iteration`, `get_query`, `get_query_results`,
  `run_wiql`, `list_iterations`
- **repositories (13)**: `list_repos`, `get_repo`, `list_branches`,
  `get_branch`, `search_commits`, `get_commit_changes`, `list_pull_requests`,
  `get_pull_request`, `get_pull_request_changes`, `list_pull_request_threads`,
  `list_pull_request_thread_comments`, `list_directory`, `get_file_content`
- **pipelines (2)**: `list_pipeline_runs`, `get_pipeline_run`
- **search (3)**: `search_code`, `search_wiki`, `search_workitem`

## Read-only invariant

The HTTP client rejects every method except `GET`, plus a tightly scoped
`POST` allowlist for endpoints that ADO REST exposes as POST but are
semantically read-only:

- `https://almsearch.dev.azure.com/...` — Search API (POST queries)
- `https://dev.azure.com/.../_apis/wit/wiql` — WIQL ad-hoc query
- `https://dev.azure.com/.../_apis/wit/wiql/{id}` — saved-query execute
- `https://dev.azure.com/.../_apis/wit/queries/{id}` — saved-query get

All other methods (PUT/PATCH/DELETE) and any other POST target return
`ErrWriteAttempted` immediately, before the request leaves the process.
This is unit-tested in `internal/ado/client_readonly_test.go`.

## Development

```sh
go vet ./...
go test ./...
```

Pure transforms in `internal/slim/` are covered by `transforms_test.go`.
The read-only HTTP guard is covered by `internal/ado/client_readonly_test.go`.
End-to-end tool behavior is **not** covered by automated tests — it requires
a live ADO org and is verified manually per `SMOKE_TEST_RESULTS.md` (TBD).

## Layout

```
ado-slim-mcp-go/
  cmd/server/main.go             CLI parse + dispatch
  internal/auth/                 PAT/AAD providers, MSAL cache, mode resolve
  internal/ado/                  HTTP client, read-only guard, URL helpers, validate
  internal/slim/                 DTO structs + pure transforms (StripHtml, FlattenIdentity, ...)
  internal/tools/                31 tool registrations + error handler
  internal/login/                --login subcommand, cross-platform browser/clipboard
  bin/                           build outputs
```
