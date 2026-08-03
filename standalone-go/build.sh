#!/bin/sh
# Builds Linux + Windows binaries from this one machine -- Go can cross-compile with zero extra
# tooling, unlike the PyInstaller-based Python version this replaces (PyInstaller can only ever
# produce a binary for the OS it's run ON). Requires only the Go toolchain -- no external Go
# modules, nothing else to install.
#
# No Mac build: Elite Dangerous itself has no supported native Mac client anymore (Frontier
# pulled the old Mac port years ago, and Apple Silicon Macs can't Boot Camp into Windows), so
# there's no realistic audience with journal files to point a native Mac binary at.
#
# Usage: ./build.sh [output-dir]   (defaults to ../dist-go)

set -e
cd "$(dirname "$0")"
OUT="${1:-../dist-go}"
mkdir -p "$OUT"

# Regenerate the Windows version-info/manifest resource (winres/winres.json -> .syso) if
# go-winres is available. A version-info-less, unsigned exe is exactly the shape Windows
# Defender's ML heuristic tends to flag as a false positive (Trojan:Win32/Wacatac.B!ml) -- this
# is the free (no-cert-purchase) mitigation for that. See README.md for more.
if command -v go-winres >/dev/null 2>&1; then
    echo "Regenerating Windows version-info resource..."
    go-winres make --arch amd64
fi

echo "Building Linux..."
go build -ldflags="-s -w" -o "$OUT/edjournalatlas-linux" .

echo "Building Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$OUT/edjournalatlas-windows.exe" .

echo
echo "Done. Sizes:"
ls -la "$OUT"
