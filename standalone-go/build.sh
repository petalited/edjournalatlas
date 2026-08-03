#!/bin/sh
# Builds all four platform binaries from this one machine -- Go can cross-compile with zero
# extra tooling, unlike the PyInstaller-based Python version this replaces (PyInstaller can only
# ever produce a binary for the OS it's run ON). Requires only the Go toolchain -- no external
# Go modules, nothing else to install.
#
# Usage: ./build.sh [output-dir]   (defaults to ../dist-go)

set -e
cd "$(dirname "$0")"
OUT="${1:-../dist-go}"
mkdir -p "$OUT"

# Regenerate the Windows version-info/manifest resource (winres/winres.json -> .syso) if
# go-winres is available -- see docs/StandaloneGoRewrite.md's "Windows Defender false positive"
# section for why this exists: a version-info-less, unsigned exe is exactly the shape Defender's
# ML heuristic tends to flag, and this is the free (no-cert-purchase) mitigation for that.
if command -v go-winres >/dev/null 2>&1; then
    echo "Regenerating Windows version-info resource..."
    go-winres make --arch amd64
fi

echo "Building Linux..."
go build -ldflags="-s -w" -o "$OUT/edexotracker-standalone-linux" .

echo "Building Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$OUT/edexotracker-standalone-windows.exe" .

echo "Building Mac (Intel)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$OUT/edexotracker-standalone-mac-intel" .

echo "Building Mac (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$OUT/edexotracker-standalone-mac-arm" .

echo
echo "Done. Sizes:"
ls -la "$OUT"
