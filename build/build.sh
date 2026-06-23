#!/usr/bin/env bash
# Cross-compile static hub-os-config binaries for Raspberry Pi targets.
#
# Usage: build/build.sh [version]
# Output: dist/hub-os-config-<arch>
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}"
PKG="./cmd/hub-os-config"
OUT="dist"

mkdir -p "$OUT"

build() {
  local name="$1"; shift
  echo "building $name ..."
  # Use `env` so the GOOS/GOARCH/GOARM args (passed via "$@") are applied as
  # environment assignments — the shell only treats literal VAR=value tokens as
  # assignments, not ones that come from expansion.
  env CGO_ENABLED=0 "$@" go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/hub-os-config-$name" "$PKG"
}

# Pi Zero / Zero W / 1            -> ARMv6
build armv6  GOOS=linux GOARCH=arm GOARM=6
# Pi 2 / 3 (32-bit)              -> ARMv7
build armv7  GOOS=linux GOARCH=arm GOARM=7
# Pi 3 / 4 / 5 (64-bit)          -> ARM64
build arm64  GOOS=linux GOARCH=arm64

echo
echo "done -> $OUT/"
ls -lh "$OUT"
