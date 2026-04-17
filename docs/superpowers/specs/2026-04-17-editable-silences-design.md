# Editable Silences — Design Spec

- **Date:** 2026-04-17
- **Project:** foghorn
- **Status:** Draft

## 1. Problem & goals

Today, foghorn can **create** silences from an alert card and Alertmanager itself holds the silence, but once a silence exists the user cannot touch it from foghorn. They have to jump to the Alertmanager or Grafana UI to change anything.

Goals:

1. Let users edit a silence that is already attached to an alert shown in foghorn, without leaving the app.
2. Editing covers: **comment**, **end time** (extend or shorten), **matchers** (add, remove, change operator / value).
3. One-click **expire now** for an existing silence, usable both inline on the alert card and from inside the editor.
4. The **create** flow gets the same rich matcher editor, so users can narrow or broaden a silence at creation time instead of accepting the default "all labels, exact match".
5. The UI stays small and focused — no new top-level screens.

## 2. Non-goals

- No global "Silences" management view. Editing happens where silences surface today: attached to alerts on the main list.
- No user-controlled `startsAt`. The field is always set to `time.Now()` on submit for both create and update — users cannot schedule a silence to start in the future or preserve a historical start time.
- No editing of pending or expired silences (foghorn filters to `state == "active"` today; unchanged).
- No backend label-suggestions endpoint in this iteration — the endpoint is sketched for a follow-up (see §9).
- No support for Prometheus / Better Stack silence edits. Alertmanager + Grafana Alerting (which shares the Alertmanager v2 API) only. Prometheus and Better Stack providers already return `SupportsSilence() == false` or do not surface silences on alerts; they are unaffected.

## 3. UX flows

### 3.1 Edit an existing silence

1. User expands an alert card that has one or more silences.
2. Each silence card under the alert body grows two buttons: **Edit** (left) and **Expire now** (right, red-tinted, with confirm).
3. Clicking **Edit** opens the `SilenceEditor` dialog in `mode="edit"`, pre-filled from that silence's `SilenceInfo`.
4. User changes matchers / comment / duration → **Save**.
5. Backend re-POSTs to Alertmanager with the existing silence ID. The silence keeps its ID; `startsAt` becomes now, `endsAt = now + duration`, matchers/comment are replaced.
6. Dialog closes. Next poll picks up the change; a single manual `RefreshAlerts()` is triggered immediately after a successful save so the user sees the update without waiting.

### 3.2 Expire a silence without opening the editor

1. User clicks the small **Expire now** button on the silence card (expanded alert body).
2. Inline confirm appears ("Expire this silence? [Cancel] [Expire]").
3. On confirm, backend calls `Unsilence`. Alert updates on next refresh (same immediate-refresh trigger as above).

### 3.3 Create a silence (updated)

1. User clicks **Silence…** on an alert (existing entry point, unchanged).
2. `SilenceEditor` opens in `mode="create"`, pre-filled with:
   - Matchers: every label on the alert as `name = value` (exact-match). Identical to today's default, but now visible and editable before submit.
   - Duration: `2h`.
   - Created by: from `UIConfig.DefaultCreatedBy`.
   - Comment: empty.
3. User tunes matchers / duration / comment and submits.
4. Backend creates the silence. Refresh as above.

## 4. Architecture

```
frontend/src/components/
  AlertCard.svelte             (modified)
      └─ uses SilenceEditor (create + edit)
      └─ uses an inline ExpireNowButton on each silence card

  SilenceEditor.svelte         (renamed from SilenceDialog.svelte, expanded)
      └─ composes MatcherEditor
      └─ shared create/edit shell via `mode: "create" | "edit"` prop

  MatcherEditor.svelte         (new)
      └─ renders chips for existing matchers
      └─ uses LabelAutocomplete for name and value inputs
      └─ per-chip operator picker (=, !=, =~, !~)

  LabelAutocomplete.svelte     (new — reusable)
      └─ input + dropdown
      └─ suggestions from currently-fetched alerts (scoped to the silence's source)
      └─ always allows free-form input (needed for regex values)
```

Backend additions/changes are listed in §5.

## 5. Backend changes

### 5.1 Data model — `internal/model/types.go`

Extend `SilenceInfo` with matchers so the frontend can render them in the editor:

