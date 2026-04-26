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

if [[ -n "${FOGHORN_VERSION:-}" ]]; then
  printf '%s\n' "$FOGHORN_VERSION"
  exit 0
fi

if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  # Strip a leading "v" so "v0.3.0" becomes "0.3.0" — consistent with FOGHORN_VERSION.
  described="$(git describe --tags --always --dirty 2>/dev/null || true)"
  if [[ -n "$described" ]]; then
    printf '%s\n' "${described#v}"
    exit 0
  fi
fi

printf 'dev\n'
