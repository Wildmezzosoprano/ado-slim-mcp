#!/usr/bin/env bash
# Build ado-slim-mcp (Go port). Produces bin/ado-slim-mcp[.exe] stripped.
set -euo pipefail

mkdir -p bin
export CGO_ENABLED=0

LDFLAGS="-s -w"

build() {
  local goos="$1" goarch="$2" name="$3"
  echo "==> Building $name ($goos/$goarch)"
  GOOS="$goos" GOARCH="$goarch" go build -ldflags="$LDFLAGS" -trimpath -o "bin/$name" ./cmd/server
}

case "${1:-}" in
  --all)
    build windows amd64 ado-slim-mcp.exe
    build linux   amd64 ado-slim-mcp-linux-amd64
    build darwin  arm64 ado-slim-mcp-darwin-arm64
    ;;
  *)
    # Default: native build.
    if [[ "${OS:-}" == "Windows_NT" ]]; then
      build windows amd64 ado-slim-mcp.exe
    else
      go build -ldflags="$LDFLAGS" -trimpath -o bin/ado-slim-mcp ./cmd/server
    fi
    ;;
esac
