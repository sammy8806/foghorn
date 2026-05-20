#!/usr/bin/env bash
#
# Emit a single version string to stdout.
#
# Resolution priority:
#   1. $FOGHORN_VERSION if set and non-empty (CI sets this from tag/dispatch)
#   2. `git describe --tags --always --dirty` when in a git checkout
#   3. Literal "dev" (tarball builds with no .git)
#
# The output is used both for -ldflags "-X main.version=<version>" injection
# and for artifact filenames (foghorn-<version>-universal.dmg, etc.).

set -euo pipefail

# Restrict to safe characters for use in filenames and -ldflags injection.
# Any character outside [0-9A-Za-z._-] is replaced with '-'.
sanitize_version() {
  local raw="$1"
  local sanitized="${raw//[^0-9A-Za-z._-]/-}"
  if [[ "$raw" != "$sanitized" ]]; then
    printf 'Warning: version %q contained unsafe characters; sanitized to %q\n' "$raw" "$sanitized" >&2
  fi
  printf '%s\n' "$sanitized"
}

if [[ -n "${FOGHORN_VERSION:-}" ]]; then
  sanitize_version "$FOGHORN_VERSION"
  exit 0
fi

if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  # Strip a leading "v" so "v0.3.0" becomes "0.3.0" — consistent with FOGHORN_VERSION.
  described="$(git describe --tags --always --dirty 2>/dev/null || true)"
  if [[ -n "$described" ]]; then
    sanitize_version "${described#v}"
    exit 0
  fi
fi

printf 'dev\n'
