<script lang="ts">
  import type { Alert, AlertFieldDisplay, DisplayConfig } from '../stores/alerts';
  import AlertCard from './AlertCard.svelte';
  import { severityColor } from '../utils/severity';

  export let groupParts: AlertFieldDisplay[] = [];
  export let alerts: Alert[];
  export let config: DisplayConfig;
  export let newKeys: Set<string> = new Set();

  $: maxSeverity = alerts.reduce((worst, a) => {
    const order = { critical: 0, warning: 1, info: 2 };
    return (order[a.severity] ?? 3) < (order[worst] ?? 3) ? a.severity : worst;
  }, 'info');

  let collapsed = false;
</script>

<div class="alert-group">
  <div class="group-header" on:click={() => (collapsed = !collapsed)} role="button" tabindex="0" on:keydown={e => e.key === 'Enter' && (collapsed = !collapsed)}>
    <span class="group-dot" style="background: {severityColor(maxSeverity)}" />
    <span class="group-name">
      {#if groupParts.length === 0}
        ungrouped
      {:else}
        {#each groupParts as part, index}
          <span class="group-part">
            {#if part.mode === 'both' && part.raw && part.resolved && part.raw !== part.resolved}
              <span>{part.raw}</span>
              <span class="group-resolved">({part.resolved})</span>
            {:else}
              <span>{part.text}</span>
            {/if}
          </span>
          {#if index < groupParts.length - 1}
            <span class="group-separator"> / </span>
          {/if}
        {/each}
      {/if}
    </span>
    <span class="group-count">{alerts.length}</span>
    <span class="chevron">{collapsed ? '▶' : '▼'}</span>
  </div>

  {#if !collapsed}
    <div class="group-alerts">
      {#each alerts as alert (alert.source + ':' + alert.id)}
        <AlertCard {alert} {config} isNew={newKeys.has(alert.source + ':' + alert.id)} />
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

  .group-resolved {
    color: #64748b;
    margin-left: 0.35rem;
    font-weight: 500;
    text-transform: none;
    letter-spacing: 0;
  }

  .group-separator {
    color: #475569;
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
