#!/usr/bin/env bash
# Exercise the packaged runtime and render the UI on an X11 display.
# Run in a clean desktop container without GTK, WebKitGTK or AppIndicator.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: $0 <AppImage> [output-directory]" >&2
  exit 2
fi
APPIMAGE_PATH="$(realpath "$1")"
OUTPUT_DIR="$(realpath -m "${2:-build/appimage-test/smoke}")"
mkdir -p "$OUTPUT_DIR"

if [[ "${FOGHORN_SMOKE_SESSION:-}" != 1 ]]; then
  # Keep a parent shell: xvfb-run waits for a startup signal from Xvfb,
  # which cannot be delivered normally when xvfb-run itself is container PID 1.
  xvfb-run -a -s '-screen 0 1280x1024x24' \
    dbus-run-session -- env FOGHORN_SMOKE_SESSION=1 "$0" "$APPIMAGE_PATH" "$OUTPUT_DIR"
  exit 0
fi

# A space in the extraction path also checks AppRun/helper path quoting.
TEST_DIR="$(mktemp -d)/AppImage smoke"
mkdir -p "$TEST_DIR"
APP_PID=""
cleanup() {
  if [[ -n "$APP_PID" ]]; then
    kill "$APP_PID" 2>/dev/null || true
    wait "$APP_PID" 2>/dev/null || true
  fi
  rm -rf "$(dirname "$TEST_DIR")"
}
trap cleanup EXIT

export LC_ALL=C
export XDG_CONFIG_HOME="$TEST_DIR/config"
export XDG_CACHE_HOME="$TEST_DIR/cache"
mkdir -p "$XDG_CONFIG_HOME/foghorn" "$XDG_CACHE_HOME"
printf 'sources: []\n' > "$XDG_CONFIG_HOME/foghorn/config.yaml"

# These go through the AppImage runtime without libfuse2 or a FUSE mount.
APPIMAGE_EXTRACT_AND_RUN=1 "$APPIMAGE_PATH" --version
(cd "$TEST_DIR" && "$APPIMAGE_PATH" --appimage-extract >/dev/null)
APPDIR_PATH="$TEST_DIR/squashfs-root"
for binary in "$APPDIR_PATH/usr/bin/foghorn" "$APPDIR_PATH/usr/libexec/webkit2gtk/"WebKit*Process; do
  [[ -x "$binary" ]] || { echo "Missing executable: $binary" >&2; exit 1; }
  dependencies="$(LD_LIBRARY_PATH="$APPDIR_PATH/usr/lib" ldd "$binary")"
  if [[ "$dependencies" == *'not found'* ]]; then
    printf '%s\n%s\n' "$binary" "$dependencies" >&2
    exit 1
  fi
done

timeout --kill-after=5s 60s "$APPDIR_PATH/AppRun" > "$OUTPUT_DIR/app.log" 2>&1 &
APP_PID=$!
# Wait for actual web content, not just a GTK window or a surviving process.
for ((attempt=0; attempt<30; attempt++)); do
  if ! kill -0 "$APP_PID" 2>/dev/null; then
    cat "$OUTPUT_DIR/app.log" >&2
    echo "AppImage exited before rendering its window" >&2
    exit 1
  fi
  WINDOW_ID="$(xdotool search --onlyvisible --name '^Foghorn$' 2>/dev/null | head -n1 || true)"
  if [[ -n "$WINDOW_ID" ]]; then
    xdotool windowmove "$WINDOW_ID" 0 0
    xdotool windowsize "$WINDOW_ID" 800 600
    import -window "$WINDOW_ID" "$OUTPUT_DIR/window.png"
    convert "$OUTPUT_DIR/window.png" -crop 800x100+0+0 -resize 300% -negate -colorspace gray "$OUTPUT_DIR/ocr.png"
    tesseract "$OUTPUT_DIR/ocr.png" "$OUTPUT_DIR/window" --psm 11 2>/dev/null
    if grep -Eiq 'Severity' "$OUTPUT_DIR/window.txt" && grep -Eiq 'alerts' "$OUTPUT_DIR/window.txt"; then
      # Stay alive past the catalog's 11-second early-exit threshold.
      sleep 12
      kill -0 "$APP_PID"
      echo "AppImage rendered its UI and remained running. Screenshot: $OUTPUT_DIR/window.png"
      exit 0
    fi
  fi
  sleep 1
done
cat "$OUTPUT_DIR/app.log" >&2
cat "$OUTPUT_DIR/window.txt" >&2 2>/dev/null || true
echo "AppImage did not render the Foghorn UI within 30 seconds" >&2
exit 1
