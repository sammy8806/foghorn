# Real Developer ID signing & notarization (local + CI)

**Date:** 2026-06-04
**Status:** Approved (pending spec review)

## Problem

Foghorn's macOS build scripts already implement a full Developer ID signing +
notarization path (`scripts/build-dmg.sh` → `scripts/build-macos-app.sh`), and
the release workflow already calls `build-dmg.sh` on `v*` tags. But signing has
never run for real: locally there is only an **Apple Development** certificate,
not the **Developer ID Application** certificate the scripts require, so the
scripts silently fall back to ad-hoc signing.

Goal: produce genuinely signed + notarized DMGs, using the **same script**
locally and in CI (tagged builds), and modernize notarization auth.

## Decisions

- **Notarization auth:** switch from the Apple-ID + app-specific-password +
  team-id triple to an **App Store Connect API key** (Issuer ID + Key ID +
  `.p8`). More robust, no password rotation, team-scoped.
- **CSR generation:** Keychain Access (Certificate Assistant). One-time; private
  key auto-lands in the login keychain.
- **Local secrets:** gitignored `scripts/.env.signing`, sourced before running
  the script. Mirrors CI env vars exactly.
- **Orphaned `scripts/build-macos-dmg.sh`:** leave as-is (out of scope).
- **Entitlements:** none added unless the notarization log reports a rejection
  (YAGNI).

## Certificate creation (one-time, manual)

The CSR is a throwaway whose real purpose is generating the private key. The
exported `.p12` is the only signing artifact that travels to CI.

```
CSR → Apple issues .cer → install .cer → export (key + .cer) as .p12 → base64
```

1. Keychain Access → Certificate Assistant → *Request a Certificate from a
   Certificate Authority* → save CSR to disk (generates the private key in the
   login keychain).
2. developer.apple.com → Certificates → **+** → **Developer ID Application** →
   upload CSR → download `.cer`.
3. Double-click `.cer` to install into the login keychain.
4. Keychain Access → right-click the cert → **Export** as `.p12` (set a
   password). This bundles the private key + certificate.
5. `base64 -i developerID.p12 | pbcopy` → `APPLE_DEVELOPER_ID_CERT_P12_BASE64`.

## App Store Connect API key (one-time, manual)

developer.apple.com → Users and Access → Integrations → App Store Connect API →
create a key with **Developer** access. Capture:

- **Issuer ID** → `APPLE_API_ISSUER_ID`
- **Key ID** → `APPLE_API_KEY_ID`
- One-time download `AuthKey_XXXX.p8` → `base64 -i AuthKey_XXXX.p8 | pbcopy` →
  `APPLE_API_KEY_P8_BASE64`

## Secret set (replaces the old 5)

| Variable | Purpose |
|---|---|
| `APPLE_DEVELOPER_ID_CERT_P12_BASE64` | signing cert + key (base64 `.p12`) |
| `APPLE_DEVELOPER_ID_CERT_PASSWORD` | `.p12` export password |
| `APPLE_API_KEY_ID` | notarization key id |
| `APPLE_API_ISSUER_ID` | notarization issuer id |
| `APPLE_API_KEY_P8_BASE64` | notarization key (base64 `.p8`) |

**Dropped:** `APPLE_ID`, `APPLE_APP_SPECIFIC_PASSWORD`, `APPLE_TEAM_ID` — the API
key is team-scoped and `codesign` reads the team from the cert.

## Code changes

### `scripts/build-macos-app.sh`
- Update `DEV_ID_VARS` to the new 5-var set (drop the three Apple-ID vars).
- The signing logic (keychain import, find "Developer ID Application" identity,
  `codesign --options runtime --timestamp`) is unchanged.

### `scripts/build-dmg.sh`
- Update `DEV_ID_VARS` to the new 5-var set (keep in sync with app script).
- In the dev-id block, decode `APPLE_API_KEY_P8_BASE64` to a temp `.p8` inside
  the existing `mktemp` staging dir; the existing cleanup trap removes it.
- Replace the `notarytool submit` call:
  ```bash
  xcrun notarytool submit "$DMG_PATH" \
    --key "$API_KEY_P8_PATH" \
    --key-id "$APPLE_API_KEY_ID" \
    --issuer "$APPLE_API_ISSUER_ID" \
    --wait
  ```
- `stapler staple` / `stapler validate` unchanged.

### `.github/workflows/release.yml`
- Replace the `build-macos` job `env:` block with the new 5 secret names.

### `scripts/.env.signing.example` (new, committed)
- Documents all 5 vars with placeholder values and the `base64 -i … | pbcopy`
  hints.

### `.gitignore`
- Add `scripts/.env.signing`.

## Local workflow

```bash
set -a; source scripts/.env.signing; set +a
./scripts/build-dmg.sh
```

## Verification

After a local dev-id build:

- `codesign --verify --deep --strict --verbose=2 build/bin/foghorn.app`
- `xcrun stapler validate build/bin/foghorn-*-universal.dmg`
- `spctl -a -t open --context context:primary-signature -v build/bin/foghorn-*-universal.dmg`
  → expect `accepted` / `source=Notarized Developer ID`
- If notarization is rejected, read the log via
  `xcrun notarytool log <submission-id> --key … --key-id … --issuer …` and add
  an entitlements file only if the log requires it.

CI: push a `v*` tag (or run the workflow_dispatch) and confirm the released DMG
passes the same `spctl` check on a clean machine.

## Out of scope

- Linux AppImage signing.
- `scripts/build-macos-dmg.sh` cleanup.
- Custom entitlements (added reactively only).
