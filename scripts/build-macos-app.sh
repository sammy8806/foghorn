#!/usr/bin/env bash
#
# Build the Foghorn macOS .app bundle (universal: arm64 + amd64) and sign it.
#
# Signing modes (decided from environment):
#   - Developer ID: all of APPLE_DEVELOPER_ID_CERT_P12_BASE64,
#     APPLE_DEVELOPER_ID_CERT_PASSWORD, APPLE_ID, APPLE_APP_SPECIFIC_PASSWORD,
#     APPLE_TEAM_ID are set. Imports the cert into a temp keychain, signs with
#     hardened runtime + timestamp. Notarization is performed by build-dmg.sh.
#   - Ad-hoc: NONE of those vars are set. Signs locally so macOS notifications
#     work, but Gatekeeper will warn users.
#   - Error: partial set — fail fast, no silent fallback.
#
# Keychain lifecycle:
#   - If $FOGHORN_SIGNING_KEYCHAIN is set (by a caller like build-dmg.sh), use
#     it and do NOT delete it — the caller owns the cleanup.
#   - Otherwise create a temp keychain for this invocation and clean it up on
#     exit via trap.
#
# The resolved version is injected via -ldflags "-X main.version=<version>".

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

VERSION="$("$ROOT_DIR/scripts/version.sh")"

# Decide signing mode.
DEV_ID_VARS=(
  APPLE_DEVELOPER_ID_CERT_P12_BASE64
  APPLE_DEVELOPER_ID_CERT_PASSWORD
  APPLE_ID
  APPLE_APP_SPECIFIC_PASSWORD
  APPLE_TEAM_ID
)
set_count=0
unset_list=()
for v in "${DEV_ID_VARS[@]}"; do
  if [[ -n "${!v:-}" ]]; then
    set_count=$((set_count + 1))
  else
    unset_list+=("$v")
  fi
done

SIGNING_MODE=""
if [[ "$set_count" -eq "${#DEV_ID_VARS[@]}" ]]; then
  SIGNING_MODE="developer-id"
elif [[ "$set_count" -eq 0 ]]; then
  SIGNING_MODE="ad-hoc"
else
  echo "Partial Developer ID configuration detected. Missing:" >&2
  for v in "${unset_list[@]}"; do
    echo "  - $v" >&2
  done
  echo "Either set all of these or none (to fall back to ad-hoc signing)." >&2
  exit 1
fi

case "$SIGNING_MODE" in
  developer-id)
    echo "Signing mode: Developer ID (will notarize at DMG step)"
    ;;
  ad-hoc)
    echo "Signing mode: ad-hoc (NOT notarized — users will see Gatekeeper warnings)"
    ;;
esac

echo "Version: $VERSION"

cd "$ROOT_DIR"

# Forward any extra args the caller passed, but always set platform + ldflags.
wails build \
  -platform darwin/universal \
  -ldflags "-X main.version=$VERSION" \
  "$@"

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

if [[ "$SIGNING_MODE" == "developer-id" ]]; then
  # If a caller (e.g. build-dmg.sh) set up the keychain, reuse it. Otherwise
  # create our own and clean up on exit.
  if [[ -n "${FOGHORN_SIGNING_KEYCHAIN:-}" ]]; then
    KEYCHAIN_PATH="$FOGHORN_SIGNING_KEYCHAIN"
    OWNS_KEYCHAIN=0
  else
    KEYCHAIN_DIR="$(mktemp -d)"
    KEYCHAIN_PATH="$KEYCHAIN_DIR/foghorn-build.keychain-db"
    KEYCHAIN_PASSWORD="$(uuidgen)"
    P12_PATH="$KEYCHAIN_DIR/cert.p12"
    OWNS_KEYCHAIN=1

    cleanup() {
      security delete-keychain "$KEYCHAIN_PATH" >/dev/null 2>&1 || true
      rm -rf "$KEYCHAIN_DIR"
    }
    trap cleanup EXIT

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
  fi

  # Find the Developer ID Application identity in the active keychain.
  IDENTITY="$(security find-identity -v -p codesigning "$KEYCHAIN_PATH" \
    | awk -F'"' '/Developer ID Application/ {print $2; exit}')"
  if [[ -z "$IDENTITY" ]]; then
    echo "Developer ID Application identity not found in keychain $KEYCHAIN_PATH." >&2
    exit 1
  fi

  codesign --force --options runtime --timestamp \
    --keychain "$KEYCHAIN_PATH" \
    --sign "$IDENTITY" \
    "$APP_PATH"

  # Silence unused-warning on OWNS_KEYCHAIN under strict linters.
  : "$OWNS_KEYCHAIN"
else
  codesign --force --deep --sign - --identifier "$BUNDLE_ID" "$APP_PATH"
fi

codesign --verify --deep --strict --verbose=2 "$APP_PATH"

echo "Built and signed $APP_PATH ($SIGNING_MODE, version $VERSION, bundle id $BUNDLE_ID)"
