#!/usr/bin/env bash
#
# Build the Foghorn macOS DMG installer.
#
# 1. In Developer ID mode: sets up a temp keychain with the imported cert so
#    both the .app and the DMG can be signed from the same identity.
# 2. Delegates to build-macos-app.sh to produce + sign the universal .app.
# 3. Wraps the .app in a DMG via `create-dmg`.
# 4. In Developer ID mode: signs + notarizes + staples the DMG.
#
# Final artifact (printed to stdout on success):
#   build/bin/foghorn-<version>-universal.dmg

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT_DIR/build/bin"
APP_PATH="$BIN_DIR/foghorn.app"

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required tool: $1" >&2
    if [[ "$1" == "create-dmg" ]]; then
      echo "Install with: npm install -g create-dmg" >&2
    fi
    exit 1
  fi
}

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script is for macOS only." >&2
  exit 1
fi

require_tool create-dmg

# Resolve the version once, up front. Export it so build-macos-app.sh reuses
# the exact same string (avoids any drift if the working tree changes between
# calls).
FOGHORN_VERSION="$("$ROOT_DIR/scripts/version.sh")"
export FOGHORN_VERSION

# Decide signing mode the same way build-macos-app.sh does, so we know whether
# to sign/notarize the DMG. Keep these two checks in sync.
DEV_ID_VARS=(
  APPLE_DEVELOPER_ID_CERT_P12_BASE64
  APPLE_DEVELOPER_ID_CERT_PASSWORD
  APPLE_ID
  APPLE_APP_SPECIFIC_PASSWORD
  APPLE_TEAM_ID
)
set_count=0
for v in "${DEV_ID_VARS[@]}"; do
  [[ -n "${!v:-}" ]] && set_count=$((set_count + 1))
done
if [[ "$set_count" -eq "${#DEV_ID_VARS[@]}" ]]; then
  SIGNING_MODE="developer-id"
elif [[ "$set_count" -eq 0 ]]; then
  SIGNING_MODE="ad-hoc"
else
  # Let build-macos-app.sh emit the specific missing-var diagnostic.
  "$ROOT_DIR/scripts/build-macos-app.sh"
  exit $?
fi

STAGE_DIR="$(mktemp -d)"
KEYCHAIN_DIR=""
KEYCHAIN_PATH=""

cleanup() {
  rm -rf "$STAGE_DIR"
  if [[ -n "$KEYCHAIN_PATH" ]]; then
    security delete-keychain "$KEYCHAIN_PATH" >/dev/null 2>&1 || true
  fi
  if [[ -n "$KEYCHAIN_DIR" ]]; then
    rm -rf "$KEYCHAIN_DIR"
  fi
}
trap cleanup EXIT

# In Developer ID mode, set up the keychain here so both the .app (signed by
# build-macos-app.sh) and the DMG (signed below) use the same identity.
if [[ "$SIGNING_MODE" == "developer-id" ]]; then
  KEYCHAIN_DIR="$(mktemp -d)"
  KEYCHAIN_PATH="$KEYCHAIN_DIR/foghorn-build.keychain-db"
  KEYCHAIN_PASSWORD="$(uuidgen)"
  P12_PATH="$KEYCHAIN_DIR/cert.p12"

  printf '%s' "$APPLE_DEVELOPER_ID_CERT_P12_BASE64" | base64 --decode > "$P12_PATH"

  security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
  security set-keychain-settings -lut 21600 "$KEYCHAIN_PATH"
  security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
  security import "$P12_PATH" \
    -k "$KEYCHAIN_PATH" \
    -P "$APPLE_DEVELOPER_ID_CERT_PASSWORD" \
    -T /usr/bin/codesign
  security set-key-partition-list \
    -S apple-tool:,apple:,codesign: \
    -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH" >/dev/null

  ORIG_KEYCHAINS="$(security list-keychains -d user | tr -d '"' | xargs)"
  # shellcheck disable=SC2086
  security list-keychains -d user -s "$KEYCHAIN_PATH" $ORIG_KEYCHAINS

  export FOGHORN_SIGNING_KEYCHAIN="$KEYCHAIN_PATH"
fi

# Build + sign the .app using the shared keychain (in dev-id mode).
"$ROOT_DIR/scripts/build-macos-app.sh"

if [[ ! -d "$APP_PATH" ]]; then
  echo "build-macos-app.sh did not produce $APP_PATH." >&2
  exit 1
fi

DMG_NAME="foghorn-${FOGHORN_VERSION}-universal.dmg"
DMG_PATH="$BIN_DIR/$DMG_NAME"

mkdir -p "$BIN_DIR"
rm -f "$DMG_PATH"

# create-dmg writes into the current directory with its own naming. Run it in
# a temp dir, then rename to our canonical filename.
if [[ "$SIGNING_MODE" == "developer-id" ]]; then
  (cd "$STAGE_DIR" && create-dmg --overwrite "$APP_PATH" "$STAGE_DIR")
else
  # Ad-hoc: tell create-dmg not to attempt Developer ID signing. Some versions
  # still exit non-zero when no identity is available but emit a usable DMG,
  # so we accept that and verify the file below.
  (cd "$STAGE_DIR" && create-dmg --overwrite --identity=- "$APP_PATH" "$STAGE_DIR") || true
fi

PRODUCED_DMG="$(find "$STAGE_DIR" -maxdepth 1 -name '*.dmg' -print -quit)"
if [[ -z "$PRODUCED_DMG" || ! -f "$PRODUCED_DMG" ]]; then
  echo "create-dmg did not produce a .dmg file." >&2
  exit 1
fi
mv "$PRODUCED_DMG" "$DMG_PATH"

if [[ "$SIGNING_MODE" == "developer-id" ]]; then
  require_tool xcrun

  IDENTITY="$(security find-identity -v -p codesigning "$KEYCHAIN_PATH" \
    | awk -F'"' '/Developer ID Application/ {print $2; exit}')"
  if [[ -z "$IDENTITY" ]]; then
    echo "Developer ID Application identity not available for DMG signing." >&2
    exit 1
  fi

  codesign --sign "$IDENTITY" --keychain "$KEYCHAIN_PATH" --timestamp "$DMG_PATH"

  echo "Submitting DMG for notarization (this can take several minutes)..."
  xcrun notarytool submit "$DMG_PATH" \
    --apple-id "$APPLE_ID" \
    --password "$APPLE_APP_SPECIFIC_PASSWORD" \
    --team-id "$APPLE_TEAM_ID" \
    --wait

  xcrun stapler staple "$DMG_PATH"
  xcrun stapler validate "$DMG_PATH"
fi

echo "$DMG_PATH"
