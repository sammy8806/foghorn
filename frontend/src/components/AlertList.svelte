<script lang="ts">
  import { onMount } from 'svelte';
  import { groupedAlerts, loading, error, displayConfig, refreshAlerts, loadDisplayConfig, initEventListeners } from '../stores/alerts';
  import { filteredAlerts, filter, availableSources } from '../stores/filter';
  import AlertGroup from './AlertGroup.svelte';
  import AlertCard from './AlertCard.svelte';

  onMount(async () => {
    initEventListeners();
    await loadDisplayConfig();
    await refreshAlerts();
  });

  $: hasGroups = $displayConfig.group_by?.length > 0;
  $: totalCount = $filteredAlerts.length;
</script>

<div class="alert-list-container">
  <!-- Filter bar -->
  <div class="filter-bar">
    <input
      class="filter-input"
      type="search"
      placeholder="Filter alerts…"
      bind:value={$filter.text}
    />
    <select class="filter-select" bind:value={$filter.severity}>
      <option value="all">All severities</option>
      <option value="critical">Critical</option>
      <option value="warning">Warning</option>
      <option value="info">Info</option>
    </select>
    {#if $availableSources.length > 1}
      <select class="filter-select" bind:value={$filter.source}>
        <option value="all">All sources</option>
        {#each $availableSources as src}
          <option value={src}>{src}</option>
        {/each}
      </select>
    {/if}
  </div>

  <!-- Status bar -->
  <div class="status-bar">
    {#if $loading}
      <span class="status-loading">Loading…</span>
    {:else if $error}
      <span class="status-error">Error: {$error}</span>
    {:else}
      <span class="status-count">{totalCount} alert{totalCount !== 1 ? 's' : ''}</span>
    {/if}
  </div>

  <!-- Alert content -->
  <div class="alerts-scroll">
    {#if $loading}
      <div class="empty-state">Loading alerts…</div>
    {:else if totalCount === 0}
      <div class="empty-state">
        {$filteredAlerts.length === 0 && $filter.text ? 'No alerts match filter' : 'No active alerts'}
      </div>
    {:else if hasGroups}
      {#each Object.entries($groupedAlerts) as [groupName, groupAlerts]}
        {@const visibleInGroup = groupAlerts.filter(a => $filteredAlerts.find(f => f.source === a.source && f.id === a.id))}
        {#if visibleInGroup.length > 0}
          <AlertGroup {groupName} alerts={visibleInGroup} config={$displayConfig} />
        {/if}
      {/each}
    {:else}
      {#each $filteredAlerts as alert (alert.source + ':' + alert.id)}
        <AlertCard {alert} config={$displayConfig} />
      {/each}
    {/if}
  </div>
</div>

<style>
  .alert-list-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: hidden;
  }

  .filter-bar {
    display: flex;
    gap: 6px;
    padding: 8px;
    background: #0f172a;
    border-bottom: 1px solid #1e293b;
    flex-shrink: 0;
  }

  .filter-input {
    flex: 1;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 4px;
    color: #e2e8f0;
    font-size: 12px;
    padding: 5px 10px;
    outline: none;
  }
  .filter-input:focus { border-color: #3b82f6; }

  .filter-select {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 4px;
    color: #e2e8f0;
    font-size: 12px;
    padding: 5px 8px;
    outline: none;
    cursor: pointer;
  }

  .status-bar {
    padding: 4px 10px;
    font-size: 11px;
    color: #475569;
    background: #0a1120;
    border-bottom: 1px solid #1e293b;
    flex-shrink: 0;
  }

  .status-error { color: #ef4444; }
  .status-loading { color: #94a3b8; }

  .alerts-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .empty-state {
    text-align: center;
    color: #475569;
    padding: 40px 20px;
    font-size: 13px;
  }
</style>
