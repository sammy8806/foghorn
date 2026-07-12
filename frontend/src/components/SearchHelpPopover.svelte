<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let open = false;

  const dispatch = createEventDispatcher<{ close: void }>();

  function close() {
    dispatch('close');
  }
</script>

<svelte:window on:keydown={(e) => { if (open && e.key === 'Escape') close(); }} />

{#if open}
  <div class="help-backdrop" on:click={close} role="presentation"></div>
  <div class="help-popover" role="dialog" aria-modal="true" aria-labelledby="search-help-title">
    <div class="help-header">
      <h2 id="search-help-title">Search syntax</h2>
      <button class="help-close" type="button" aria-label="Close search help" on:click={close}>✕</button>
    </div>
    <p class="help-intro">Terms are ANDed together. Use these patterns in the filter bar:</p>
    <table class="help-table">
      <thead>
        <tr><th>Form</th><th>Meaning</th><th>Example</th></tr>
      </thead>
      <tbody>
        <tr><td><code>word</code></td><td>Free-text substring</td><td><code>database</code></td></tr>
        <tr><td><code>-word</code></td><td>Negated free text</td><td><code>-staging</code></td></tr>
        <tr><td><code>key=value</code></td><td>Label equals</td><td><code>severity=critical</code></td></tr>
        <tr><td><code>key!=value</code></td><td>Label not equals</td><td><code>team!=platform</code></td></tr>
        <tr><td><code>key=~regex</code></td><td>Label regex (anchored)</td><td><code>pod=~worker-.*</code></td></tr>
        <tr><td><code>annotation:key=…</code></td><td>Annotation matcher</td><td><code>annotation:runbook=~.*db.*</code></td></tr>
      </tbody>
    </table>
    <p class="help-note">The bell icon creates an Alertmanager silence from label matchers in your query. Free-text and annotation terms are dropped from the silence.</p>
  </div>
{/if}

<style>
  .help-backdrop {
    position: fixed;
    inset: 0;
    z-index: 40;
    background: rgba(2, 6, 23, 0.45);
  }

  .help-popover {
    position: fixed;
    top: 52px;
    left: 10px;
    right: 10px;
    z-index: 41;
    max-height: calc(100vh - 70px);
    overflow: auto;
    border: 1px solid #334155;
    border-radius: 8px;
    background: #1e293b;
    color: #e2e8f0;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.35);
    padding: 12px 14px;
  }

  .help-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  h2 {
    margin: 0;
    font-size: calc(13px * var(--font-scale, 1));
    font-weight: 700;
  }

  .help-close {
    border: none;
    background: transparent;
    color: #94a3b8;
    cursor: pointer;
    font-size: calc(14px * var(--font-scale, 1));
    padding: 2px 6px;
  }

  .help-intro,
  .help-note {
    margin: 8px 0 0;
    font-size: calc(11.5px * var(--font-scale, 1));
    color: #94a3b8;
    line-height: 1.45;
  }

  .help-table {
    width: 100%;
    margin-top: 10px;
    border-collapse: collapse;
    font-size: calc(11px * var(--font-scale, 1));
  }

  .help-table th,
  .help-table td {
    border-bottom: 1px solid #334155;
    padding: 6px 4px;
    text-align: left;
    vertical-align: top;
  }

  .help-table th {
    color: #cbd5e1;
    font-weight: 600;
  }

  code {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10.5px * var(--font-scale, 1));
    color: #bfdbfe;
  }
</style>
