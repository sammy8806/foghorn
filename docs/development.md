# Development and source builds

[Back to README](../README.md)

Run commands from the repository root unless stated otherwise.

## Build from source

Foghorn is built with [Wails v2](https://wails.io/) (Go + Svelte). Build and release artifacts are written to `build/bin/`.

### Prerequisites

The release workflow uses Go 1.25, Node.js 20, and Wails 2.15. Install Go and Node.js, then the Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
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

## Development

Run in live-development mode with hot reload on macOS or Windows:

```bash
wails dev
```

On Linux with WebKitGTK 4.1 and AppIndicator support:

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
