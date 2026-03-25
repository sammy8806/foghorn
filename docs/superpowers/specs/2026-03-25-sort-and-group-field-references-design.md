# Sort and Group Field References

**Date:** 2026-03-25
**Status:** Draft

## Summary

Add flexible sorting and grouping to the alert list. Both `sort_by` and `group_by` in config support a unified field reference syntax that goes beyond simple label names. The UI gets an ephemeral sort toggle in the status bar for quick switching between presets.

## Field Reference Syntax

All field references use an explicit prefix to distinguish built-in fields from labels and annotations.

| Prefix | Description | Examples |
|---|---|---|
| `field:<name>` | Built-in alert field | `field:severity`, `field:startsAt` |
| `label:<name>` | Alert label value | `label:cluster`, `label:namespace` |
| `annotation:<name>` | Alert annotation value | `annotation:team` |

### Available built-in fields (`field:*`)

| Field | Type | Sort behavior |
|---|---|---|
| `field:severity` | enum | critical < warning < info < unknown |
| `field:startsAt` | time | Newest first (desc) by default |
| `field:updatedAt` | time | Newest first (desc) by default |
| `field:source` | string | Alphabetical (asc) by default |
| `field:name` | string | Alphabetical (asc) by default |
| `field:state` | enum | firing < silenced < inhibited < resolved |

### Backwards compatibility

Bare strings without a prefix are treated as label names, preserving current behavior:

```yaml
# These are equivalent:
group_by:
  - cluster
group_by:
  - label:cluster
```

The one exception: `sort_by: severity` (bare string, not a list) continues to work as before.

## Config Format

### `group_by`

Accepts a list of field references. Alerts are grouped by the combined values.

```yaml
display:
  # Current (still works)
  group_by:
    - cluster

  # New: group by built-in fields and labels
  group_by:
    - field:source
    - label:namespace
```

### `sort_by`

Accepts either a simple string (backwards compatible) or a list of sort criteria.

```yaml
display:
  # Simple (backwards compatible)
  sort_by: severity

  # Advanced: ordered list of sort criteria
  sort_by:
    - field: field:severity
      order: asc
    - field: field:startsAt
      order: desc
    - field: label:namespace
      order: asc
```

Each sort criterion has:
- `field` (required): A field reference
- `order` (optional): `asc` or `desc`. Defaults vary by field type: time fields default to `desc` (newest first), all others default to `asc`.

When `sort_by` is a simple string:
- `severity` maps to `[{field: field:severity, order: asc}, {field: field:startsAt, order: desc}]`
- Any other bare string is treated as a label sort: `[{field: label:<value>, order: asc}]`

## UI Sort Toggle

### Location

In the status bar, after the alert count on the left side:

```
12 alerts · Severity ▾                        ● 14:32:05 ↻
```

### Interaction

Clicking the sort label opens a dropdown overlay with preset options:

| Preset | Resolves to |
|---|---|
| Default | Whatever `sort_by` is in the config file |
| Severity | `field:severity` asc, then `field:startsAt` desc |
| First seen | `field:startsAt` desc |
| Last seen | `field:updatedAt` desc |
| Active first | `field:state` asc, then `field:severity` asc |
| Source | `field:source` asc, then `field:severity` asc |

### Behavior

- Selection is **ephemeral** — resets to config default on app restart
- "Default" always reflects the current config file value (including hot-reloads)
- The dropdown dismisses on selection or clicking outside
- Active preset is indicated with a check mark or highlight

## Changes Required

### Backend (Go)

**`internal/config/types.go`**

- `DisplayConfig.SortBy` changes from `string` to `interface{}` to accept both string and list via YAML unmarshaling
- `DisplayConfig.GroupBy` remains `[]string` (field references are just strings with prefixes)
- Add a `SortCriterion` struct: `{Field string, Order string}`
- Add `ParsedSortBy() []SortCriterion` method that normalizes both forms into a structured list
- Add `ResolveFieldRef(ref string) (kind string, name string)` utility to parse `field:x`, `label:x`, `annotation:x`

**`app.go`**

- `GetDisplayConfig()` returns the parsed/normalized config to the frontend

### Frontend (TypeScript/Svelte)

**`frontend/src/stores/alerts.ts`**

- Add `SortCriterion` interface: `{field: string, order: 'asc' | 'desc'}`
- Add `activeSortMode` writable store (string, defaults to `'default'`)
- Add `activeSortCriteria` derived store that resolves the active mode to `SortCriterion[]`
- Update `DisplayConfig` interface: `sort_by` becomes `SortCriterion[]` (backend normalizes before sending)
- Replace `sortByConfig()` with generic `sortByCriteria(criteria: SortCriterion[])` that:
  - Resolves `field:*` to built-in alert properties
  - Resolves `label:*` to `alert.labels[name]`
  - Resolves `annotation:*` to `alert.annotations[name]`
  - Handles enum ordering for `severity` and `state`
  - Handles time comparison for `startsAt`/`updatedAt`
  - Falls back to string comparison otherwise
- Update `groupedAlerts` derived store to resolve field references in `group_by`

**`frontend/src/components/AlertList.svelte`**

- Add sort toggle to status bar: clickable label + dropdown
- Import `activeSortMode` store
- Dropdown component with the 6 preset options
- Click-outside dismissal

**`frontend/src/stores/filter.ts`**

- No changes needed — filtering is independent of sorting

### Example Config

Update `config.example.yaml` to show the new syntax with documented possible values.

## Out of Scope

- Persisting UI sort selection across restarts (future enhancement)
- Custom user-defined presets in config
- Sort direction toggle in UI (presets have fixed directions)
