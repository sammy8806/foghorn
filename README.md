<p align="center">
  <img src="build/appicon.png" alt="Foghorn" width="96" height="96">
</p>

<h1 align="center">Foghorn</h1>

<p align="center">
  A desktop alert monitor for Alertmanager, Grafana Alerting, Prometheus, and Better Stack.
</p>

<!-- TODO: add a screenshot here, e.g. docs/screenshot.png -->

## Features

- **Multiple alert sources** — poll Alertmanager, Grafana Alerting, Prometheus, and Better Stack side by side.
- **Silences** — view, create, edit, and expire Alertmanager silences directly from the app.
- **On-call at a glance** — show the current Better Stack on-call person in the status bar, with direct links into incidents.
- **Filter and group** — severity, source, and a query-syntax search bar (free text, negation, `key=value`/`key=~regex` label matchers); configurable grouping and sorting.
- **Desktop notifications** — native notifications on macOS, Linux, and Windows.
- **Optional system tray** — opt-in tray support on Linux (AppIndicator); always-on on macOS and Windows.
- **Configurable** — YAML configuration with `${ENV_VAR}` interpolation for secrets.

## Build and install

Foghorn is built with [Wails v2](https://wails.io/) (Go + Svelte). Build and release artifacts are written to `build/bin/`.

### Prerequisites

Install Go, Node.js, and the Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
wails doctor
```

### macOS app bundle

```bash
make build
```

This wraps `wails build` and re-signs the `.app` bundle with the bundle identifier from `Info.plist`, which is required for macOS notifications to work reliably. The built app lands in `build/bin/`.

### macOS DMG

To build the app and package it as a DMG:

```bash
make dmg
```

The DMG is written to `build/bin/foghorn-<version>-universal.dmg`. It contains `foghorn.app` and an `Applications` shortcut, so users can install it with the usual drag-and-drop flow.

If `build/bin/foghorn.app` already exists and you only want to create a quick local DMG without rebuilding the app:

```bash
make dmg-existing
```

That helper writes `build/dist/foghorn.dmg`.

### Linux

Wails on Linux needs native GTK and WebKitGTK development packages. On Fedora 43:

```bash
sudo dnf install gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel
```

Fedora 43 ships WebKitGTK 4.1, so Foghorn must be built with the `webkit2_41` build tag:

```bash
wails build -tags "webkit2_41"
```

**Optional tray support** uses AppIndicator via [`getlantern/systray`](https://github.com/getlantern/systray). To enable it:

```bash
sudo dnf install libayatana-appindicator-gtk3-devel
wails build -tags "webkit2_41 linux_tray"
```

The Linux binary is written to `build/bin/foghorn`.

To build an AppImage on Linux x86_64:

```bash
make appimage
```

The AppImage is written to `build/bin/foghorn-<version>-x86_64.AppImage`.

The Linux AppImage intentionally uses the host system's GTK/WebKitGTK runtime
instead of bundling WebKitGTK. WebKitGTK includes helper processes plus graphics
and media stacks that must match the host desktop closely; bundling Ubuntu's
WebKitGTK stack caused renderer crashes on Fedora. This means the AppImage is
small and more reliable across desktop environments, but the target system must
provide the GTK/WebKitGTK runtime packages.

Runtime packages for Fedora/KDE/GNOME:

```bash
sudo dnf install webkit2gtk4.1 gtk3 libayatana-appindicator-gtk3
```

Runtime packages for Ubuntu 24.04/Kubuntu:

```bash
sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 libayatana-appindicator3-1
```

KDE users still need these GTK/WebKitGTK packages because Wails uses WebKitGTK
for the embedded web view on Linux. KDE itself is supported; it just does not
replace the GTK/WebKitGTK runtime requirement.

On GNOME, tray visibility still depends on the desktop environment exposing AppIndicator or StatusNotifier items. If Foghorn is built without `linux_tray`, or tray support is unavailable at runtime, it falls back to a normal visible window so it remains usable. Even with `linux_tray` enabled, Linux starts with a visible window by default — the tray is an optional convenience, not the primary entry point.

If `dnf` prompts about an unrelated third-party repository GPG key during prerequisite installation, resolve that repo configuration first or temporarily disable that repo for the install command.

### Windows

For a bare executable, use the standard Wails build:

```powershell
wails build
```

The Windows executable is written to `build\bin\foghorn.exe`.

To build a distributable NSIS installer, you need [NSIS](https://nsis.sourceforge.io/) on your `PATH` in addition to the standard Wails prerequisites (Go, Node.js/npm, the Wails CLI, and the WebView2 runtime — run `wails doctor` to check). NSIS installs to `C:\Program Files (x86)\NSIS`, which neither installer below adds to `PATH` automatically:

```powershell
# Install NSIS (pick one)
winget install NSIS.NSIS     # winget, built into Windows 11
choco install nsis -y        # or chocolatey

# Add it to PATH for your user (open a fresh terminal afterwards)
[Environment]::SetEnvironmentVariable(
  "Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";C:\Program Files (x86)\NSIS",
  "User")
```

Then build the installer:

```powershell
./scripts/build-windows.ps1
```

The installer is written to `build\bin\foghorn-<version>-amd64-installer.exe`. It bundles the binary and bootstraps the WebView2 runtime on the target machine. The script resolves its version the same way as `scripts/version.sh` (prefers `FOGHORN_VERSION`, then `git describe`, then `dev`).

### Release builds

Local release artifacts use `scripts/version.sh` (or, on Windows, `scripts/build-windows.ps1`) for their version string. They prefer `FOGHORN_VERSION`, then `git describe --tags --always --dirty`, then `dev`.

Maintainers can use the release workflow to build the macOS DMG, Linux AppImage, and Windows installer from tags or manual dispatch. Pushing a `v*` tag (or running the workflow manually) builds all three and publishes them to a GitHub release.

Windows builds include the native system tray. Closing the window hides it; use the tray menu to show or hide the window, or to quit Foghorn.

## Configuration

Foghorn reads YAML configuration from a platform-specific location:

| Platform | Path |
|----------|------|
| macOS    | `~/Library/Application Support/foghorn/config.yaml` |
| Linux    | `~/.config/foghorn/config.yaml` (or `$XDG_CONFIG_HOME/foghorn/config.yaml`) |
| Windows  | `%APPDATA%\foghorn\config.yaml` |

A fully annotated example lives in [`config.example.yaml`](config.example.yaml) — copy it to the right location for your platform and edit to your environment:

```bash
# macOS
mkdir -p ~/Library/Application\ Support/foghorn
cp config.example.yaml ~/Library/Application\ Support/foghorn/config.yaml

# Linux
mkdir -p ~/.config/foghorn
cp config.example.yaml ~/.config/foghorn/config.yaml
```

```powershell
# Windows
New-Item -ItemType Directory -Force "$env:APPDATA\foghorn"
Copy-Item config.example.yaml "$env:APPDATA\foghorn\config.yaml"
```

A minimal source configuration looks like:

```yaml
sources:
  - name: local-alertmanager
    type: alertmanager
    url: http://localhost:9093
    auth:
      type: basic
      username: ${FOGHORN_AM_USER}
      password: ${FOGHORN_AM_PASS}
    poll_interval: 30s
```

`${ENV_VAR}` references are expanded at load time, so secrets can live in your shell environment or a secrets manager rather than the config file.

See `config.example.yaml` for the full reference, including severity mapping, display/grouping options, notification rules, and Better Stack-specific fields.

### Command line

Foghorn can report its version and manage saved cookie and OIDC logins without
opening the desktop application:

```bash
foghorn --version
foghorn -v
foghorn auth list
foghorn auth clear my-alertmanager
foghorn auth clear --all
```

`auth list` reports whether each supported source has a saved login and where
it is stored; it never prints cookies or token values. `auth clear` removes the
saved login so the source prompts for authentication again. OIDC credentials
use macOS Keychain, Linux Secret Service, or Windows Credential Manager. These
commands do not modify passwords or API tokens in the configuration.

For Alertmanager login through the OIDC device flow, including Keycloak client
and group mapper settings, secure credential-store-backed persistent login,
local logout, reverse-proxy bearer passthrough, and `302`/`403` diagnostics, see
[`docs/oidc-device-auth.md`](docs/oidc-device-auth.md).

The `ui.scale` block can enlarge the app for accessibility. Use `mode: fonts` to scale text only, or `mode: interface` to scale the full web UI. `factor` accepts `0.75` through `2.0`; when `mode: interface`, `apply_to_popup: true` also resizes the popup window.

For how the **Sort** selector's presets work — what each one orders by, where `startsAt`/`updatedAt` come from, and the provider-specific caveats — see [`docs/sorting.md`](docs/sorting.md).

For the search bar's query syntax — field matchers, negation, regex anchoring, and how a query becomes a silence — see [`docs/search.md`](docs/search.md).

## Development

Run in live-development mode with hot reload:

```bash
wails dev -tags "webkit2_41 linux_tray"
```

Drop the `linux_tray` tag if you don't want tray support while developing. If you prefer to develop the UI in a browser with access to Go methods, Wails exposes a dev server at `http://localhost:34115`.

## Troubleshooting

### A silence doesn't appear after creating it

Provider HTTP traffic happens in the Go backend, so the browser/Wails devtools
network panel can't see calls to Alertmanager. Use the built-in HTTP debug
logging instead.

Start Foghorn with `FOGHORN_HTTP_DEBUG=1` (also accepts `true`/`yes`/`on`) and
watch the logs:

```bash
# Dev mode — logs stream to the terminal:
FOGHORN_HTTP_DEBUG=1 wails dev -tags "webkit2_41 linux_tray"

# Built binary — launch from a terminal so stderr is visible:
FOGHORN_HTTP_DEBUG=1 ./build/bin/foghorn.app/Contents/MacOS/foghorn 2>&1 | tee /tmp/foghorn-silence.log
```

> On macOS a double-clicked `.app` hides stderr. Either launch it from a
> terminal as above, or open **Console.app** and filter by process `foghorn`.

Now reproduce the silence and read the three log lines it produces:

```
app: CreateSilence source="prod" matchers=4 duration="2h" -> id="..." err=<nil>
http: POST https://alertmanager.example.com/api/v2/silences -> 200 (12ms)
silence: alertmanager prod response HTTP 200 body={"silenceID":"..."}
```

Read them top-down to find the failing layer:

| Symptom in logs | Meaning |
|---|---|
| No `app: CreateSilence` line | The frontend never called the backend — a UI-side problem (e.g. the dialog's submit was disabled). |
| `app: CreateSilence … err=<set>` | The create failed and the error is being surfaced; the `http:` and `silence:` lines show the status, body, and any redirect. |
| `http: … -> 30x … Location=…` | A proxy/ingress is redirecting the request. Foghorn does **not** follow redirects on silence writes (a followed redirect downgrades `POST` to `GET` and silently drops the body), so this is reported as an error rather than a phantom success. Point the source `url` at the address the proxy redirects *to*. |
| `app: CreateSilence … id="<real-id>" err=<nil>` | Alertmanager accepted the silence. If it still isn't shown in the UI, it likely isn't in the `active` state yet (clock skew can make a fresh silence `pending`) or it landed on a different replica in an HA Alertmanager cluster. |

Raw credentials are never logged: the `Authorization` header, encoded OIDC
tokens, and any URL userinfo are redacted. For OIDC troubleshooting, debug mode
does log selected non-identity authorization claims such as groups, roles,
scope, audience, issuer, and client ID. `FOGHORN_HTTP_DEBUG` covers all
providers (Alertmanager, Grafana, Prometheus, Better Stack); the
`app: CreateSilence`/`UpdateSilence` boundary lines are always logged regardless
of the flag.

## Project layout

```
.
├── main.go, app.go        # Wails entry point and app lifecycle
├── internal/              # Go backend: alert sources, notifications, tray, config
├── frontend/              # Svelte frontend (Vite)
├── scripts/               # Build helpers (e.g. macOS re-signing)
├── build/                 # Wails build artifacts and platform assets
└── config.example.yaml    # Annotated configuration reference
```

## License

TBD.
