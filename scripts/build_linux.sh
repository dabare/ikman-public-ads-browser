#!/usr/bin/env sh
set -eu

APP_NAME="${APP_NAME:-ikman-browser}"
ARCH="${ARCH:-amd64}"
OUT_DIR="${OUT_DIR:-dist}"
VERSION="${VERSION:-dev}"

mkdir -p "$OUT_DIR"

printf 'Building %s for Linux/%s\n' "$APP_NAME" "$ARCH"
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build \
  -trimpath \
  -ldflags "-s -w -X main.version=$VERSION" \
  -o "$OUT_DIR/$APP_NAME-linux-$ARCH" \
  ./cmd/ikman-browser

printf 'Built %s\n' "$OUT_DIR/$APP_NAME-linux-$ARCH"