```go
type SilenceInfo struct {
    ID        string    `json:"id"`
    CreatedBy string    `json:"createdBy"`
    Comment   string    `json:"comment"`
    StartsAt  time.Time `json:"startsAt"`
    EndsAt    time.Time `json:"endsAt"`
    Matchers  []Matcher `json:"matchers"` // NEW
}
```

No change to `Matcher` itself — it already carries `Name`, `Value`, `IsRegex`, `IsEqual`.

### 5.2 Provider — `internal/provider/alertmanager.go`

- Add `Matchers []amMatcher` to the `amSilence` response struct and copy them into `SilenceInfo` in `FetchSilences`.
- Add `ID string` to `amSilenceRequest`. When non-empty, Alertmanager v2's `POST /silences` treats the call as an update of the existing silence (verified against Alertmanager API: the `gettableSilence` / `postableSilence` schema accepts an `id` for updates; Grafana Alerting's Alertmanager endpoint behaves the same).
- `alertmanagerAPI.Silence` keeps its signature but forwards `req.ID` into the body (empty for create, set for update).

### 5.3 Manager — `internal/silence/manager.go`

Current `SilenceAlert` derives matchers from the alert's labels. That is the wrong place now that matchers come from the UI. Reshape the manager around explicit matchers:

```go
// Create a silence with explicit matchers.
func (m *Manager) CreateSilence(
    ctx context.Context,
    source string,
    matchers []model.Matcher,
    duration string,
    createdBy, comment string,
    defaultCreatedBy string,
) (string, error)

// Update an existing silence in place.
func (m *Manager) UpdateSilence(
    ctx context.Context,
    source, silenceID string,
    matchers []model.Matcher,
    duration string,
    createdBy, comment string,
    defaultCreatedBy string,
) error

// Unsilence stays as-is: m.Unsilence(ctx, source, silenceID) error
```

Both methods compute `startsAt = time.Now()` and `endsAt = time.Now().Add(dur)` and forward to the provider. The only difference is that `UpdateSilence` sets `req.ID = silenceID`.

The existing `Unsilence` method (and its Wails binding) stays as-is — only the UI-facing label changes to **Expire now** in §6. The old `SilenceAlert` and `SilenceByLabels` methods are removed; no other callers besides the Wails binding, and that binding is changing anyway. Tests in `manager_test.go` migrate to the new signatures.

### 5.4 Wails bindings — `app.go`

Replace the existing `SilenceAlert` binding and add two more:

```go
func (a *App) CreateSilence(
    source string,
    matchers []model.Matcher,
    duration, createdBy, comment string,
) (string, error)

func (a *App) UpdateSilence(
    source, silenceID string,
    matchers []model.Matcher,
    duration, createdBy, comment string,
) error

// existing, unchanged:
func (a *App) Unsilence(source, silenceID string) error
```

`Matcher` is a simple JSON struct so it maps cleanly into Wails' generated TS types.

### 5.5 Future: backend label suggestions (deferred, 2c)

Out of scope for the initial build; sketched here so a later iteration can plug in without UI rework:

```go
// Returns unique (label name → sorted unique values) for the given source.
func (a *App) GetLabelSuggestions(source string) map[string][]string
```

When implemented, `LabelAutocomplete.svelte` will prefer this over the frontend-aggregated list if present. Not wired in v1.

## 6. Frontend changes

### 6.1 `SilenceEditor.svelte` (renamed, expanded)

**Props:**

```ts
export let alert: Alert | null = null;           // source of truth for create; identifies source for edit
export let silence: SilenceInfo | null = null;   // present in edit mode
export let mode: 'create' | 'edit' = 'create';
export let open = false;
```

**State:**

- `matchers: Matcher[]` — seeded from the alert's labels (create) or `silence.matchers` (edit).
- `duration: string` — `"2h"` for create; for edit, derived from `silence.endsAt - now`, rounded to a clean unit (`1h30m`, not `1h29m58s`), and clamped to `"0s"` if somehow already negative.
- `createdBy: string` — editable input. Default: `UIConfig.DefaultCreatedBy` for create; `silence.createdBy` for edit.
- `comment: string` — empty for create; `silence.comment` for edit. Always editable.
- `loading`, `error`, `confirmExpire` (edit only).

**Layout (top to bottom):**

1. Header: `Silence alert` (create) / `Edit silence` (edit).
2. Read-only context strip:
   - Create: alert name + source (same as today).
   - Edit: silence ID (truncated), original `startsAt`, original `createdBy`, and `expires in Xh Ym`.
