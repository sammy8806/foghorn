# Silence Editor — Revealed Matchers UX

**Date:** 2026-05-27  
**Status:** Approved

## Problem

When a user clicks "Show N more matchers" in the silence editor, the previously-collapsed
matchers are appended to the bottom of the list but look identical to the always-visible
ones. The user has to re-scan the entire list to find what just appeared.

## Goals

- Revealed matchers always appear **below** the already-visible ones (already true; preserved).
- A persistent visual **separator** marks the boundary between always-visible and previously-collapsed matchers, so the user has a stable anchor while reading.
- The previously-collapsed chips receive an **ephemeral tint** (fades out ~2.5 s) so the eye is drawn to what just appeared without permanent visual noise.
- The **Matchers field label** shows the total count including collapsed ones, so the user always knows how many matchers exist without having to expand.

## Non-goals

- No reordering of matchers.
- No permanent colour-coding by "type" of matcher.
- No changes to collapse/expand trigger behaviour.

## Design

### SilenceEditor.svelte

Two new reactive variables:

```ts
let revealedAfterIndex: number | null = null;  // index where revealed matchers start
let revealedCount: number = 0;                 // how many were revealed in this expand
```

**`expandMatchers()`** — snapshot before appending:

```ts
function expandMatchers() {
  revealedAfterIndex = editorMatchers.length;
  revealedCount = hiddenMatchers.length;
  editorMatchers = [...editorMatchers, ...hiddenMatchers];
  hiddenMatchers = [];
  expanded = true;
}
```

**`collapseMatchers()`** — clear the markers:

```ts
function collapseMatchers() {
  const { visible, hidden } = splitMatchers(editorMatchers);
  editorMatchers = visible;
  hiddenMatchers = hidden;
  expanded = false;
  revealedAfterIndex = null;
  revealedCount = 0;
}
```

**`applyCollapse()`** — also clear on full reset (e.g. dialog re-open):

```ts
revealedAfterIndex = null;
revealedCount = 0;
```

**Matchers field label** — use `allMatchers.length`:

```svelte
<span class="field-label">Matchers ({allMatchers.length})</span>
```

Pass new props to `MatcherEditor`:

```svelte
<MatcherEditor
  bind:matchers={editorMatchers}
  source={alert.source}
  revealedAfterIndex={revealedAfterIndex}
  revealedCount={revealedCount}
>
```

### MatcherEditor.svelte

Two new optional props:

```ts
export let revealedAfterIndex: number | null = null;
export let revealedCount: number = 0;
```

Helper to decide whether a chip index falls in the revealed range:

```ts
function isRevealed(i: number): boolean {
  if (revealedAfterIndex === null) return false;
  return i >= revealedAfterIndex && i < revealedAfterIndex + revealedCount;
}
```

**Separator** — rendered as a sibling element just before the first revealed chip.
Because `{#each}` doesn't support injecting elements between iterations cleanly,
wrap each iteration and conditionally render the separator before the chip at
`i === revealedAfterIndex`:

```svelte
{#each matchers as m, i (i)}
  {#if revealedAfterIndex !== null && i === revealedAfterIndex}
    <div class="revealed-separator">· · · more · · ·</div>
  {/if}
  <div class="chip" class:invalid={...} class:was-collapsed={isRevealed(i)}>
    …
  </div>
{/each}
```

**CSS — separator** (persistent, disappears only when `revealedAfterIndex` becomes null):

```css
.revealed-separator {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #475569;
  font-size: 10px;
  letter-spacing: 0.05em;
  margin: 2px 0;
  user-select: none;
}
.revealed-separator::before,
.revealed-separator::after {
  content: '';
  flex: 1;
  height: 1px;
  background: #334155;
}
```

**CSS — ephemeral chip tint** (fades in 2.5 s, does not shift layout):

```css
@keyframes revealed-fade {
  0%   { background: #1e3a5f; box-shadow: -2px 0 0 #3b82f6; }
  70%  { background: #1e3a5f; box-shadow: -2px 0 0 #3b82f6; }
  100% { background: #0f172a; box-shadow: none; }
}

.chip.was-collapsed {
  animation: revealed-fade 2.5s ease-out forwards;
}
```

Note: `box-shadow: -2px 0 0` is used instead of `border-left` to avoid any layout shift (box-shadow does not affect the box model). The terminal background `#0f172a` matches `.chip`'s resting background.

### Edge cases

- **User adds a matcher after expanding:** new matchers are appended beyond
  `revealedAfterIndex + revealedCount` and receive no tint. The separator stays
  in place between the original two groups. ✓
- **User removes a visible matcher (index < revealedAfterIndex):** all indices
  shift down by one; the separator drifts by one position. Accepted best-effort —
  this is a rare action and low-impact visually.
- **`collapseEnabled = false`:** `revealedAfterIndex` stays null; no separator or
  tint is ever rendered.

## Files changed

| File | Change |
|------|--------|
| `frontend/src/components/SilenceEditor.svelte` | Add `revealedAfterIndex` / `revealedCount` state; update `expandMatchers`, `collapseMatchers`, `applyCollapse`; update field label; pass props to `MatcherEditor` |
| `frontend/src/components/MatcherEditor.svelte` | Add props; render separator; apply `was-collapsed` class; add CSS |
