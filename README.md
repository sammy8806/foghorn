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
- **Filter and group** — severity, source, and free-text filters; configurable grouping and sorting.
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

Windows is theoretically supported via the standard Wails build:

```powershell
wails build
```

The Windows executable is written to `build\bin\foghorn.exe`. Windows builds are not regularly tested.

### Release builds

Local release artifacts use `scripts/version.sh` for their version string. It prefers `FOGHORN_VERSION`, then `git describe --tags --always --dirty`, then `dev`.

Maintainers can use the release workflow to build the macOS DMG and Linux AppImage from tags or manual dispatch.
```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
$env:Path = (go env GOPATH) + ";$env:Path"
wails build
```

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

## Development

Run in live-development mode with hot reload:

```bash
wails dev -tags "webkit2_41 linux_tray"
```

Drop the `linux_tray` tag if you don't want tray support while developing. If you prefer to develop the UI in a browser with access to Go methods, Wails exposes a dev server at `http://localhost:34115`.

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
