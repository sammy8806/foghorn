# Configuration

[Back to README](../README.md)

## Configuration

Foghorn reads YAML configuration from a platform-specific location:

| Platform | Path |
|----------|------|
| macOS    | `~/Library/Application Support/foghorn/config.yaml` |
| Linux    | `~/.config/foghorn/config.yaml` (or `$XDG_CONFIG_HOME/foghorn/config.yaml`) |
| Windows  | `%APPDATA%\foghorn\config.yaml` |

A fully annotated example lives in [`config.example.yaml`](../config.example.yaml) — copy it to the right location for your platform and edit to your environment:

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

For a local Alertmanager without authentication, omit the `auth` block. Source URLs must be reachable from the machine running Foghorn.

Add other providers to the same `sources` list as needed:

```yaml
sources:
  - name: grafana
    type: grafana
    url: https://grafana.example.com
    auth:
      type: bearer
      token: ${FOGHORN_GRAFANA_TOKEN}

  - name: prometheus
    type: prometheus
    url: http://localhost:9090

  - name: betterstack
    type: betterstack
    auth:
      type: bearer
      token: ${FOGHORN_BETTERSTACK_TOKEN}
    betterstack:
      on_call_schedule: default
```

`${ENV_VAR}` references are expanded at load time, so secrets can live in your shell environment or a secrets manager rather than the config file.

The variables must be available to the Foghorn process. An app opened from the desktop does not necessarily inherit variables exported in a terminal. Launch the executable from that terminal when using shell-provided credentials.

See `config.example.yaml` for the full reference, including severity mapping, display/grouping options, notification rules, and Better Stack-specific fields.

### Command line

Foghorn can report its version and manage saved cookie and OIDC logins without
opening the desktop application:

```bash
foghorn --version
foghorn -v
foghorn auth list
foghorn auth list --json
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
[`docs/oidc-device-auth.md`](oidc-device-auth.md).

The `ui.scale` block can enlarge the app for accessibility. Use `mode: fonts` to scale text only, or `mode: interface` to scale the full web UI. `factor` accepts `0.75` through `2.0`; when `mode: interface`, `apply_to_popup: true` also resizes the popup window.

For how the **Sort** selector's presets work — what each one orders by, where `startsAt`/`updatedAt` come from, and the provider-specific caveats — see [`docs/sorting.md`](sorting.md).

For the search bar's query syntax — field matchers, negation, regex anchoring, and how a query becomes a silence — see [`docs/search.md`](search.md).

## Resolvers and actions

See [resolvers](resolvers.md) for local field transformations and [troubleshooting](troubleshooting.md#local-alert-actions) for removed shell actions.
