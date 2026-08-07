# Sorting & display ordering

How foghorn orders alerts in the list, where the underlying values come from,
and the caveats worth knowing when a sort doesn't behave as you'd expect.

## Sort presets (the UI toggle)

The **Sort** selector at the top of the alert list switches between a set of
named presets. Each preset resolves to a list of sort *criteria* that are
applied in order — the first criterion is the primary sort, the rest are
tiebreakers. The presets are defined in `frontend/src/stores/alerts.ts`
(`SORT_PRESETS` / `SORT_PRESET_OPTIONS`).

| Preset | Criteria (in order) | Effect |
|--------|---------------------|--------|
| **Default** | from config `display.sort_by` | Whatever the loaded config specifies. Ships as `severity` asc, then `startsAt` desc. |
| **Severity** | `field:severity` asc, `field:startsAt` desc | Most severe first; within a severity, newest first. |
| **First seen** | `field:startsAt` asc | Oldest alert start time first. |
| **Last seen** | `field:updatedAt` desc | Most recently updated first. |
| **Active first** | `field:state` asc, `field:severity` asc | Firing alerts first, most severe within each state. |
| **Source** | `field:source` asc, `field:severity` asc | Grouped by source name, then severity. |
| **Cluster** | `label:cluster` asc, `field:severity` asc | Grouped by the `cluster` label, then severity. |
| **Alert name** | `field:name` asc, `field:severity` asc | Alphabetical by alert name, then severity. |

The active preset is **ephemeral** — it lives in the `activeSortMode` store and
resets to **Default** on app restart. Only the config file persists a sort.

## Where the fields get their data

Sort criteria reference alert fields (`field:*`) or labels (`label:*`). The
field values are populated by the backend providers when alerts are fetched
(`internal/provider/*.go`) and mapped onto `model.Alert`.

| Field | Source | Notes |
|-------|--------|-------|
| `field:startsAt` | Prometheus: rule `activeAt`. Alertmanager: alert `startsAt`. | When the current firing episode began. **Resets when an alert flutters/refires**, so it tracks the latest episode, not the all-time first occurrence. |
| `field:updatedAt` | Prometheus: `time.Now()` at poll time. Alertmanager: alert `updatedAt`. | See the Prometheus caveat below. |
| `field:severity` | Derived from labels via the configured severity label. | Ordered by configured rank, not alphabetically — see below. |
| `field:state` | Prometheus: rule `State`. Alertmanager: `status.state`. | Ordered by a fixed rank — see below. |
| `field:source` | Configured source name. | |
| `field:sourceType` | Provider type (`prometheus`, `alertmanager`, `betterstack`, ...). | |
| `field:name` | `alertname` label. | |
| `label:<name>` | The named alert label (e.g. `label:cluster`). | String compare via `localeCompare`. Missing label sorts as empty string. |

### "First seen" vs "Last seen"

- **First seen** (`startsAt` asc) is meaningful for both provider types.
- **Last seen** (`updatedAt` desc) is only meaningful for **Alertmanager**
  alerts, which carry a real `updatedAt` timestamp. For **Prometheus** alerts
  `updatedAt` is set to the moment foghorn polled, so "Last seen" effectively
  orders Prometheus alerts by *most recent scrape*, not by any per-alert update.

## Ordering rules for ranked fields

Two fields don't sort lexically — they use explicit rank maps.

### State ordering

`field:state` uses `STATE_ORDER` in `alerts.ts`. Ascending order is:

```
firing (0) < silenced (1) < inhibited (2) < resolved (3)
```

Unknown states fall back to rank `99`, so they sort to the bottom. This is what
makes **Active first** put live alerts on top and resolved ones last.

### Severity ordering

`field:severity` uses the configured severity ranks (`display` /
`severities.levels` in config; defaults in `frontend/src/stores/severity.ts`).
The shipped default ranking, ascending:

```
critical (0) < warning (1) < info (2) < unknown (3)
```

Severity is canonicalised through configured aliases before ranking; an
unrecognised severity sorts after all configured levels. Because ranks are
configurable, severity order follows your config, not alphabetical order.

## How criteria combine (priority)

Sorting is performed by `sortByCriteria`. A separate **priority** ranking
(`display.priority`) can be applied *before* or *after* the sort criteria:

- `priorityFirst = true` (default): priority alerts are pinned to the top
  regardless of the active sort, with the preset's criteria ordering the rest.
- `priorityFirst = false`: priority is only a final tiebreaker.

So a preset never overrides a "before-sort" priority — it orders everything
that priority hasn't already pinned.

## Reference

- Presets / criteria / state order: `frontend/src/stores/alerts.ts`
- Severity ranks and aliases: `frontend/src/stores/severity.ts`
- Field population: `internal/provider/prometheus.go`, `internal/provider/alertmanager.go`
- Field reference for config: comments in `config.example.yaml`
