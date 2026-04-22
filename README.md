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

## Install

Foghorn is built with [Wails v2](https://wails.io/) (Go + Svelte). There are no prebuilt binaries yet — build from source.

### Prerequisites

Install Go, Node.js, and the Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
wails doctor
```

### macOS

```bash
make build
```

This wraps `wails build` and re-signs the `.app` bundle with the bundle identifier from `Info.plist`, which is required for macOS notifications to work reliably. The built app lands in `build/bin/`.

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

On GNOME, tray visibility still depends on the desktop environment exposing AppIndicator or StatusNotifier items. If Foghorn is built without `linux_tray`, or tray support is unavailable at runtime, it falls back to a normal visible window so it remains usable. Even with `linux_tray` enabled, Linux starts with a visible window by default — the tray is an optional convenience, not the primary entry point.

If `dnf` prompts about an unrelated third-party repository GPG key during prerequisite installation, resolve that repo configuration first or temporarily disable that repo for the install command.

### Windows

Windows is theoretically supported via the standard `wails build`, but is not regularly tested. Issues and patches welcome.

## Configuration

Foghorn reads YAML configuration. A fully annotated example lives in [`config.example.yaml`](config.example.yaml) — copy it and edit to your environment. A minimal source configuration looks like:

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
