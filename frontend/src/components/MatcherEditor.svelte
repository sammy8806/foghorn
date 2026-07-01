<script lang="ts">
  import { alerts, labelNamesForSource, labelValuesForSource, type Matcher } from '../stores/alerts';
  import { parseMatcherBlock, valuesToRegexMatcher } from '../stores/matchers';
  import LabelAutocomplete from './LabelAutocomplete.svelte';

  export let matchers: Matcher[] = [];
  export let source: string = '';
  export let revealedAfterIndex: number | null = null;
  export let revealedCount: number = 0;

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

  function applyPaste() {
    const { matchers: parsed, skipped } = parseMatcherBlock(pasteText);
    if (parsed.length > 0) {
      matchers = [...matchers, ...parsed];
    }
    pasteNote = skipped > 0 ? `${skipped} line${skipped === 1 ? '' : 's'} skipped` : '';
    if (parsed.length > 0) {
      pasteText = '';
      showPaste = false;
    }
  }

  let showFromValues = false;
  let fvName = '';
  let fvSelected: Record<string, boolean> = {};

  $: fvValues = fvName ? labelValuesForSource(source, (void $alerts, fvName)) : [];

  function toggleFromValues() {
    showFromValues = !showFromValues;
    if (showFromValues) {
      fvName = '';
      fvSelected = {};
    }
  }

  function applyFromValues() {
    const chosen = fvValues.filter((v) => fvSelected[v]);
    if (fvName && chosen.length > 0) {
      matchers = [...matchers, valuesToRegexMatcher(fvName, chosen)];
    }
    showFromValues = false;
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
        <span>· · · more · · ·</span>
      </div>
    {/if}
    <div class="chip" class:invalid={invalidRegex || invalidName || invalidValue} class:was-collapsed={isRevealed(i)}>
      <div class="chip-field name">
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
      <div class="chip-field value">
        <LabelAutocomplete
          value={m.value}
          suggestions={valueSuggestions(m.name)}
          placeholder={m.isRegex ? 'regex' : 'value'}
          ariaLabel="Matcher value"
          invalid={invalidRegex || invalidValue}
          on:change={(e) => updateValue(i, e.detail)}
        />
      </div>
      <button class="remove" aria-label="Remove matcher" on:click={() => removeAt(i)}>✕</button>
      {#if invalidRegex}
        <span class="chip-error">invalid regex</span>
      {/if}
    </div>
  {/each}
  <div class="matcher-footer">
    <button class="add" type="button" on:click={addBlank}>+ Add matcher</button>
    <button class="add" type="button" on:click={() => (showPaste = !showPaste)}>Paste</button>
    <button class="add" type="button" on:click={toggleFromValues}>From values…</button>
    <slot name="actions" />
  </div>

  {#if showPaste}
    <div class="paste-panel">
      <textarea
        class="paste-input"
        bind:value={pasteText}
        rows="4"
        placeholder={'ns=prod\napp=~api.*\nseverity="critical"'}
      />
      {#if pasteNote}<span class="paste-note">{pasteNote}</span>{/if}
      <div class="paste-actions">
        <button class="add" type="button" on:click={applyPaste}>Add matchers</button>
        <button class="add" type="button" on:click={() => { showPaste = false; pasteText = ''; pasteNote = ''; }}>Cancel</button>
      </div>
    </div>
  {/if}

  {#if showFromValues}
    <div class="paste-panel">
      <LabelAutocomplete
        value={fvName}
        suggestions={nameSuggestions}
        placeholder="label name"
        ariaLabel="Regex-from-values label name"
        on:change={(e) => { fvName = e.detail; fvSelected = {}; }}
      />
      {#if fvName && fvValues.length > 0}
        <div class="fv-values">
          {#each fvValues as v}
            <label class="fv-value">
              <input type="checkbox" bind:checked={fvSelected[v]} /> {v}
            </label>
          {/each}
        </div>
      {:else if fvName}
        <span class="paste-note">No observed values for “{fvName}”.</span>
      {/if}
      <div class="paste-actions">
        <button class="add" type="button" on:click={applyFromValues}>Add regex matcher</button>
        <button class="add" type="button" on:click={() => (showFromValues = false)}>Cancel</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .matcher-editor {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .chip {
    display: grid;
    grid-template-columns: 1fr 56px 1fr auto;
    gap: 4px;
    align-items: center;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 3px;
    padding: 4px 6px;
  }
  .chip.invalid {
    border-color: #f87171;
  }
  .chip-field {
    min-width: 0;
  }
  .op {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 3px;
    color: #e2e8f0;
    font-size: calc(12px * var(--font-scale, 1));
    padding: 3px 4px;
    font-family: monospace;
    outline: none;
    text-align: center;
  }
  .op:focus { border-color: #3b82f6; }
  .remove {
    background: none;
    border: none;
    color: #64748b;
    font-size: calc(13px * var(--font-scale, 1));
    cursor: pointer;
    padding: 0 4px;
  }
  .remove:hover { color: #f87171; }
  .chip-error {
    grid-column: 1 / -1;
    color: #f87171;
    font-size: calc(10px * var(--font-scale, 1));
  }
  .matcher-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .add {
    align-self: flex-start;
    background: none;
    border: 1px dashed #334155;
    border-radius: 3px;
    color: #94a3b8;
    font-size: calc(11px * var(--font-scale, 1));
    padding: 3px 8px;
    cursor: pointer;
  }
  .add:hover {
    border-color: #3b82f6;
    color: #e2e8f0;
  }
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
    font-size: calc(10px * var(--font-scale, 1));
    letter-spacing: 0.05em;
    white-space: nowrap;
  }
  @keyframes revealed-fade {
    0%   { background: #1e3a5f; box-shadow: -2px 0 0 #3b82f6; }
    70%  { background: #1e3a5f; box-shadow: -2px 0 0 #3b82f6; }
    100% { background: #0f172a; box-shadow: none; }
  }

  .chip.was-collapsed {
    animation: revealed-fade 2.5s ease-out forwards;
  }

  .paste-panel {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 6px;
    padding: 8px;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 3px;
  }
  .paste-input {
    background: #0b1220;
    border: 1px solid #334155;
    border-radius: 3px;
    color: #e2e8f0;
    font-family: monospace;
    font-size: calc(12px * var(--font-scale, 1));
    padding: 6px;
    resize: vertical;
  }
  .paste-note { color: #fbbf24; font-size: calc(10px * var(--font-scale, 1)); }
  .paste-actions { display: flex; gap: 6px; }
  .fv-values {
    display: flex;
    flex-direction: column;
    gap: 3px;
    max-height: 160px;
    overflow-y: auto;
  }
  .fv-value {
    display: flex;
    align-items: center;
    gap: 6px;
    color: #cbd5e1;
    font-size: calc(12px * var(--font-scale, 1));
  }
</style>
