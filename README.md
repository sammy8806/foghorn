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

## Live Development

To run in live development mode, use:

```bash
wails dev -tags webkit2_41
```

This runs a Vite development server with hot reload for frontend changes. If you
want to develop in a browser and have access to your Go methods, there is also a
dev server that runs on `http://localhost:34115`.

## Building

To build a redistributable production package, use:

```bash
wails build -tags webkit2_41
```

If `dnf` prompts about an unrelated third-party repository GPG key during
dependency installation, resolve that repo configuration first or temporarily
disable that repo for the install command.
