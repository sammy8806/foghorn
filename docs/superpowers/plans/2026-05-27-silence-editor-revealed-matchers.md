# Silence Editor — Revealed Matchers UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When previously-collapsed matchers are revealed in the silence editor, display them below always-visible ones with a persistent `· · · more · · ·` separator and an ephemeral chip highlight that fades out after ~2.5 s; also show total matcher count in the "Matchers" field label.

**Architecture:** `SilenceEditor.svelte` snapshots `revealedAfterIndex` and `revealedCount` when `expandMatchers()` runs and clears them on collapse/reset. It passes both down to `MatcherEditor.svelte`, which renders the separator before the first revealed chip and applies a `was-collapsed` CSS class to chips in the revealed range. No new files are needed.

**Tech Stack:** Svelte 3, TypeScript, CSS `@keyframes` (no external dependencies)

**Spec:** `docs/superpowers/specs/2026-05-27-silence-editor-revealed-matchers-design.md`

---

## File Map

| File | Change |
|------|--------|
| `frontend/src/components/SilenceEditor.svelte` | Add state variables; update `expandMatchers`, `collapseMatchers`, `applyCollapse`; update field label; pass props |
| `frontend/src/components/MatcherEditor.svelte` | Add props + `isRevealed` helper; inject separator; apply `was-collapsed` class + CSS |

---

### Task 1: Add revealed-range state to SilenceEditor

**Files:**
- Modify: `frontend/src/components/SilenceEditor.svelte`

- [ ] **Step 1: Add the two new state variables after the existing `expanded` declaration (line ~16)**

  Find the block:
  ```ts
  let expanded = false;
  ```
  Replace with:
  ```ts
  let expanded = false;
  let revealedAfterIndex: number | null = null;
  let revealedCount = 0;
  ```

- [ ] **Step 2: Update `expandMatchers` to snapshot the range before appending**

  Find:
  ```ts
  function expandMatchers() {
    editorMatchers = [...editorMatchers, ...hiddenMatchers];
    hiddenMatchers = [];
    expanded = true;
  }
  ```
  Replace with:
  ```ts
  function expandMatchers() {
    revealedAfterIndex = editorMatchers.length;
    revealedCount = hiddenMatchers.length;
    editorMatchers = [...editorMatchers, ...hiddenMatchers];
    hiddenMatchers = [];
    expanded = true;
  }
  ```

- [ ] **Step 3: Update `collapseMatchers` to clear the markers**

  Find:
  ```ts
  function collapseMatchers() {
    const { visible, hidden } = splitMatchers(editorMatchers);
    editorMatchers = visible;
    hiddenMatchers = hidden;
    expanded = false;
  }
  ```
  Replace with:
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

- [ ] **Step 4: Update `applyCollapse` to clear the markers on every full reset**

  Find:
  ```ts
  function applyCollapse(all: Matcher[]) {
    if (!collapseEnabled) {
      editorMatchers = all;
      hiddenMatchers = [];
      expanded = true;
      return;
    }
    const { visible, hidden } = splitMatchers(all);
    editorMatchers = visible;
    hiddenMatchers = hidden;
    expanded = false;
  }
  ```
  Replace with:
  ```ts
  function applyCollapse(all: Matcher[]) {
    revealedAfterIndex = null;
    revealedCount = 0;
    if (!collapseEnabled) {
      editorMatchers = all;
      hiddenMatchers = [];
      expanded = true;
      return;
    }
    const { visible, hidden } = splitMatchers(all);
    editorMatchers = visible;
    hiddenMatchers = hidden;
    expanded = false;
  }
  ```

- [ ] **Step 5: Run type-check to verify no TypeScript errors**

  ```bash
  cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
  ```
  Expected: `0 errors` (warnings about unused CSS are acceptable).

- [ ] **Step 6: Commit**

  ```bash
  git add frontend/src/components/SilenceEditor.svelte
  git commit -m "feat(silence-editor): snapshot revealed-range on expand"
  ```

---

### Task 2: Show total matcher count in the field label

**Files:**
- Modify: `frontend/src/components/SilenceEditor.svelte`

- [ ] **Step 1: Update the Matchers field label**

  Find:
  ```svelte
  <span class="field-label">Matchers</span>
  ```
  Replace with:
  ```svelte
  <span class="field-label">Matchers ({allMatchers.length})</span>
  ```

  `allMatchers` is already defined as `$: allMatchers = [...editorMatchers, ...hiddenMatchers]` — this will always reflect the full count including collapsed ones.

- [ ] **Step 2: Run type-check**

  ```bash
  cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
  ```
  Expected: `0 errors`.

- [ ] **Step 3: Commit**

  ```bash
  git add frontend/src/components/SilenceEditor.svelte
  git commit -m "feat(silence-editor): show total matcher count in field label"
  ```

---

### Task 3: Pass revealed-range props from SilenceEditor to MatcherEditor

**Files:**
- Modify: `frontend/src/components/SilenceEditor.svelte`

- [ ] **Step 1: Update the `<MatcherEditor>` usage to pass the new props**

  Find:
  ```svelte
  <MatcherEditor bind:matchers={editorMatchers} source={alert.source}>
  ```
  Replace with:
  ```svelte
  <MatcherEditor
    bind:matchers={editorMatchers}
    source={alert.source}
    revealedAfterIndex={revealedAfterIndex}
    revealedCount={revealedCount}
  >
  ```

- [ ] **Step 2: Run type-check**

  ```bash
  cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
  ```
  Expected: a complaint that `revealedAfterIndex` and `revealedCount` are unknown props on `MatcherEditor` — that's expected; it will clear after Task 4.