3. `MatcherEditor` (see §6.2).
4. Duration row:
   - Input labeled **Ends in** — accepts any `time.ParseDuration`-compatible string.
   - Base presets: `30m`, `1h`, `2h`, `4h`, `8h`, `24h` — replace the value when clicked.
   - **Extend-by** shortcuts (edit mode only): `+30m`, `+1h`, `+4h`, `+1d`. These parse the current field value, add the shortcut, and write the normalized sum back (e.g., `2h30m` + `+30m` = `3h`). Keeps the "duration is now-anchored" model consistent.
5. `Comment` textarea.
6. `Created by` input.
7. Footer:
   - Left: **Expire now** (edit mode only, red tint, two-click confirm).
   - Right: **Cancel**, **Save** (label reads `Silence` in create mode, `Save changes` in edit).

**Submit logic:**

- Create: `CreateSilence(alert.source, matchers, duration, createdBy, comment)`.
- Edit: `UpdateSilence(alert.source, silence.id, matchers, duration, createdBy, comment)`.
- Expire now: `Unsilence(alert.source, silence.id)`.
- After any success: dispatch a `silenced` event; `AlertList` listens and calls `RefreshAlerts()` (existing pattern).

### 6.2 `MatcherEditor.svelte`

**Props:**

```ts
export let matchers: Matcher[];             // two-way bound via bind:matchers
export let source: string;                  // scopes autocomplete
```

**Rendering:**

- One chip per matcher, horizontal wrap:
  ```
  [ name ▼ ] [ op ▼ ] [ value ▼ ]  ✕
  ```
  - `name ▼`: `LabelAutocomplete` (suggests names).
  - `op ▼`: small select with `=`, `!=`, `=~`, `!~`.
  - `value ▼`: `LabelAutocomplete` (suggests values for the chosen name).
  - `✕`: removes the chip.
- Trailing "+ Add matcher" button inserts a blank chip focused on the name field.
- Matchers can be reordered? **No** — ordering is cosmetic only for Alertmanager, so we skip drag-and-drop.

**Operator → `(isEqual, isRegex)` mapping:**

| Symbol | isEqual | isRegex |
|--------|---------|---------|
| `=`    | true    | false   |
| `!=`   | false   | false   |
| `=~`   | true    | true    |
| `!~`   | false   | true    |

**Validation (inline, per-chip):**

- Name must be non-empty.
- Value must be non-empty.
- If `isRegex`, value is compiled with `new RegExp(value)` in the frontend for an early smoke test; invalid regex highlights the chip and disables Save.
- Whole-form validation: at least one matcher present (Alertmanager rejects empty matcher lists anyway, but we catch it client-side).

### 6.3 `LabelAutocomplete.svelte`

**Props:**

```ts
export let value: string;                   // bound
export let suggestions: string[];           // caller-provided, pre-filtered
export let placeholder: string;
export let ariaLabel: string;
```

- Plain `<input>` with a custom dropdown beneath it.
- Dropdown filters `suggestions` by case-insensitive substring match on every keystroke.
- Arrow keys navigate; Enter picks; Escape closes.
- Free-form text is always allowed — typing something that isn't in the list and tabbing out just accepts the raw value. Matters for regex values.
- Component has no knowledge of labels specifically; it's a generic "typeahead that allows free input". The matcher editor supplies the right suggestion list per row.

**Suggestion sources (frontend-derived, per 2c):**

A small helper in `stores/alerts.ts`:

```ts
export function labelNamesForSource(source: string): string[]
export function labelValuesForSource(source: string, name: string): string[]
```

Both scan `$alerts` (current store), filter by `source`, and return de-duplicated sorted lists. Called reactively inside `MatcherEditor` so the suggestion arrays update as alerts refresh.

### 6.4 `AlertCard.svelte` — silence card + Expire-now inline

The existing silence details block (expanded body) is restructured slightly:

```html
{#each alert.silences as s}
  <div class="silence-card">
    <div class="silence-header">
      <span class="silence-author">{s.createdBy}</span>
      <span class="silence-expiry">expires in {formatTimeRemaining(s.endsAt)}</span>
      <div class="silence-actions">
        <button class="btn-link-edit"    on:click={() => openEdit(s)}>Edit</button>
        <button class="btn-link-expire"  on:click={() => confirmExpire(s)}>Expire now</button>
      </div>
    </div>
    {#if s.comment}
      <div class="silence-comment">{s.comment}</div>
    {/if}
  </div>
{/each}
```

