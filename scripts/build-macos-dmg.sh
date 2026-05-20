#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_NAME="${APP_NAME:-foghorn}"
VOLUME_NAME="${VOLUME_NAME:-Foghorn}"
APP_PATH="${APP_PATH:-$ROOT_DIR/build/bin/${APP_NAME}.app}"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/build/dist}"
DMG_PATH="${DMG_PATH:-$DIST_DIR/${APP_NAME}.dmg}"
BUILD_APP=1

usage() {
  cat <<EOF
Usage: $(basename "$0") [--skip-build] [--app-path PATH] [--dmg-path PATH] [--] [wails build args...]

Builds a macOS .app bundle and packages it into a compressed DMG.

Options:
  --skip-build      Package an existing .app bundle instead of running the app build first.
  --app-path PATH   Path to the .app bundle to package. Defaults to $APP_PATH.
  --dmg-path PATH   Output DMG path. Defaults to $DMG_PATH.
  -h, --help        Show this help.
EOF
}

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required tool: $1" >&2
    exit 1
  fi
}

BUILD_ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build)
      BUILD_APP=0
      shift
      ;;
    --app-path)
      APP_PATH="$2"
      shift 2
      ;;
    --dmg-path)
      DMG_PATH="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      BUILD_ARGS+=("$@")
      break
      ;;
    *)
      BUILD_ARGS+=("$1")
      shift
      ;;
  esac
done

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "DMG packaging requires macOS." >&2
  exit 1
fi

require_tool hdiutil

if [[ "$BUILD_APP" -eq 1 ]]; then
  "$ROOT_DIR/scripts/build-macos-app.sh" "${BUILD_ARGS[@]}"
fi

if [[ ! -d "$APP_PATH" ]]; then
  echo "Expected app bundle not found: $APP_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$DMG_PATH")"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/foghorn-dmg.XXXXXX")"
STAGING_DIR="$TMP_DIR/staging"
RW_DMG="$TMP_DIR/${APP_NAME}-rw.dmg"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$STAGING_DIR"
cp -R "$APP_PATH" "$STAGING_DIR/"
ln -s /Applications "$STAGING_DIR/Applications"

hdiutil create \
  -quiet \
  -volname "$VOLUME_NAME" \
  -srcfolder "$STAGING_DIR" \
  -fs HFS+ \
  -fsargs "-c c=64,a=16,e=16" \
  -format UDRW \
  "$RW_DMG"

rm -f "$DMG_PATH"
hdiutil convert "$RW_DMG" -quiet -format UDZO -imagekey zlib-level=9 -o "$DMG_PATH"

echo "Created $DMG_PATH"
