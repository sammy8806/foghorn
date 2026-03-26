# README

## About

Foghorn is a desktop app built with Wails v2, Go, and Svelte.

## Install Wails

Wails requires Go and npm/node to be installed first.

Install the Wails CLI with:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify the installation with:

```bash
wails doctor
```

## Linux Prerequisites

Wails on Linux requires native GTK and WebKitGTK development packages.

On Fedora 43, install:

```bash
sudo dnf install gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel
```

Fedora 43 ships WebKitGTK 4.1, so Wails must be built with the `webkit2_41`
build tag.

Optional tray support on Linux uses AppIndicator via
`github.com/getlantern/systray`. On Fedora, install:

```bash
sudo dnf install libayatana-appindicator-gtk3-devel
```

Then build or run with the additional `linux_tray` build tag.

On GNOME, tray visibility still depends on the desktop environment exposing
AppIndicator or StatusNotifier items. If the app is built without
`linux_tray`, or if tray support is not available at runtime, Foghorn falls
back to opening a normal visible window so it remains usable without a tray.
Even with `linux_tray` enabled, Linux starts with a visible window by default;
the tray is treated as an optional convenience, not the primary entry point.

## Live Development

To run in live development mode, use:

```bash
wails dev -tags "webkit2_41 linux_tray"
```

This runs a Vite development server with hot reload for frontend changes. If you
want to develop in a browser and have access to your Go methods, there is also a
dev server that runs on `http://localhost:34115`.

If you do not want Linux tray support, omit the `linux_tray` tag and the app
will start with a normal visible window instead.

## Building

To build on macOS, use `make build` (or `make build-macos`).
This wraps `wails build` and then re-signs the `.app` bundle with the bundle identifier from `Info.plist`, which is required for macOS notifications to work reliably.

On non-macOS platforms, `make build` falls back to `wails build`.
To build a redistributable production package, use:

```bash
wails build -tags "webkit2_41 linux_tray"
```

If `dnf` prompts about an unrelated third-party repository GPG key during
dependency installation, resolve that repo configuration first or temporarily
disable that repo for the install command.
