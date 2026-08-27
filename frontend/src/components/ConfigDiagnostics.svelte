<script lang="ts">
  import {
    dismissConfigDiagnostics,
    visibleConfigDiagnostics,
  } from '../stores/diagnostics';

  // Collapsed by default, same as the health banner: the summary line carries
  // what is wrong and where, the details are one click away.
  let expanded = false;
</script>

{#if $visibleConfigDiagnostics}
  <section class="diagnostics" role="alert" aria-labelledby="config-diagnostics-title" class:expanded>
    <button
      type="button"
      class="summary"
      on:click={() => (expanded = !expanded)}
      aria-expanded={expanded}
      aria-controls="config-diagnostics-details"
    >
      <span class="summary-heading">
        <strong id="config-diagnostics-title">Config problems</strong>
        <span class="count">{$visibleConfigDiagnostics.items.length}</span>
      </span>
      <span class="field-list">{$visibleConfigDiagnostics.items.map(item => item.field).join(', ')}</span>
      <svg class="chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <polyline points="6 9 12 15 18 9"></polyline>
      </svg>
    </button>
    <button type="button" class="action" on:click={dismissConfigDiagnostics}>Dismiss</button>
    {#if expanded}
      <div class="items" id="config-diagnostics-details">
        {#each $visibleConfigDiagnostics.items as item}
          <div class="item">
            <div class="item-title">
              <code>{item.field}</code>
              <small>{item.field === 'config' ? 'Configuration not loaded' : item.dropped ? 'Entry not loaded' : 'Using default'}</small>
            </div>
            <p>{item.message}</p>
          </div>
        {/each}
        {#if $visibleConfigDiagnostics.path}
          <footer>Edit <code>{$visibleConfigDiagnostics.path}</code> and save to recheck.</footer>
        {/if}
      </div>
    {/if}
  </section>
{/if}

<style>
  /* Same card language as the health banner in AlertList: an inset rounded
     card with a translucent severity tint, a one-line summary that expands
     into boxed detail rows, and a plain text action on the right. It renders
     in the same slot below the filter bar; the titlebar row is never its. */
  .diagnostics {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 8px 8px 0;
    padding: 7px 9px;
    border: 1px solid rgba(251, 191, 36, 0.35);
    border-radius: 6px;
    background: rgba(120, 53, 15, 0.28);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
  }

  .diagnostics.expanded {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 7px 10px;
  }

  .summary {
    display: flex;
    align-items: center;
    flex: 1;
    min-width: 0;
    gap: 6px;
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    cursor: pointer;
    text-align: left;
  }

  .summary:hover .summary-heading {
    color: #fffbeb;
  }

  .summary-heading {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    gap: 7px;
    color: #fde68a;
    font-size: calc(11px * var(--font-scale, 1));
  }

  .count {
    min-width: 20px;
    padding: 1px 6px;
    border-radius: 999px;
    text-align: center;
    background: rgba(251, 191, 36, 0.18);
    font-size: calc(11px * var(--font-scale, 1));
  }

  .field-list {
    min-width: 0;
    overflow: hidden;
    color: #fcd34d;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10px * var(--font-scale, 1));
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chevron {
    flex-shrink: 0;
    color: #fbbf24;
    transition: transform 120ms ease;
  }

  .diagnostics.expanded .chevron {
    transform: rotate(180deg);
  }

  .action {
    flex-shrink: 0;
    border: 0;
    border-radius: 4px;
    padding: 3px 5px;
    background: transparent;
    color: #fcd34d;
    font-size: calc(10px * var(--font-scale, 1));
    font-weight: 700;
    cursor: pointer;
    white-space: nowrap;
  }

  .action:hover {
    color: #fffbeb;
    background: rgba(251, 191, 36, 0.16);
  }

  .items {
    grid-column: 1 / -1;
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 7px;
    border-top: 1px solid rgba(251, 191, 36, 0.2);
  }

  .item {
    padding: 6px 7px;
    border-radius: 4px;
    background: rgba(15, 23, 42, 0.45);
    border: 1px solid rgba(251, 191, 36, 0.12);
    font-size: calc(10px * var(--font-scale, 1));
  }

  .item-title {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
  }

  .item-title code {
    min-width: 0;
    color: #fcd34d;
    font-weight: 700;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    overflow-wrap: anywhere;
  }

  .item-title small {
    flex-shrink: 0;
    color: #94a3b8;
    font-size: calc(9px * var(--font-scale, 1));
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .item p {
    margin: 3px 0 0;
    color: #fde68a;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  footer {
    color: #94a3b8;
    font-size: calc(10px * var(--font-scale, 1));
    overflow-wrap: anywhere;
  }

  footer code {
    color: #cbd5e1;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }
</style>
