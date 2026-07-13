<script lang="ts">
  import { createEventDispatcher, tick } from 'svelte';

  export let value: string = '';
  export let suggestions: string[] = [];
  export let placeholder: string = '';
  export let ariaLabel: string = '';
  export let invalid: boolean = false;

  const dispatch = createEventDispatcher<{ change: string }>();

  const MAX_SUGGESTIONS = 50;
  let focused = false;
  let highlighted = -1;
  let inputEl: HTMLInputElement | null = null;

  $: filtered = filterSuggestions(value, suggestions);

  function filterSuggestions(current: string, all: string[]): string[] {
    const q = (current || '').toLowerCase();
    if (!q) return all.slice(0, MAX_SUGGESTIONS);
    return all.filter((s) => s.toLowerCase().includes(q)).slice(0, MAX_SUGGESTIONS);
  }

  function onInput(e: Event) {
    value = (e.target as HTMLInputElement).value;
    highlighted = -1;
    dispatch('change', value);
  }

  async function pick(candidate: string) {
    value = candidate;
    dispatch('change', value);
    focused = false;
    await tick();
    inputEl?.blur();
  }

  function onKeydown(e: KeyboardEvent) {
    if (!focused) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (filtered.length === 0) return;
      highlighted = (highlighted + 1) % filtered.length;
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (filtered.length === 0) return;
      highlighted = (highlighted - 1 + filtered.length) % filtered.length;
    } else if (e.key === 'Enter') {
      if (highlighted >= 0 && highlighted < filtered.length) {
        e.preventDefault();
        void pick(filtered[highlighted]);
      }
    } else if (e.key === 'Escape') {
      focused = false;
    }
  }
</script>

<div class="autocomplete" class:invalid>
  <input
    bind:this={inputEl}
    class="input"
    type="text"
    aria-label={ariaLabel}
    {placeholder}
    {value}
    on:input={onInput}
    on:focus={() => (focused = true)}
    on:blur={() => setTimeout(() => (focused = false), 120)}
    on:keydown={onKeydown}
  />
  {#if focused && filtered.length > 0}
    <ul class="dropdown" role="listbox">
      {#each filtered as candidate, i}
        <li
          role="option"
          aria-selected={i === highlighted}
          class:highlighted={i === highlighted}
          on:mousedown|preventDefault={() => pick(candidate)}
        >
          {candidate}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .autocomplete {
    position: relative;
    display: inline-block;
    min-width: 0;
    width: 100%;
  }

  .input {
    background: var(--color-input-bg);
    border: 1px solid var(--color-control-border);
    border-radius: 3px;
    color: var(--color-text);
    font-size: calc(12px * var(--font-scale, 1));
    padding: 3px 6px;
    outline: none;
    width: 100%;
    box-sizing: border-box;
    font-family: monospace;
  }
  .input:focus { border-color: var(--color-focus); }
  .invalid .input { border-color: var(--color-danger); }

  .dropdown {
    position: absolute;
    top: calc(100% + 2px);
    left: 0;
    right: 0;
    z-index: 20;
    margin: 0;
    padding: 2px 0;
    list-style: none;
    background: var(--color-surface);
    border: 1px solid var(--color-control-border);
    border-radius: 3px;
    max-height: 180px;
    overflow-y: auto;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5);
  }
  .dropdown li {
    padding: 3px 8px;
    font-size: calc(12px * var(--font-scale, 1));
    color: var(--color-text);
    cursor: pointer;
    font-family: monospace;
  }
  .dropdown li.highlighted,
  .dropdown li:hover {
    background: var(--color-interactive-bg);
    color: var(--color-interactive-strong);
  }
</style>
