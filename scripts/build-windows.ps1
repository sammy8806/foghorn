#!/usr/bin/env pwsh
#
# Build a self-contained Foghorn NSIS installer for Windows amd64.
#
# Produces an installer that bundles the Foghorn binary and bootstraps the
# WebView2 runtime on the target machine (handled by the Wails NSIS template
# in build/windows/installer/project.nsi).
#
# Version resolution mirrors scripts/version.sh:
#   1. $env:FOGHORN_VERSION if set and non-empty (CI sets this from tag/dispatch)
#   2. `git describe --tags --always --dirty` when in a git checkout
#   3. Literal "dev"
#
# The version is injected into the binary via -ldflags "-X main.version=<version>"
# and used in the artifact filename.
#
# Final artifact: build/bin/foghorn-<version>-amd64-installer.exe

$ErrorActionPreference = 'Stop'

$RootDir = Split-Path -Parent $PSScriptRoot
Set-Location $RootDir

# Restrict to safe characters for use in filenames and -ldflags injection.
# Any character outside [0-9A-Za-z._-] is replaced with '-'.
function ConvertTo-SafeVersion {
    param([string]$Raw)
    $sanitized = $Raw -replace '[^0-9A-Za-z._-]', '-'
    if ($Raw -ne $sanitized) {
        Write-Warning "version '$Raw' contained unsafe characters; sanitized to '$sanitized'"
    }
    return $sanitized
}

function Resolve-Version {
    if ($env:FOGHORN_VERSION) {
        return (ConvertTo-SafeVersion $env:FOGHORN_VERSION)
    }
    $described = (git describe --tags --always --dirty 2>$null)
    if ($LASTEXITCODE -eq 0 -and $described) {
        # Strip a leading "v" so "v0.3.0" becomes "0.3.0".
        return (ConvertTo-SafeVersion ($described -replace '^v', ''))
    }
    return 'dev'
}

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    throw "Missing required tool: wails (install with 'go install github.com/wailsapp/wails/v2/cmd/wails@latest')"
}

# Wails shells out to makensis to build the NSIS installer. If it isn't on
# PATH, try the standard NSIS install locations and prepend the one we find so
# the build works without requiring a manual PATH edit.
if (-not (Get-Command makensis -ErrorAction SilentlyContinue)) {
    $candidates = @(
        (Join-Path $env:ProgramFiles 'NSIS')
        (Join-Path ${env:ProgramFiles(x86)} 'NSIS')
        (Join-Path $env:LOCALAPPDATA 'Programs\NSIS')
    )
    $nsisDir = $candidates | Where-Object { $_ -and (Test-Path (Join-Path $_ 'makensis.exe')) } | Select-Object -First 1
    if ($nsisDir) {
        Write-Host "Adding NSIS to PATH for this build: $nsisDir"
        $env:Path = "$nsisDir;$env:Path"
    } else {
        throw "Missing required tool: makensis (install NSIS with 'winget install NSIS.NSIS', or add its install dir to PATH)"
    }
}

$Version = Resolve-Version
Write-Host "Version: $Version"

wails build --platform windows/amd64 --nsis --ldflags "-X main.version=$Version"
if ($LASTEXITCODE -ne 0) {
    throw "wails build failed (exit $LASTEXITCODE)"
}

$BinDir = Join-Path $RootDir 'build\bin'
$Installer = Join-Path $BinDir 'foghorn-amd64-installer.exe'
if (-not (Test-Path $Installer)) {
    throw "Expected wails NSIS output not found: $Installer"
}

$OutPath = Join-Path $BinDir "foghorn-$Version-amd64-installer.exe"
Move-Item -Force -Path $Installer -Destination $OutPath
Write-Host $OutPath
