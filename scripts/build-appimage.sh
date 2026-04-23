#!/usr/bin/env bash
#
# Build a self-contained Foghorn AppImage for Linux x86_64.
#
# Bundles WebKitGTK 4.0, GTK3, and AppIndicator3 so the AppImage works on any
# host with glibc >= 2.35 (Ubuntu 22.04, Debian 12, Fedora 36+, RHEL 9+).
#
# Final artifact: build/bin/foghorn-<version>-x86_64.AppImage

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT_DIR/build/bin"
CACHE_DIR="${FOGHORN_BUILD_CACHE:-$ROOT_DIR/build/.cache}"
LINUXDEPLOY_VERSION="1-alpha-20240109-1"
LINUXDEPLOY_URL="https://github.com/linuxdeploy/linuxdeploy/releases/download/${LINUXDEPLOY_VERSION}/linuxdeploy-x86_64.AppImage"
LINUXDEPLOY_GTK_URL="https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "This script is for Linux only." >&2
  exit 1
fi

if [[ "$(uname -m)" != "x86_64" ]]; then
  echo "This script currently supports x86_64 only (got $(uname -m))." >&2
  exit 1
fi

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required tool: $1" >&2
    exit 1
  fi
}

require_tool wails
require_tool pkg-config
require_tool curl
require_tool patchelf   # linuxdeploy needs this

# Verify system dev libs are present. The runtime libs inside these packages
# are what we bundle into the AppImage.
MISSING_PKGS=()
for pc in webkit2gtk-4.0 gtk+-3.0 ayatana-appindicator3-0.1; do
  if ! pkg-config --exists "$pc"; then
    MISSING_PKGS+=("$pc")
  fi
done
if (( ${#MISSING_PKGS[@]} > 0 )); then
  echo "Missing pkg-config packages: ${MISSING_PKGS[*]}" >&2
  echo "Install them before building:" >&2
  echo "  Debian/Ubuntu: sudo apt install libwebkit2gtk-4.0-dev libgtk-3-dev libayatana-appindicator3-dev" >&2
  echo "  Fedora:        sudo dnf install webkit2gtk4.0-devel gtk3-devel libayatana-appindicator-gtk3-devel" >&2
  exit 1
fi

VERSION="$("$ROOT_DIR/scripts/version.sh")"
echo "Version: $VERSION"

mkdir -p "$BIN_DIR" "$CACHE_DIR"

# Fetch linuxdeploy + gtk plugin into the cache if missing.
LINUXDEPLOY="$CACHE_DIR/linuxdeploy-x86_64.AppImage"
LINUXDEPLOY_GTK="$CACHE_DIR/linuxdeploy-plugin-gtk.sh"

if [[ ! -x "$LINUXDEPLOY" ]]; then
  echo "Downloading linuxdeploy $LINUXDEPLOY_VERSION..."
  curl -fsSL -o "$LINUXDEPLOY" "$LINUXDEPLOY_URL"
  chmod +x "$LINUXDEPLOY"
fi
if [[ ! -x "$LINUXDEPLOY_GTK" ]]; then
  echo "Downloading linuxdeploy-plugin-gtk..."
  curl -fsSL -o "$LINUXDEPLOY_GTK" "$LINUXDEPLOY_GTK_URL"
  chmod +x "$LINUXDEPLOY_GTK"
fi

# Build the Wails binary. No webkit2_41 tag — we target WebKitGTK 4.0 for
# maximum distro compat.
cd "$ROOT_DIR"
wails build \
  -tags "linux_tray" \
  -ldflags "-X main.version=$VERSION"

BIN_SRC="$BIN_DIR/foghorn"
if [[ ! -f "$BIN_SRC" ]]; then
  echo "Expected wails output not found: $BIN_SRC" >&2
  exit 1
fi

# Assemble AppDir.
APPDIR="$(mktemp -d)/foghorn.AppDir"
STAGE_DIR="$(dirname "$APPDIR")"
trap 'rm -rf "$STAGE_DIR"' EXIT

mkdir -p "$APPDIR/usr/bin"
cp "$BIN_SRC" "$APPDIR/usr/bin/foghorn"
chmod +x "$APPDIR/usr/bin/foghorn"

cp "$ROOT_DIR/build/linux/foghorn.desktop" "$APPDIR/foghorn.desktop"
cp "$ROOT_DIR/build/appicon.png" "$APPDIR/foghorn.png"

# Locate shared libraries to bundle explicitly (the gtk plugin auto-detects GTK
# itself but won't pick these up on its own).
find_lib() {
  local pattern="$1"
  # Search common multiarch locations.
  for dir in /usr/lib/x86_64-linux-gnu /usr/lib64 /usr/lib; do
    local match
    match="$(find "$dir" -maxdepth 1 -name "$pattern" -print -quit 2>/dev/null || true)"
    if [[ -n "$match" ]]; then
      printf '%s\n' "$match"
      return 0
    fi
  done
  return 1
}

LIB_WEBKIT="$(find_lib 'libwebkit2gtk-4.0.so.*' | head -n1)"
LIB_JSCORE="$(find_lib 'libjavascriptcoregtk-4.0.so.*' | head -n1)"
LIB_AYATANA="$(find_lib 'libayatana-appindicator3.so.*' | head -n1)"

if [[ -z "$LIB_WEBKIT" || -z "$LIB_JSCORE" || -z "$LIB_AYATANA" ]]; then
  echo "Could not locate one of the runtime libraries to bundle:" >&2
  echo "  webkit2gtk-4.0: ${LIB_WEBKIT:-MISSING}" >&2
  echo "  javascriptcoregtk-4.0: ${LIB_JSCORE:-MISSING}" >&2
  echo "  ayatana-appindicator3: ${LIB_AYATANA:-MISSING}" >&2
  exit 1
fi

# Run linuxdeploy. Work from STAGE_DIR so the output .AppImage lands there.
cd "$STAGE_DIR"

DEPLOY_GTK_VERSION=3 \
"$LINUXDEPLOY" \
  --appdir "$APPDIR" \
  --plugin gtk \
  --library "$LIB_WEBKIT" \
  --library "$LIB_JSCORE" \
  --library "$LIB_AYATANA" \
  --output appimage

PRODUCED="$(find "$STAGE_DIR" -maxdepth 1 -name '*.AppImage' -print -quit)"
if [[ -z "$PRODUCED" || ! -f "$PRODUCED" ]]; then
  echo "linuxdeploy did not produce an AppImage." >&2
  exit 1
fi

OUT_PATH="$BIN_DIR/foghorn-${VERSION}-x86_64.AppImage"
rm -f "$OUT_PATH"
mv "$PRODUCED" "$OUT_PATH"
chmod +x "$OUT_PATH"

echo "$OUT_PATH"
