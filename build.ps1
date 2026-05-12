# Build script for ado-slim-mcp (Go port).
# Produces a stripped, single-file static binary in bin/.
#
# Usage:
#   ./build.ps1                 # build for current platform
#   ./build.ps1 -All            # cross-compile windows/linux/darwin
#   ./build.ps1 -Upx            # also run UPX compression (off by default;
#                                 expect AV false-positives and a startup
#                                 cost from the decompression stub)

param(
    [switch]$All,
    [switch]$Upx
)

$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "0"

$ldflags = "-s -w"
$out = "bin"
New-Item -ItemType Directory -Force -Path $out | Out-Null

function Build($goos, $goarch, $name) {
    Write-Host "==> Building $name ($goos/$goarch)"
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    $target = Join-Path $out $name
    & go build -ldflags="$ldflags" -trimpath -o $target ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "go build failed for $goos/$goarch" }
    if ($Upx) {
        if (Get-Command upx -ErrorAction SilentlyContinue) {
            & upx --best --lzma $target
        } else {
            Write-Warning "upx not on PATH, skipping compression"
        }
    }
}

if ($All) {
    Build "windows" "amd64" "ado-slim-mcp.exe"
    Build "linux"   "amd64" "ado-slim-mcp-linux-amd64"
    Build "darwin"  "arm64" "ado-slim-mcp-darwin-arm64"
} else {
    Build "windows" "amd64" "ado-slim-mcp.exe"
}
