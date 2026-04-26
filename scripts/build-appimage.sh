#!/usr/bin/env bash
#
# Build a self-contained Foghorn AppImage for Linux x86_64.
#
# Bundles WebKitGTK (4.0 or 4.1, whichever is present), GTK3, and
# AppIndicator3 so the AppImage runs on any host with a glibc >= the build
# host's glibc version.
#
# CAVEATS
# -------
# 1. glibc ABI floor: the AppImage's minimum glibc version equals the build
#    host's glibc version. Build on the oldest supported distro (e.g. Ubuntu
#    22.04 / glibc 2.35) to maximise portability.  Fedora ships a newer glibc
#    so AppImages built here won't run on older distros.
#
# 2. WebKitGTK API version:
#    - webkit2gtk-4.0: shipped by Ubuntu ≤ 23.10, Debian 12, RHEL 9.
#    - webkit2gtk-4.1: shipped by Ubuntu 24.04+, Fedora 37+.
#    This script auto-detects which variant is present and selects the correct
#    Wails build tag (webkit2_41).  The resulting AppImage requires the same
#    ABI on the target host; you cannot mix API versions.
#
# 3. Sandbox / SUID: WebKit's process sandbox may require a SUID helper.
#    If the app opens but pages are blank, run with --no-sandbox or install
#    the appropriate webkit helper on the target system.
#
# 4. Wayland: the bundle uses the X11/XWayland GTK backend. Native Wayland
#    support requires additional environment variables (GDK_BACKEND=wayland)
#    and may have input-method limitations.
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
  local cmd="$1"
  local hint="${2:-}"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required tool: $cmd" >&2
    if [[ -n "$hint" ]]; then
      echo "  $hint" >&2
    fi
    exit 1
  fi
}

require_tool wails
require_tool pkg-config
require_tool curl
require_tool patchelf \
  "Install with: sudo dnf install patchelf  (Fedora) | sudo apt install patchelf  (Debian/Ubuntu)"

# ── WebKitGTK version detection ───────────────────────────────────────────────
# Prefer 4.0 for broadest target-host compat; fall back to 4.1 (Fedora 37+,
# Ubuntu 24.04+).  Wails requires the 'webkit2_41' build tag with the 4.1 API.

WEBKIT_PC=""
WEBKIT_VER=""
WEBKIT_BUILD_TAG=""

if pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
  WEBKIT_PC="webkit2gtk-4.0"
  WEBKIT_VER="4.0"
elif pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
  WEBKIT_PC="webkit2gtk-4.1"
  WEBKIT_VER="4.1"
  WEBKIT_BUILD_TAG="webkit2_41"
else
  echo "Neither webkit2gtk-4.0 nor webkit2gtk-4.1 found via pkg-config." >&2
  echo "Install one of:" >&2
  echo "  Debian/Ubuntu 22.04: sudo apt install libwebkit2gtk-4.0-dev" >&2
  echo "  Debian/Ubuntu 24.04: sudo apt install libwebkit2gtk-4.1-dev" >&2
  echo "  Fedora:              sudo dnf install webkit2gtk4.1-devel" >&2
  exit 1
fi

echo "WebKitGTK: $WEBKIT_VER (pkg-config: $WEBKIT_PC)"

# Verify remaining required libs.
MISSING_PKGS=()
for pc in gtk+-3.0 ayatana-appindicator3-0.1; do
  if ! pkg-config --exists "$pc" 2>/dev/null; then
    MISSING_PKGS+=("$pc")
  fi
