<script context="module" lang="ts">
  let nextSelectMenuId = 0;
</script>

<script lang="ts">
  import { createEventDispatcher, onDestroy, tick } from 'svelte';
  import { shouldDropUp } from '../utils/popover';

  export let value: string = '';
  export let options: { value: string; label: string }[] = [];
  export let ariaLabel: string = '';
  export let placeholder: string = '';
  /** Narrow, centred, monospace trigger — the matcher operator. */
  export let compact: boolean = false;

  const dispatch = createEventDispatcher<{ change: string }>();

  let open = false;
  let highlighted = -1;
  let measured = false;
  let dropUp = false;
  let wrapEl: HTMLDivElement;
  const id = `select-menu-${++nextSelectMenuId}`;
  const menuId = `${id}-listbox`;
  let triggerEl: HTMLDivElement;
  let menuEl: HTMLUListElement | null = null;

  $: selected = options.find((o) => o.value === value) ?? null;

  async function openMenu() {
    if (open) return;
    open = true;
    measured = false;
    highlighted = Math.max(0, options.findIndex((o) => o.value === value));
    document.addEventListener('mousedown', onDocMouseDown, true);
    await tick();
    dropUp = shouldDropUp(triggerEl, menuEl?.offsetHeight ?? 0);
    measured = true;
  }

  function closeMenu() {
    if (!open) return;
    open = false;
    measured = false;
    document.removeEventListener('mousedown', onDocMouseDown, true);
  }

  function onDocMouseDown(e: MouseEvent) {
    if (!wrapEl?.contains(e.target as Node)) closeMenu();
  }

  onDestroy(() => document.removeEventListener('mousedown', onDocMouseDown, true));

  function pick(v: string, restoreFocus = true) {
    value = v;
    dispatch('change', v);
    closeMenu();
    if (restoreFocus) triggerEl?.focus();
  }

  async function moveHighlight(next: number) {
    if (options.length === 0) return;
    highlighted = Math.max(0, Math.min(next, options.length - 1));
    await tick();
    document.getElementById(optionId(highlighted))?.scrollIntoView?.({ block: 'nearest' });
  }

  function optionId(index: number): string {
    return `${id}-option-${index}`;
  }

  function onKeydown(e: KeyboardEvent) {
    if (!open) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        void openMenu();
      }
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      void moveHighlight(highlighted + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      void moveHighlight(highlighted - 1);
    } else if (e.key === 'Home') {
      e.preventDefault();
      void moveHighlight(0);
    } else if (e.key === 'End') {
      e.preventDefault();
      void moveHighlight(options.length - 1);
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (highlighted >= 0 && highlighted < options.length) pick(options[highlighted].value);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      closeMenu();
    } else if (e.key === 'Tab') {
      if (highlighted >= 0 && highlighted < options.length) {
        pick(options[highlighted].value, false);
      } else {
        closeMenu();
      }
    }
  }
</script>

<div class="select-menu" class:compact bind:this={wrapEl}>
  <div
    class="trigger"
    class:open
    role="combobox"
    tabindex="0"
    aria-label={ariaLabel}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-controls={menuId}
    aria-activedescendant={open && highlighted >= 0 ? optionId(highlighted) : undefined}
    bind:this={triggerEl}
    on:click={() => (open ? closeMenu() : openMenu())}
    on:keydown={onKeydown}
  >
    <span class="trigger-label" class:placeholder={!selected}>{selected ? selected.label : placeholder}</span>
    {#if !compact}
      <svg class="chevron" viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="m7 15 5 5 5-5M7 9l5-5 5 5" />
      </svg>
    {/if}
  </div>

  {#if open}
    <ul
      id={menuId}
      class="menu"
      class:up={dropUp}
      role="listbox"
      aria-label={ariaLabel}
      bind:this={menuEl}
      style:visibility={measured ? 'visible' : 'hidden'}
    >
      {#each options as o, i}
        <li
          id={optionId(i)}
          role="option"
          aria-selected={i === highlighted}
          class:highlighted={i === highlighted}
          on:mousedown|preventDefault={() => pick(o.value)}
          on:mouseenter={() => (highlighted = i)}
        >
          <span class="check" aria-hidden="true">{o.value === value ? '✓' : ''}</span>
          <span class="option-label">{o.label}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .select-menu { position: relative; min-width: 0; }

  /* Matches the bare controls the grouped cards hold: no box of its own, the
     card supplies the frame. */
  .trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    width: 100%;
    box-sizing: border-box;
    height: 26px;
    padding: 0 9px;
    border: 1px solid transparent;
    border-radius: 6px;
    background: transparent;
    color: var(--ctrl-fg);
    font-family: inherit;
    font-size: calc(12.5px * var(--font-scale, 1));
    text-align: left;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
  }
  .trigger:hover { background: var(--chrome-ctrl-hover); }
  .trigger:focus-visible,
  .trigger.open {
    outline: none;
    border-color: rgba(96, 165, 250, 0.6);
    box-shadow: var(--focus-ring);
  }
  .trigger-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .trigger-label.placeholder { color: var(--group-placeholder); }
  .chevron { flex-shrink: 0; color: var(--ctrl-fg-dim); }

  /* The operator sits in a fixed grid cell, so it drops the chevron and keeps
     the quiet pill fill that marks it as the row's one popup. */
  .compact .trigger {
    /* Matches the matcher row's other two controls, which sit at the 24px
       target floor; a shorter pill between them read as a dropped baseline. */
    height: 24px;
    padding: 0;
    justify-content: center;
    border-radius: 5px;
    background: rgba(148, 163, 184, 0.1);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: calc(11.5px * var(--font-scale, 1));
    text-align: center;
  }
  .compact .trigger:hover { background: rgba(148, 163, 184, 0.18); }
  .compact .trigger-label { overflow: visible; }

  .menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    z-index: 30;
    min-width: 100%;
    margin: 0;
    padding: 4px;
    list-style: none;
    background: var(--popover-bg);
    -webkit-backdrop-filter: blur(20px) saturate(140%);
    backdrop-filter: blur(20px) saturate(140%);
    border: 1px solid var(--panel-border);
    border-radius: 8px;
    max-height: 220px;
    overflow-y: auto;
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.62), inset 0 1px 0 rgba(255, 255, 255, 0.07);
  }
  .menu.up { top: auto; bottom: calc(100% + 4px); }
  /* Menu rows stack edge to edge, so there is no clear space to expand a tap
     box into — the floor has to come out of real height. */
  .menu li {
    display: flex;
    align-items: center;
    min-height: 24px;
    box-sizing: border-box;
    gap: 6px;
    padding: 4px 8px 4px 5px;
    border-radius: 5px;
    font-size: calc(12px * var(--font-scale, 1));
    color: #cbd5e1;
    cursor: pointer;
    white-space: nowrap;
  }
  .menu li.highlighted { background: var(--accent); color: #fff; }
  .check {
    width: 11px;
    flex-shrink: 0;
    font-size: calc(10px * var(--font-scale, 1));
    text-align: center;
  }
  .compact .menu li { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