- `confirmExpire(s)` flips the card into a two-state confirm (inline), then calls `Unsilence(alert.source, s.id)`.
- `openEdit(s)` opens the single `SilenceEditor` instance in edit mode.

The existing "Silence…" button (below, in the action row) keeps working and opens the same editor in create mode. The guard `!alert.silencedBy?.length` is kept so we don't show two entry points to silence the same alert.

## 7. Error handling

| Where | Failure | Handling |
|-------|---------|----------|
| `UpdateSilence` | Alertmanager returns 4xx (e.g. unknown ID, bad matcher) | Surface body in dialog's `error` pane; keep dialog open so the user can retry or cancel. |
| `UpdateSilence` | Network error | Same — error pane, no retry loop. |
| `Unsilence` (inline) | Any failure | Replace the confirm row with an inline error (red text, tiny retry button). |
| `FetchSilences` | Silence referenced by an alert isn't returned (race between poll and edit) | Current code already tolerates this — the silence card just doesn't render. No regression. |
| Regex validation | Client-side `new RegExp` throws | Highlight chip, block Save. We don't re-validate server-side — Alertmanager's own validation catches anything we miss. |
| Save with empty matcher list (create or edit) | N/A — Save button is disabled whenever `matchers.length === 0` or any chip fails validation. |  |

No silent fallbacks anywhere. An operation either succeeds or surfaces the error. This matches the rest of the app.

## 8. Testing strategy

**Backend (Go):**

- `internal/silence/manager_test.go`:
  - Rewrite existing tests against the new `CreateSilence` / `UpdateSilence` signatures.
  - Add a test that `UpdateSilence` forwards the silence ID into the provider's `Silence` call (using the existing fake provider).
  - Add a test that duration parsing errors bubble up without calling the provider.
- `internal/provider/alertmanager_test.go`:
  - Add a test that POSTing a `SilenceRequest` with a non-empty ID includes `"id":"..."` in the body.
  - Add a test that `FetchSilences` parses matchers into `SilenceInfo.Matchers` correctly (fixture with one `=`, one `=~`, one `!=`, one `!~`).

**Frontend (manual for now — foghorn has no Svelte test harness set up):**

- Open a silenced alert, hit Edit, change the comment, Save → comment updates on next refresh.
- Edit a silence, change a matcher from `=` to `=~`, set value to `.*prod.*`, Save → silence now matches the regex.
- Extend a silence: click `+1h` — duration field bumps; Save — expiry moves forward by 1h.
- Click Expire now from the card → confirm → silence disappears from alert after refresh.
- Click Expire now from inside the editor → silence disappears; dialog closes.
- Create: open Silence…, remove a few chips, add one with `=~` and a regex value, Save → silence created with the custom matchers.
- Error cases: disconnect network, try to save — error banner appears in the dialog, dialog stays open.

If we add a Svelte test harness later, the manual checklist becomes the test matrix for `SilenceEditor`, `MatcherEditor`, and `AlertCard`.

## 9. Deferred / future work

- **Backend label suggestions (2c):** `GetLabelSuggestions(source)` — described in §5.5. Valuable once we want suggestions from alerts that aren't currently firing or from Prometheus `/api/v1/labels`. Not blocking.
- **Silences management view:** a dedicated screen listing all active silences across sources, with filter/sort. Out of scope here; today's "edit from the alert card" flow covers the stated need.
- **`startsAt` editing:** if a use case emerges for scheduling a silence to start later or to keep a historical start time, revisit. Currently explicitly out of scope.
- **Prometheus / Better Stack silence edits:** contingent on those providers exposing silence edit APIs.

## 10. Build sequence (rough)

1. Backend: extend `SilenceInfo` with matchers + parse in `FetchSilences`.
2. Backend: add `id` to `amSilenceRequest`; thread through `Silence`.
3. Backend: reshape `silence.Manager` and Wails bindings (`CreateSilence`, `UpdateSilence`).
4. Backend tests.
5. Frontend: `LabelAutocomplete.svelte` (generic, testable on its own).
6. Frontend: `MatcherEditor.svelte`.
7. Frontend: rename + expand `SilenceDialog` → `SilenceEditor`. Wire both modes.
8. Frontend: update `AlertCard.svelte` — Edit + inline Expire-now buttons on silence cards.
9. Manual test pass against a local Alertmanager.

Detailed steps, interfaces, and checkpoints are the job of the follow-up implementation plan.
