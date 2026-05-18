#!/usr/bin/env sh
set -eu

APP_NAME="${APP_NAME:-ikman-browser}"
ARCH="${ARCH:-arm64}"
OUT_DIR="${OUT_DIR:-dist}"
VERSION="${VERSION:-dev}"

mkdir -p "$OUT_DIR"

printf 'Building %s for macOS/%s\n' "$APP_NAME" "$ARCH"
CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH" go build \
  -trimpath \
  -ldflags "-s -w -X main.version=$VERSION" \
  -o "$OUT_DIR/$APP_NAME-darwin-$ARCH" \
  ./cmd/ikman-browser

printf 'Built %s\n' "$OUT_DIR/$APP_NAME-darwin-$ARCH"
