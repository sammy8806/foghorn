#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_NAME="foghorn"
APP_PATH="$ROOT_DIR/build/bin/${APP_NAME}.app"

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required tool: $1" >&2
    exit 1
  fi
}

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script is for macOS only." >&2
  exit 1
fi

require_tool wails
require_tool codesign
require_tool plutil

cd "$ROOT_DIR"
wails build "$@"

if [[ ! -d "$APP_PATH" ]]; then
  echo "Expected app bundle not found: $APP_PATH" >&2
  exit 1
fi

BUNDLE_ID="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$APP_PATH/Contents/Info.plist" 2>/dev/null || true)"
if [[ -z "$BUNDLE_ID" ]]; then
  BUNDLE_ID="$(plutil -extract CFBundleIdentifier raw -o - "$APP_PATH/Contents/Info.plist" 2>/dev/null || true)"
fi
if [[ -z "$BUNDLE_ID" ]]; then
  echo "Unable to determine CFBundleIdentifier from $APP_PATH/Contents/Info.plist" >&2
  exit 1
fi

codesign --force --deep --sign - --identifier "$BUNDLE_ID" "$APP_PATH"
codesign --verify --deep --strict --verbose=2 "$APP_PATH"

echo "Built and re-signed $APP_PATH with identifier $BUNDLE_ID"
