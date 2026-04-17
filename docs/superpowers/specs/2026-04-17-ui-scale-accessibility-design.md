# Foghorn UI Scaling for Accessibility — Design

**Status:** Draft
**Date:** 2026-04-17
**Author:** steven.tappert@gmail.com (drafted with Claude)

## Summary

Foghorn currently hardcodes all UI dimensions in CSS pixels, including a
`font-size: 13px` base on `body`. Users who need larger text or a zoomed
interface have no way to adjust it. This spec introduces an optional
`ui.scale` configuration block that supports two modes:

- **`fonts`** — scales only text.
- **`interface`** — scales fonts, spacing, icons, and (optionally) the
  popup window.

Scaling applies live through the existing config watcher; no restart is
required.

## Goals

- Provide accessibility-driven font scaling for users who need larger text.
- Provide full-interface scaling for users who prefer a uniformly larger UI.
- Keep configuration idiomatic with the existing `ui:` section.
- Apply changes live on config save, consistent with other Foghorn settings.
- No behavior change for users who do not set `ui.scale`.

## Non-goals

- Scaling native OS surfaces: tray menus, system notifications, macOS
  window chrome. These are outside the app's control.
- Per-component font-size overrides.
- Runtime UI controls (hotkeys, preferences panel). Config-only for now.
- Remembering a preference independent of `config.yaml`.

## Configuration

New nested block under `ui:`:

```yaml
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
  default_created_by: ${USER}
  # Optional UI scaling for accessibility.
  scale:
    factor: 1.0            # 0.75–2.0, suggested presets 1.0/1.25/1.5/1.75/2.0
    mode: fonts            # "fonts" | "interface"
    apply_to_popup: true   # only consulted when mode is "interface"
```

### Fields

| Field            | Type    | Default   | Notes                                                                 |
|------------------|---------|-----------|-----------------------------------------------------------------------|
| `factor`         | float   | `1.0`     | Valid range `[0.75, 2.0]`. Out-of-range clamped with a warning log.   |
| `mode`           | string  | `"fonts"` | One of `fonts`, `interface`. Invalid value → config error.            |
| `apply_to_popup` | bool    | `true`    | When `mode: interface`, resize popup window by `factor`. Else ignored. |

### Backwards compatibility

Existing configs without a `scale:` block receive the defaults, which
evaluate to a no-op. No migration required.

## Architecture

```
config.yaml  ──►  internal/config (load + watcher)
                        │
                        ▼
                  app.go (Wails binding + events)
                    │               │
     GetUIScale() ──┘               └── runtime.EventsEmit("ui:scale", msg)
          │                              │
          ▼                              ▼
   Svelte store `uiScale`  ◄────  subscribes on startup
          │
          ├── sets CSS custom properties on :root
          │     --font-scale    (multiplier for font-size declarations)
          │     --ui-scale      (multiplier for #app zoom in interface mode)
          ├── toggles class `.scale-interface` on #app
          └── triggers runtime.WindowSetSize when mode=interface
              && apply_to_popup=true
```

### Backend (Go)

**`internal/config/types.go`:**

```go
type UIScale struct {
    Factor       float64 `yaml:"factor"`
    Mode         string  `yaml:"mode"`            // "fonts" | "interface"
    ApplyToPopup bool    `yaml:"apply_to_popup"`
}

type UI struct {
    // ... existing fields ...
    Scale UIScale `yaml:"scale"`
}
```

**`internal/config/config.go`:**
- Extend the existing `Default()` function to pre-populate:
  - `Scale.Factor: 1.0`
  - `Scale.Mode: "fonts"`
  - `Scale.ApplyToPopup: true`
  This matches the project's convention for true-by-default bools
  (see `Notifications.Enabled`, `Notifications.OnNew` in `Default()`).
  YAML unmarshal then overlays user-specified values on top.
- Extend the existing post-unmarshal normalize logic (near the
  `default_created_by` fallback in `Load`):
  - If `Factor == 0` (YAML explicitly set to `0` or omitted when pattern
    fails), re-set to `1.0`.
  - If `Factor < 0.75` or `Factor > 2.0`, clamp to the nearest bound and
    log a warning: `ui.scale.factor 3.0 is outside [0.75, 2.0], clamped to 2.0`.
  - If `Mode == ""`, re-set to `"fonts"`.
  - If `Mode` not in `{"fonts", "interface"}`, return a config load error
    (same pattern as other enum fields in the file).

