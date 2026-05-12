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

## CLI

```text
ado-slim-mcp                       Start the MCP stdio server (default).
ado-slim-mcp --login [--force]     Run AAD device-code login interactively.
ado-slim-mcp --help                Show usage.
```

`--force` skips the silent-from-cache path and runs a fresh device-code flow.

## MCP client config

### Claude Desktop / generic stdio client (PAT)

```json
{
  "mcpServers": {
    "ado-slim": {
      "command": "C:/path/to/ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "your-org",
        "ADO_PAT": "your-pat"
      }
    }
  }
}
```

### AAD (no PAT)

```json
{
  "mcpServers": {
    "ado-slim": {
      "command": "C:/path/to/ado-slim-mcp.exe",
      "env": {
        "ADO_ORG": "your-org"
      }
    }
  }
}
```

Run `ado-slim-mcp --login` first.

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
