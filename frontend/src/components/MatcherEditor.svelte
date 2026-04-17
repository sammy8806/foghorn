<script lang="ts">
  import { alerts, labelNamesForSource, labelValuesForSource, type Matcher } from '../stores/alerts';
  import LabelAutocomplete from './LabelAutocomplete.svelte';

  export let matchers: Matcher[] = [];
  export let source: string = '';

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
</script>

<div class="matcher-editor">
  {#each matchers as m, i (i)}
    {@const invalidRegex = !regexValid(m)}
    {@const invalidName = !m.name.trim()}
    {@const invalidValue = !m.value}
    <div class="chip" class:invalid={invalidRegex || invalidName || invalidValue}>
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
  <button class="add" type="button" on:click={addBlank}>+ Add matcher</button>
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
    font-size: 12px;
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
    font-size: 13px;
    cursor: pointer;
    padding: 0 4px;
  }
  .remove:hover { color: #f87171; }
  .chip-error {
    grid-column: 1 / -1;
    color: #f87171;
    font-size: 10px;
  }
  .add {
    align-self: flex-start;
    background: none;
    border: 1px dashed #334155;
    border-radius: 3px;
    color: #94a3b8;
    font-size: 11px;
    padding: 3px 8px;
    cursor: pointer;
  }
  .add:hover {
    border-color: #3b82f6;
    color: #e2e8f0;
  }
</style>
