<script lang="ts">
  import {
    dismissConfigDiagnostics,
    visibleConfigDiagnostics,
  } from '../stores/diagnostics';
  import Notice from './Notice.svelte';

  function rowTag(item: { field: string; dropped: boolean }): string {
    if (item.field === 'config') return 'Configuration not loaded';
    return item.dropped ? 'Entry not loaded' : 'Using default';
  }
</script>

{#if $visibleConfigDiagnostics}
  <Notice
    severity="caution"
    title="Config problems"
    count={$visibleConfigDiagnostics.items.length}
    subtitle={$visibleConfigDiagnostics.items.map(item => item.field).join(', ')}
    collapsible
  >
    <button slot="action" type="button" class="notice-action" on:click={dismissConfigDiagnostics}>
      Dismiss
    </button>

    {#each $visibleConfigDiagnostics.items as item}
      <div class="notice-row">
        <div class="notice-row-head">
          <span class="notice-row-name">{item.field}</span>
          <span class="notice-row-tag">{rowTag(item)}</span>
        </div>
        <p class="notice-row-text">{item.message}</p>
      </div>
    {/each}
    {#if $visibleConfigDiagnostics.path}
      <div class="notice-footer">Edit <code>{$visibleConfigDiagnostics.path}</code> and save to recheck.</div>
    {/if}
  </Notice>
{/if}
