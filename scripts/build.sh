#!/usr/bin/env sh
set -eu

OUT_DIR="${OUT_DIR:-dist}"
VERSION="${VERSION:-dev}"
APP_NAME="${APP_NAME:-ikman-browser}"
export OUT_DIR VERSION APP_NAME

mkdir -p "$OUT_DIR"

ARCH=arm64 ./scripts/build_macos.sh
ARCH=amd64 ./scripts/build_macos.sh
ARCH=amd64 ./scripts/build_windows.sh
ARCH=amd64 ./scripts/build_linux.sh

printf 'All builds are in %s\n' "$OUT_DIR"