- [ ] **Step 3: Commit**

  ```bash
  git add frontend/src/components/SilenceEditor.svelte
  git commit -m "feat(silence-editor): pass revealed-range props to MatcherEditor"
  ```

---

### Task 4: Accept props and add `isRevealed` helper in MatcherEditor

**Files:**
- Modify: `frontend/src/components/MatcherEditor.svelte`

- [ ] **Step 1: Add props after the existing `export let source` declaration**

  Find:
  ```ts
  export let source: string = '';
  ```
  Replace with:
  ```ts
  export let source: string = '';
  export let revealedAfterIndex: number | null = null;
  export let revealedCount: number = 0;
  ```

- [ ] **Step 2: Add the `isRevealed` helper after the `addBlank` function (end of the `<script>` block)**

  Find:
  ```ts
  function addBlank() {
    matchers = [...matchers, { name: '', value: '', isRegex: false, isEqual: true }];
  }
  ```
  Replace with:
  ```ts
  function addBlank() {
    matchers = [...matchers, { name: '', value: '', isRegex: false, isEqual: true }];
  }

  function isRevealed(i: number): boolean {
    if (revealedAfterIndex === null) return false;
    return i >= revealedAfterIndex && i < revealedAfterIndex + revealedCount;
  }
  ```

- [ ] **Step 3: Run type-check — should now be clean**

  ```bash
  cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
  ```
  Expected: `0 errors`.

- [ ] **Step 4: Commit**

  ```bash
  git add frontend/src/components/MatcherEditor.svelte
  git commit -m "feat(matcher-editor): add revealedAfterIndex/revealedCount props"
  ```

---

### Task 5: Render the persistent separator in MatcherEditor

**Files:**
- Modify: `frontend/src/components/MatcherEditor.svelte`

- [ ] **Step 1: Update the `{#each}` loop to inject the separator before the first revealed chip**

  Find:
  ```svelte
  {#each matchers as m, i (i)}
    {@const invalidRegex = !regexValid(m)}
    {@const invalidName = !m.name.trim()}
    {@const invalidValue = !m.value}
    <div class="chip" class:invalid={invalidRegex || invalidName || invalidValue}>
  ```
  Replace with:
  ```svelte
  {#each matchers as m, i (i)}
    {@const invalidRegex = !regexValid(m)}
    {@const invalidName = !m.name.trim()}
    {@const invalidValue = !m.value}
    {#if revealedAfterIndex !== null && i === revealedAfterIndex}
      <div class="revealed-separator" aria-hidden="true">
        <span>· · · more · · ·</span>
      </div>
    {/if}
    <div class="chip" class:invalid={invalidRegex || invalidName || invalidValue}>
  ```

- [ ] **Step 2: Add separator CSS at the end of the `<style>` block**

  Find the closing `</style>` tag and insert before it:
  ```css
  .revealed-separator {
    display: flex;
    align-items: center;
    gap: 6px;
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
  .revealed-separator span {
    color: #475569;
    font-size: 10px;
    letter-spacing: 0.05em;
    white-space: nowrap;
  }
  ```

- [ ] **Step 3: Run type-check**

  ```bash
  cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
  ```
  Expected: `0 errors`.

- [ ] **Step 4: Commit**

  ```bash
  git add frontend/src/components/MatcherEditor.svelte
  git commit -m "feat(matcher-editor): render persistent separator at revealed boundary"
  ```

---

### Task 6: Apply ephemeral chip tint to revealed matchers

**Files:**
- Modify: `frontend/src/components/MatcherEditor.svelte`

- [ ] **Step 1: Apply `was-collapsed` class to chips in the revealed range**

  Find:
  ```svelte
  <div class="chip" class:invalid={invalidRegex || invalidName || invalidValue}>
  ```
  Replace with:
  ```svelte
  <div class="chip" class:invalid={invalidRegex || invalidName || invalidValue} class:was-collapsed={isRevealed(i)}>
  ```

- [ ] **Step 2: Add the keyframe animation and chip modifier CSS**

  Inside the `<style>` block, before `</style>`, add:
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

  Note: `box-shadow: -2px 0 0 <color>` renders a left-side accent without changing the element's box model (unlike `border-left`), so there is no layout shift.

- [ ] **Step 3: Run type-check**

  ```bash
  cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
  ```
  Expected: `0 errors`.

- [ ] **Step 4: Commit**

  ```bash
  git add frontend/src/components/MatcherEditor.svelte
  git commit -m "feat(matcher-editor): ephemeral tint on revealed chips"
  ```

---

### Task 7: Smoke-test in the running app

**Files:** none (verification only)

- [ ] **Step 1: Build the frontend**

  ```bash
  cd frontend && npm run build 2>&1 | tail -20
  ```
  Expected: build succeeds, no errors.

- [ ] **Step 2: Run the app and open the silence editor**

  Start the app via the normal dev workflow (e.g. `wails dev` from the repo root, or open the built binary). Open any alert's silence editor. Confirm:

  - **Collapsed state:** "Matchers (N)" shows the correct total count, including hidden ones.
  - **Expand:** Click "▸ Show N more matchers". Matchers appear below the existing ones. The `· · · more · · ·` separator appears between the two groups. Revealed chips briefly show a blue left-border + tint that fades to normal within ~2.5 s.
  - **Separator persistence:** Wait 5 s after expanding. The separator is still visible; the chip tint is gone.
  - **Collapse:** Click "▾ Hide matchers". Separator disappears. Re-expand: tint re-triggers on the same chips.
  - **Re-open dialog:** Close and reopen. Separator is absent (starts collapsed).

- [ ] **Step 3: Commit any fixups found during smoke-test, then push**

  ```bash
  git push
  ```
