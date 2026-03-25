<script lang="ts">
  import type { Alert, DisplayConfig } from '../stores/alerts';
  import AlertCard from './AlertCard.svelte';
  import { severityColor } from '../utils/severity';

  export let groupName: string;
  export let alerts: Alert[];
  export let config: DisplayConfig;

  $: maxSeverity = alerts.reduce((worst, a) => {
    const order = { critical: 0, warning: 1, info: 2 };
    return (order[a.severity] ?? 3) < (order[worst] ?? 3) ? a.severity : worst;
  }, 'info');

  let collapsed = false;
</script>

<div class="alert-group">
  <div class="group-header" on:click={() => (collapsed = !collapsed)} role="button" tabindex="0" on:keydown={e => e.key === 'Enter' && (collapsed = !collapsed)}>
    <span class="group-dot" style="background: {severityColor(maxSeverity)}" />
    <span class="group-name">{groupName}</span>
    <span class="group-count">{alerts.length}</span>
    <span class="chevron">{collapsed ? '▶' : '▼'}</span>
  </div>

  {#if !collapsed}
    <div class="group-alerts">
      {#each alerts as alert (alert.source + ':' + alert.id)}
        <AlertCard {alert} {config} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .alert-group {
    margin-bottom: 8px;
  }

  .group-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    background: rgba(255,255,255,0.04);
    border-radius: 4px;
    cursor: pointer;
    user-select: none;
    margin-bottom: 4px;
  }
  .group-header:hover { background: rgba(255,255,255,0.07); }

  .group-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .group-name {
    font-size: 12px;
    font-weight: 600;
    color: #94a3b8;
    flex: 1;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .group-count {
    font-size: 11px;
    background: #1e293b;
    padding: 1px 7px;
    border-radius: 10px;
    color: #64748b;
  }

  .chevron { font-size: 10px; color: #475569; }

  .group-alerts { padding-left: 8px; }
</style>