done
if (( ${#MISSING_PKGS[@]} > 0 )); then
  echo "Missing pkg-config packages: ${MISSING_PKGS[*]}" >&2
  echo "Install them before building:" >&2
  echo "  Debian/Ubuntu: sudo apt install libgtk-3-dev libayatana-appindicator3-dev" >&2
  echo "  Fedora:        sudo dnf install gtk3-devel libayatana-appindicator-gtk3-devel" >&2
  exit 1
fi

VERSION="$("$ROOT_DIR/scripts/version.sh")"
echo "Version: $VERSION"

mkdir -p "$BIN_DIR" "$CACHE_DIR"

# ── Fetch linuxdeploy + gtk plugin ────────────────────────────────────────────
LINUXDEPLOY_APPIMAGE="$CACHE_DIR/linuxdeploy-x86_64.AppImage"
LINUXDEPLOY_EXTRACTED="$CACHE_DIR/linuxdeploy-extracted"
LINUXDEPLOY_GTK="$CACHE_DIR/linuxdeploy-plugin-gtk.sh"

if [[ ! -x "$LINUXDEPLOY_APPIMAGE" ]]; then
  echo "Downloading linuxdeploy $LINUXDEPLOY_VERSION..."
  curl -fsSL -o "$LINUXDEPLOY_APPIMAGE" "$LINUXDEPLOY_URL"
  chmod +x "$LINUXDEPLOY_APPIMAGE"
  rm -rf "$LINUXDEPLOY_EXTRACTED"  # force re-extract when AppImage changes
fi
if [[ ! -x "$LINUXDEPLOY_GTK" ]]; then
  echo "Downloading linuxdeploy-plugin-gtk..."
  curl -fsSL -o "$LINUXDEPLOY_GTK" "$LINUXDEPLOY_GTK_URL"
  chmod +x "$LINUXDEPLOY_GTK"
fi

# Extract linuxdeploy to a known cache location and patch its bundled strip.
# The bundled strip is too old to handle .relr.dyn ELF sections (type 0x13)
# present in glibc 2.40+ libraries (Fedora 40+, Ubuntu 24.10+).  Running the
# AppImage via APPIMAGE_EXTRACT_AND_RUN extracts to a hash-named temp dir that
# gets recreated on each boot; using our own extracted copy lets us patch it
# persistently.  --appimage-extract works without FUSE.
if [[ ! -d "$LINUXDEPLOY_EXTRACTED" ]]; then
  echo "Extracting linuxdeploy..."
  pushd "$CACHE_DIR" >/dev/null
  "$LINUXDEPLOY_APPIMAGE" --appimage-extract >/dev/null
  mv squashfs-root linuxdeploy-extracted
  popd >/dev/null
fi
# Replace the bundled strip and patchelf with system versions.
# linuxdeploy 2024-01-09 bundles strip and patchelf that are too old to handle
# .relr.dyn ELF sections (type 0x13) used by glibc 2.40+ distros.  The old
# strip refuses to process these libraries; the old patchelf silently corrupts
# them when rewriting RPATH, causing constructor crashes at runtime.
ln -sf "$(command -v strip)"    "$LINUXDEPLOY_EXTRACTED/usr/bin/strip"
ln -sf "$(command -v patchelf)" "$LINUXDEPLOY_EXTRACTED/usr/bin/patchelf"
LINUXDEPLOY="$LINUXDEPLOY_EXTRACTED/AppRun"

# ── Wails build ───────────────────────────────────────────────────────────────
# Build tags: linux_tray always; webkit2_41 when targeting WebKitGTK 4.1.
BUILD_TAGS="linux_tray"
if [[ -n "$WEBKIT_BUILD_TAG" ]]; then
  BUILD_TAGS="$BUILD_TAGS $WEBKIT_BUILD_TAG"
fi

cd "$ROOT_DIR"
wails build \
  -tags "$BUILD_TAGS" \
  -ldflags "-X main.version=$VERSION"

BIN_SRC="$BIN_DIR/foghorn"
if [[ ! -f "$BIN_SRC" ]]; then
  echo "Expected wails output not found: $BIN_SRC" >&2
  exit 1
fi

# ── Assemble AppDir ───────────────────────────────────────────────────────────
# Use FOGHORN_APPDIR rather than APPDIR: the linuxdeploy extracted AppRun binary
# injects APPDIR pointing at its own extraction directory, which would collide
# with a variable named APPDIR in this script.
FOGHORN_APPDIR="$(mktemp -d)/foghorn.AppDir"
STAGE_DIR="$(dirname "$FOGHORN_APPDIR")"
trap 'rm -rf "$STAGE_DIR"' EXIT

mkdir -p "$FOGHORN_APPDIR/usr/bin"
cp "$BIN_SRC" "$FOGHORN_APPDIR/usr/bin/foghorn"
chmod +x "$FOGHORN_APPDIR/usr/bin/foghorn"

cp "$ROOT_DIR/build/linux/foghorn.desktop" "$FOGHORN_APPDIR/foghorn.desktop"
cp "$ROOT_DIR/build/appicon.png" "$FOGHORN_APPDIR/foghorn.png"

# Write a custom AppRun.  linuxdeploy should create this automatically, but
# when run from an extracted AppRun the APPDIR env var injected by that runtime
# can interfere with its desktop-file search, causing the step to be silently
# skipped.  Providing it explicitly avoids the issue entirely.
cat > "$FOGHORN_APPDIR/AppRun" <<'EOF'
#!/usr/bin/env bash
HERE="$(dirname "$(readlink -f "$0")")"

# Source apprun-hooks (e.g. GTK theme/icon setup from linuxdeploy-plugin-gtk).
if [[ -d "$HERE/apprun-hooks" ]]; then
  for _hook in "$HERE/apprun-hooks"/*.sh; do
    # shellcheck source=/dev/null
    source "$_hook"
  done
fi

export LD_LIBRARY_PATH="$HERE/usr/lib:${LD_LIBRARY_PATH:-}"
exec "$HERE/usr/bin/foghorn" "$@"
EOF
chmod +x "$FOGHORN_APPDIR/AppRun"

# ── Locate runtime libraries to bundle explicitly ─────────────────────────────
# The gtk plugin auto-detects GTK itself but won't pick these up on its own.
find_lib() {
  local pattern="$1"
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

LIB_WEBKIT="$(find_lib "libwebkit2gtk-${WEBKIT_VER}.so.*" | head -n1)"
LIB_JSCORE="$(find_lib "libjavascriptcoregtk-${WEBKIT_VER}.so.*" | head -n1)"
LIB_AYATANA="$(find_lib 'libayatana-appindicator3.so.*' | head -n1)"

if [[ -z "$LIB_WEBKIT" || -z "$LIB_JSCORE" || -z "$LIB_AYATANA" ]]; then
  echo "Could not locate one or more runtime libraries to bundle:" >&2
  echo "  libwebkit2gtk-${WEBKIT_VER}: ${LIB_WEBKIT:-MISSING}" >&2
  echo "  libjavascriptcoregtk-${WEBKIT_VER}: ${LIB_JSCORE:-MISSING}" >&2
  echo "  libayatana-appindicator3: ${LIB_AYATANA:-MISSING}" >&2
  exit 1
fi

# ── Run linuxdeploy ───────────────────────────────────────────────────────────
# Add CACHE_DIR to PATH so linuxdeploy can find linuxdeploy-plugin-gtk.sh.
# When running from an extracted AppRun, plugin discovery searches PATH; the
# original AppImage approach found plugins in its own directory automatically.
cd "$STAGE_DIR"

PATH="$CACHE_DIR:$PATH" \
DEPLOY_GTK_VERSION=3 \
"$LINUXDEPLOY" \
  --appdir "$FOGHORN_APPDIR" \
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
