<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { alerts, labelNamesForSource, labelValuesForSource, type Matcher } from '../stores/alerts';
  import { formatMatcherBlock, parseMatcherBlock } from '../stores/matchers';
  import LabelAutocomplete from './LabelAutocomplete.svelte';

  export let matchers: Matcher[] = [];
  export let textMatchers: Matcher[] = matchers;
  export let source: string = '';
  export let revealedAfterIndex: number | null = null;
  export let revealedCount: number = 0;

  const dispatch = createEventDispatcher<{ replaceAll: Matcher[] }>();

  type Op = '=' | '!=' | '=~' | '!~';
  const OPS: Op[] = ['=', '!=', '=~', '!~'];

  function toOp(m: Matcher): Op {
    if (m.isRegex && m.isEqual) return '=~';
    if (m.isRegex && !m.isEqual) return '!~';
    if (!m.isRegex && m.isEqual) return '=';
    return '!=';
  }

  function fromOp(op: Op): { isRegex: boolean; isEqual: boolean } {
    switch (op) {
      case '=':  return { isRegex: false, isEqual: true };
      case '!=': return { isRegex: false, isEqual: false };
      case '=~': return { isRegex: true,  isEqual: true };
      case '!~': return { isRegex: true,  isEqual: false };
    }
  }

  function regexValid(m: Matcher): boolean {
    if (!m.isRegex) return true;
    try {
      new RegExp(m.value);
      return true;
    } catch {
      return false;
    }
  }

  // Reactively recompute label names when alerts update or the source changes.
  // The `$alerts` access (via `void`) registers the store as a reactive
  // dependency so suggestion lists refresh whenever the alerts store changes.
  $: nameSuggestions = (void $alerts, source) ? labelNamesForSource(source) : [];

  function valueSuggestions(name: string): string[] {
    // Access $alerts to re-evaluate when alerts change.
    void $alerts;
    return name ? labelValuesForSource(source, name) : [];
  }

  function updateName(i: number, name: string) {
    matchers = matchers.map((m, idx) => (idx === i ? { ...m, name } : m));
  }
  function updateValue(i: number, value: string) {
    matchers = matchers.map((m, idx) => (idx === i ? { ...m, value } : m));
  }
  function updateOp(i: number, op: Op) {
    const { isRegex, isEqual } = fromOp(op);
    matchers = matchers.map((m, idx) => (idx === i ? { ...m, isRegex, isEqual } : m));
  }
  function onOpChange(i: number, e: Event) {
    const raw = (e.currentTarget as HTMLSelectElement).value;
    updateOp(i, raw as Op);
  }
  function removeAt(i: number) {
    matchers = matchers.filter((_, idx) => idx !== i);
  }
  function addBlank() {
    matchers = [...matchers, { name: '', value: '', isRegex: false, isEqual: true }];
  }

  let showPaste = false;
  let pasteText = '';
  let pasteNote = '';
  let pasteParseError = false;
  let pasteFocused = false;
  let pasteTextarea: HTMLTextAreaElement | null = null;

  function togglePaste() {
    showPaste = !showPaste;
    if (showPaste) {
      pasteText = formatMatcherBlock(textMatchers);
      pasteNote = '';
      pasteParseError = false;
    }
  }

  function syncFromPasteText() {
    const { matchers: parsed, skipped } = parseMatcherBlock(pasteText);
    if (skipped > 0) {
      pasteNote = `${skipped} line${skipped === 1 ? '' : 's'} skipped`;
      pasteParseError = true;
      return;
    }
    dispatch('replaceAll', parsed);
    pasteNote = pasteText.trim() ? `${parsed.length} matcher${parsed.length === 1 ? '' : 's'}` : '';
    pasteParseError = false;
  }

  async function copyPasteText() {
    try {
      await navigator.clipboard.writeText(pasteText);
      pasteNote = 'Copied';
    } catch {
      pasteTextarea?.select();
      pasteNote = 'Text selected';
    }
  }

  $: formattedMatchers = formatMatcherBlock(textMatchers);
  $: if (showPaste && !pasteFocused && !pasteParseError && pasteText !== formattedMatchers) {
    pasteText = formattedMatchers;
  }

  function isRevealed(i: number): boolean {
    if (revealedAfterIndex === null) return false;
    return i >= revealedAfterIndex && i < revealedAfterIndex + revealedCount;
  }