**`app.go`:**
- Add bound method `GetUIScale() UIScale` for the frontend to read the
  current value on startup.
- Subscribe to config reload events already emitted by the watcher; on
  each reload, `runtime.EventsEmit(ctx, "ui:scale", currentScale)`.
- When `Mode == "interface"` and `ApplyToPopup`, also call
  `runtime.WindowSetSize(ctx, int(popupWidth*factor), int(popupHeight*factor))`.
  When transitioning *out of* interface mode (or reducing the factor),
  reset to the base `popup_width`/`popup_height`.

### Frontend (Svelte + CSS)

**`frontend/src/stores/uiScale.ts`** (new):
- Exports a writable Svelte store holding `{ factor, mode, applyToPopup }`.
- On module load, calls `GetUIScale()` (generated Wails binding) to seed.
- Subscribes via `EventsOn("ui:scale", …)` to receive live updates.

**`frontend/src/App.svelte`:**
- Subscribes to the `uiScale` store.
- On every change:
  - `document.documentElement.style.setProperty('--font-scale', String(factor))`
  - `document.documentElement.style.setProperty('--ui-scale', mode === 'interface' ? String(factor) : '1')`
  - Toggles `document.getElementById('app')?.classList.toggle('scale-interface', mode === 'interface')`.

**`frontend/src/style.css`:**

```css
:root {
  --font-scale: 1;
  --ui-scale: 1;
}

#app.scale-interface {
  zoom: var(--ui-scale);
}
```

**Refactor pass (prep commit):**

Every `font-size: Npx` declaration across the Svelte frontend is rewritten
to `font-size: calc(Npx * var(--font-scale, 1))`. The `, 1` fallback
keeps behavior unchanged when the custom property is not set (e.g., Vite
dev preview).

Targets:
- `frontend/src/style.css`
- `frontend/src/App.svelte`
- `frontend/src/components/AlertCard.svelte`
- `frontend/src/components/AlertGroup.svelte`
- `frontend/src/components/AlertList.svelte`
- `frontend/src/components/SilenceDialog.svelte`

This refactor is behavior-preserving at the default `--font-scale: 1`.

## Error handling

- Invalid `mode` at startup: config load fails (same behavior as other
  enum fields). Foghorn exits with a clear message.
- Invalid `mode` introduced on hot-reload: log the error and keep the
  previous valid scale settings. Do not crash the app.
- Out-of-range `factor`: clamp and warn, both at startup and on reload.
- Popup resize call failure: log at warn level and keep going; text
  scaling still applies.

## Testing

### Go unit tests (`internal/config/config_test.go`)
- Defaults applied when `scale:` block is absent.
- Defaults applied when fields are only partially specified.
- Factor clamped at `0.75` lower bound (e.g. input `0.5` → `0.75`).
- Factor clamped at `2.0` upper bound (e.g. input `5.0` → `2.0`).
- Invalid `mode` returns an error at load time.
- Watcher reload updates the scale fields (uses existing watcher test
  patterns in `watcher_test.go`).

### Manual verification checklist
1. Fresh install (no `scale:` block) → identical to pre-change behavior.
2. `scale.factor: 1.25`, `mode: fonts` → text grows, popup size unchanged.
3. `scale.factor: 2.0`, `mode: fonts` → text at 2x, layout still intact,
   no visual regression in `AlertCard` badges/chevrons.
4. `scale.factor: 1.5`, `mode: interface`, `apply_to_popup: true` →
   popup becomes 1200×900, all content scales uniformly.
5. `scale.factor: 1.5`, `mode: interface`, `apply_to_popup: false` →
   popup stays 800×600, content at 150% (scrollbars expected).
6. Live edit: while app is running, change `factor` in `config.yaml`,
   save → UI updates without restart.
7. Out-of-range value (e.g. `factor: 3.0`) → warning logged, scale
   applied at `2.0`.
8. Invalid `mode: huge` at startup → app refuses to start with a clear
   error.

## Rollout

Three commits in order:

1. **Refactor** — `font-size` declarations rewritten to
   `calc(Npx * var(--font-scale, 1))`. Zero behavior change at defaults.
2. **Feature** — config schema, validation, watcher integration, Wails
   bindings, events, Svelte store, CSS vars, popup resize.
3. **Docs** — update `config.example.yaml` and a brief note in
   `README.md` describing the new section.

## Open questions

None at time of writing. All design questions were resolved during
brainstorming; see the Q&A log in the session transcript for context.
