# FAQ and troubleshooting

[Back to README](../README.md)

## Where is my configuration?

See the [configuration file locations](configuration.md#configuration). If no file exists, copy the [annotated example](../config.example.yaml) there. The current app uses defaults in memory when the file is missing; it does not create the example on first launch.

## Why does closing the window leave Foghorn running?

On macOS and Windows, closing the window hides it in the tray. Use the tray menu to show the window or quit.

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

### Silence buttons are greyed out

Foghorn greys out **Silence**, **Edit** and **Expire** when the source's
credentials can't do it, and says why in the button's tooltip:

| Hint | Fix |
|---|---|
| `token can't create silences (needs alert.silences:create)` | Give the Grafana service account or user a role with silence permissions (Editor, or a folder **Edit** permission). |
| `token can't silence alerts in folder "…"` | The token has silence permissions on other folders only. Grant **Edit** on that folder or a parent folder. |
| `token can only silence single alert rules; keep the __alert_rule_uid__ matcher` | Folder-scoped permissions only cover silences tied to one rule. Keep the rule-UID matcher in the editor, or grant permissions on all folders. |
| `token can't edit or expire silences …` | Same as above, for `alert.silences:write`. Grafana checks the silence's own matchers, not the alert it's shown on. |
| `a silence was refused (HTTP 403); fix the credentials, then refresh` | Grafana or Alertmanager refused a silence write. Foghorn keeps this until the token's permissions change, you press refresh, or the config is reloaded. |
| `silence details hidden: token can't read silences` (on the **silenced** badge) | Grant `alert.silences:read`; until then Foghorn doesn't fetch silences. |

After changing the token's permissions in Grafana, press refresh: a manual
refresh forgets refused writes and checks the permissions again instead of
waiting for `silence_check.interval`. A write that is still refused greys the
controls out again.

Grafana OSS supports folder-scoped silence permissions; they come from folder
permissions (Edit on a folder includes creating silences for its rules,
inherited by subfolders).

The log says what was checked and what came out (see the HTTP debug section
above for where logs go):

```
silence check: grafana grafana-main: configured granularity=folder on_start=true interval=15m0s on_denied=true
silence check: grafana grafana-main: granularity=folder check=ok create=folders[abc123] write=folders[abc123] read=yes rules=122 folders=17
silence write refused: grafana grafana-main HTTP 403; marked create in folder "def456"; response: user is not authorized to create silence for rule r1
error fetching silences from grafana-main: …
```

A failed lookup logs `check=error error="…"`, an old Grafana without the
permissions endpoint logs `check=unsupported`, and a token without
`alert.silences:read` adds `silence reads skipped`.

Nothing is greyed out until a permission lookup succeeds. If the lookup or
the silence list fails, the problem strip says so. To switch the check off or
make it coarser, set `silence_check` on the Grafana source (see
[Silence permission check](configuration.md#silence-permission-check)):

```yaml
silence_check:
  granularity: off   # or: source
  on_denied: false   # also stop learning from refused writes
```

### Local alert actions

Foghorn has removed shell actions and does not expose action execution to the
webview. There is no action UI, so the old Wails methods gave scripts running in
the webview backend capability that the product did not need.

The shell action format also mixed remote alert fields with shell syntax in one
rendered string. A malicious label or annotation could therefore change the
command. The renderer could not tell intended syntax from substitutions that
needed escaping. Requiring every user to quote every template correctly was not
a security boundary, so Foghorn will probably not support string-templated local
commands again. Old `type: shell` configurations now fail validation.

The non-command `url` and `clipboard` types remain in the configuration model,
but Foghorn has no UI for them yet. If local command actions return, they will
need a fixed executable and static arguments, alert data on standard input, and
a confirmation enforced by the Go backend.