</script>

<div class="matcher-editor">
  {#each matchers as m, i (i)}
    {@const invalidRegex = !regexValid(m)}
    {@const invalidName = !m.name.trim()}
    {@const invalidValue = !m.value}
    {#if revealedAfterIndex !== null && i === revealedAfterIndex}
      <div class="revealed-separator" aria-hidden="true">
        <span>more</span>
      </div>
    {/if}
    <div class="row" class:invalid={invalidRegex || invalidName || invalidValue} class:was-collapsed={isRevealed(i)}>
      <div class="cell">
        <LabelAutocomplete
          value={m.name}
          suggestions={nameSuggestions}
          placeholder="name"
          ariaLabel="Matcher name"
          invalid={invalidName}
          on:change={(e) => updateName(i, e.detail)}
        />
      </div>
      <select
        class="op"
        aria-label="Matcher operator"
        value={toOp(m)}
        on:change={(e) => onOpChange(i, e)}
      >
        {#each OPS as op}
          <option value={op}>{op}</option>
        {/each}
      </select>
      <div class="cell">
        <LabelAutocomplete
          value={m.value}
          suggestions={valueSuggestions(m.name)}
          placeholder={m.isRegex ? 'regex' : 'value'}
          ariaLabel="Matcher value"
          invalid={invalidRegex || invalidValue}
          on:change={(e) => updateValue(i, e.detail)}
        />
      </div>
      <button class="remove" aria-label="Remove matcher" on:click={() => removeAt(i)}>
        <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round">
          <path d="M18 6 6 18M6 6l12 12" />
        </svg>
      </button>
      {#if invalidRegex}
        <span class="row-error">invalid regex</span>
      {/if}
    </div>
  {/each}

  <div class="row footer-row">
    <div class="footer-actions">
      <button class="ghost" type="button" on:click={addBlank}>
        <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true">
          <path d="M12 5v14M5 12h14" />
        </svg>
        Add matcher
      </button>
      <button class="ghost" type="button" class:on={showPaste} on:click={togglePaste}>Paste</button>
    </div>
    <slot name="actions" />
  </div>

  {#if showPaste}
    <div class="row paste-row">
      <textarea
        class="paste-input"
        aria-label="Matchers as text"
        bind:this={pasteTextarea}
        bind:value={pasteText}
        on:focus={() => (pasteFocused = true)}
        on:blur={() => (pasteFocused = false)}
        on:input={syncFromPasteText}
        rows="4"
        placeholder={'ns=prod\napp=~api.*\nseverity="critical"'}
      />
      <div class="paste-actions">
        {#if pasteNote}<span class="paste-note" class:error={pasteParseError}>{pasteNote}</span>{/if}
        <button class="ghost" type="button" on:click={copyPasteText}>Copy</button>
        <button class="ghost" type="button" on:click={() => { showPaste = false; pasteNote = ''; pasteParseError = false; pasteFocused = false; }}>Done</button>
      </div>
    </div>
  {/if}
</div>

<style>
  /* One hairline card whose rows are split by a fainter divider — the same
     grouped-section shape the rest of the dialog uses. Each matcher used to
     draw its own border, which turned a five-matcher silence into five stacked
     boxes inside a sixth. */
  .matcher-editor {
    display: flex;
    flex-direction: column;
    background: var(--group-bg);
    border: 1px solid var(--chrome-hairline);
    border-radius: 9px;
    overflow: hidden;
  }
  .row {
    display: grid;
    /* Values run longer than label names, and giving them the wider share also
       pulls the operator in off the middle of the row so each matcher reads as
       one phrase rather than three scattered columns. */
    grid-template-columns: minmax(0, 0.78fr) 44px minmax(0, 1.22fr) 22px;
    gap: 5px;
    align-items: center;
    padding: 5px 7px 5px 9px;
    transition: background 0.15s;
  }
  .row + .row,
  .revealed-separator + .row { border-top: 1px solid var(--group-divider); }
  .row.invalid { background: rgba(248, 113, 113, 0.07); }
  .cell { min-width: 0; }

  /* Bare popup: the row is the control's frame, so the operator draws no box
     of its own until it's focused. */
  .op {
    -webkit-appearance: none;
    appearance: none;
    height: 22px;
    padding: 0;
    border: 1px solid transparent;
    border-radius: 5px;
    background: rgba(148, 163, 184, 0.1);
    color: var(--ctrl-fg);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: calc(11.5px * var(--font-scale, 1));
    text-align: center;
    text-align-last: center;
    outline: none;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
  }
  .op:hover { background: rgba(148, 163, 184, 0.18); }
  .op:focus {
    border-color: rgba(96, 165, 250, 0.6);
    box-shadow: var(--focus-ring);
  }

  /* Quiet until the row is under the pointer: eight of these at full strength
     read as a column of delete buttons rather than a list of matchers. */
  .remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: transparent;
    color: #64748b;
    cursor: pointer;
    opacity: 0.5;
    transition: opacity 0.15s, background 0.15s, color 0.15s;
  }
  .row:hover .remove { opacity: 1; }
  .remove:hover {
    background: rgba(248, 113, 113, 0.16);
    color: var(--danger);
  }
  .remove:focus-visible {
    opacity: 1;
    outline: none;
    box-shadow: var(--focus-ring);
  }
  .row-error {
    grid-column: 1 / -1;
    padding-top: 1px;
    color: var(--danger);
    font-size: calc(10px * var(--font-scale, 1));
  }

  .footer-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 9px;
  }
  .footer-actions { display: flex; align-items: center; gap: 5px; }
  .ghost {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 22px;
    padding: 0 8px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--ctrl-fg-dim);
    font-family: inherit;
    font-size: calc(11px * var(--font-scale, 1));
    cursor: pointer;
    white-space: nowrap;
    transition: background 0.15s, color 0.15s;
  }
  .ghost:hover { background: var(--chrome-ctrl-hover); color: var(--ctrl-fg); }
  .ghost.on { background: var(--chrome-ctrl-on); color: var(--ctrl-fg-active); }
  .ghost:focus-visible { outline: none; box-shadow: var(--focus-ring); }

  .revealed-separator {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 3px 9px;
    border-top: 1px solid var(--group-divider);
    user-select: none;
  }
  .revealed-separator::before,
  .revealed-separator::after {
    content: '';
    flex: 1;
    height: 1px;
    background: var(--group-divider);
  }
  .revealed-separator span {
    color: #5b6b83;
    font-size: calc(9.5px * var(--font-scale, 1));
    letter-spacing: 0.08em;
    text-transform: uppercase;
    white-space: nowrap;
  }

  /* Rows that were just revealed glow briefly, then settle into the card. The
     inset bar (rather than an outer box-shadow) keeps the highlight inside the
     card's clipped corners. */
  @keyframes revealed-fade {
    0%   { background: rgba(59, 130, 246, 0.22); box-shadow: inset 2px 0 0 var(--accent); }
    70%  { background: rgba(59, 130, 246, 0.22); box-shadow: inset 2px 0 0 var(--accent); }
    100% { background: transparent; box-shadow: inset 2px 0 0 transparent; }
  }
  .row.was-collapsed { animation: revealed-fade 2.5s ease-out forwards; }
  @media (prefers-reduced-motion: reduce) {
    .row.was-collapsed { animation: none; }
  }

  .paste-row {
    display: flex;
    flex-direction: column;
    /* .row centres its children; as a column that would shrink-wrap the action
       strip and park it mid-row instead of at the trailing edge. */
    align-items: stretch;
    gap: 6px;
    padding: 8px 9px;
    border-top: 1px solid var(--group-divider);
  }
  .paste-input {
    width: 100%;
    box-sizing: border-box;
    padding: 7px 9px;
    border: 1px solid var(--chrome-field-border);
    border-radius: 7px;
    background: var(--chrome-field-bg);
    color: var(--ctrl-fg);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: calc(11.5px * var(--font-scale, 1));
    line-height: 1.5;
    resize: vertical;
    outline: none;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  .paste-input::placeholder { color: #5b6b83; }
  .paste-input:focus {
    border-color: rgba(96, 165, 250, 0.6);
    box-shadow: var(--focus-ring);
  }
  .paste-actions { display: flex; align-items: center; justify-content: flex-end; gap: 5px; }
  .paste-note {
    margin-right: auto;
    color: var(--ctrl-fg-dim);
    font-size: calc(10.5px * var(--font-scale, 1));
  }
  .paste-note.error { color: #fbbf24; }
</style>
