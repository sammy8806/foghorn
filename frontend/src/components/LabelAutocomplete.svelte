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

  /* Bare field: the matcher row is already inside a hairline card, so drawing a
     box here would nest a border in a border in a border. Focus is the only
     state that gets a frame. */
  .input {
    width: 100%;
    box-sizing: border-box;
    height: 22px;
    padding: 0 6px;
    border: 1px solid transparent;
    border-radius: 5px;
    background: transparent;
    color: var(--ctrl-fg);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: calc(11.5px * var(--font-scale, 1));
    outline: none;
    transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
  }
  .input::placeholder { color: #566579; }
  .input:hover { background: rgba(148, 163, 184, 0.09); }
  .input:focus {
    background: var(--chrome-field-bg);
    border-color: rgba(96, 165, 250, 0.6);
    box-shadow: var(--focus-ring);
  }
  .invalid .input { color: #fca5a5; }
  .invalid .input:focus { border-color: rgba(248, 113, 113, 0.7); box-shadow: 0 0 0 3px rgba(248, 113, 113, 0.24); }

  .dropdown {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    z-index: 20;
    margin: 0;
    padding: 4px;
    list-style: none;
    background: var(--popover-bg);
    -webkit-backdrop-filter: blur(20px) saturate(140%);
    backdrop-filter: blur(20px) saturate(140%);
    border: 1px solid var(--panel-border);
    border-radius: 8px;
    max-height: 180px;
    overflow-y: auto;
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.62), inset 0 1px 0 rgba(255, 255, 255, 0.07);
  }
  .dropdown li {
    padding: 4px 7px;
    border-radius: 5px;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: calc(11.5px * var(--font-scale, 1));
    color: #cbd5e1;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dropdown li.highlighted,
  .dropdown li:hover {
    background: var(--accent);
    color: #fff;
  }
</style>
