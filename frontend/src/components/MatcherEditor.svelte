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
    <button class="add" type="button" on:click={togglePaste}>Paste</button>
    <slot name="actions" />
  </div>

  {#if showPaste}
    <div class="paste-panel">
      <textarea
        class="paste-input"
        bind:this={pasteTextarea}
        bind:value={pasteText}
        on:focus={() => (pasteFocused = true)}
        on:blur={() => (pasteFocused = false)}
        on:input={syncFromPasteText}
        rows="4"
        placeholder={'ns=prod\napp=~api.*\nseverity="critical"'}
      />
      {#if pasteNote}<span class="paste-note">{pasteNote}</span>{/if}
      <div class="paste-actions">
        <button class="add" type="button" on:click={copyPasteText}>Copy</button>
        <button class="add" type="button" on:click={() => { showPaste = false; pasteNote = ''; pasteParseError = false; pasteFocused = false; }}>Done</button>
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
    background: var(--color-input-bg);
    border: 1px solid var(--color-control-border);
    border-radius: 3px;
    padding: 4px 6px;
  }
  .chip.invalid {
    border-color: var(--color-danger);
  }
  .chip-field {
    min-width: 0;
  }
  .op {
    background: var(--color-input-bg);
    border: 1px solid var(--color-control-border);
    border-radius: 3px;
    color: var(--color-text);
    font-size: calc(12px * var(--font-scale, 1));
    padding: 3px 4px;
    font-family: monospace;
    outline: none;
    text-align: center;
  }
  .op:focus { border-color: var(--color-focus); }
  .remove {
    background: none;
    border: none;
    color: var(--color-text-faint);
    font-size: calc(13px * var(--font-scale, 1));
    cursor: pointer;
    padding: 0 4px;
  }
  .remove:hover { color: var(--color-danger); }
  .chip-error {
    grid-column: 1 / -1;
    color: var(--color-danger);
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
    border: 1px dashed var(--color-control-border);
    border-radius: 3px;
    color: var(--color-text-muted);
    font-size: calc(11px * var(--font-scale, 1));
    padding: 3px 8px;
    cursor: pointer;
  }
  .add:hover {
    border-color: var(--color-focus);
    color: var(--color-text);
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
    background: var(--color-border);
  }
  .revealed-separator span {
    color: var(--color-text-faint);
    font-size: calc(10px * var(--font-scale, 1));
    letter-spacing: 0.05em;
    white-space: nowrap;
  }
  @keyframes revealed-fade {
    0%   { background: var(--color-interactive-bg); box-shadow: -2px 0 0 var(--color-focus); }
    70%  { background: var(--color-interactive-bg); box-shadow: -2px 0 0 var(--color-focus); }
    100% { background: var(--color-input-bg); box-shadow: none; }
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
    background: var(--color-input-bg);
    border: 1px solid var(--color-control-border);
    border-radius: 3px;
  }
  .paste-input {
    background: var(--color-control-bg);
    border: 1px solid var(--color-control-border);
    border-radius: 3px;
    color: var(--color-text);
    font-family: monospace;
    font-size: calc(12px * var(--font-scale, 1));
    padding: 6px;
    resize: vertical;
  }
  .paste-note { color: var(--color-warning); font-size: calc(10px * var(--font-scale, 1)); }
  .paste-actions { display: flex; gap: 6px; }
</style>
