<script lang="ts">
  import { onMount } from 'svelte';
  import { groupedAlerts, loading, error, displayConfig, verbose, sourcesHealth, newAlertKeys, refreshAlerts, loadDisplayConfig, initEventListeners, waitForBridge } from '../stores/alerts';
  import { filteredAlerts, filter, availableSources } from '../stores/filter';
  import AlertGroup from './AlertGroup.svelte';
  import AlertCard from './AlertCard.svelte';

  onMount(async () => {
    await waitForBridge();
    initEventListeners();
    await loadDisplayConfig();
    await refreshAlerts();
  });

  $: hasGroups = $displayConfig.group_by?.length > 0;
  $: totalCount = $filteredAlerts.length;

  let refreshing = false;
  async function handleRefresh() {
    refreshing = true;
    await refreshAlerts();
    refreshing = false;
  }

  $: noHealthYet = $sourcesHealth.length === 0;
  $: allSourcesOK = $sourcesHealth.length > 0 && $sourcesHealth.every(h => h.ok);
  $: anySourceFailing = $sourcesHealth.length > 0 && $sourcesHealth.some(h => !h.ok);
  $: healthTitle = noHealthYet
    ? 'Waiting for first poll…'
    : $sourcesHealth.map(h =>
        `${h.source}: ${h.ok ? 'OK' : h.lastError || 'failing'}${h.consecFails > 0 ? ` (${h.consecFails} consecutive failures)` : ''}`
      ).join('\n');
  $: latestPoll = $sourcesHealth.reduce((latest, h) => {
    const t = new Date(h.lastPoll);
    return t > latest ? t : latest;
  }, new Date(0));

  function formatTime(d: Date): string {
    if (d.getTime() === 0) return '';
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }
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
    <label class="verbose-toggle">
      <input type="checkbox" bind:checked={$verbose} />
      Verbose
    </label>
  </div>

  <!-- Status bar -->
  <div class="status-bar">
    <div class="status-left">
      {#if $loading}
        <span class="status-loading">Loading…</span>
      {:else if $error}
        <span class="status-error">Error: {$error}</span>
      {:else}
        <span class="status-count">{totalCount} alert{totalCount !== 1 ? 's' : ''}</span>
        {#if $verbose}<span class="status-verbose">VERBOSE</span>{/if}
      {/if}
    </div>
    <div class="status-right">
      <span class="refresh-status"
        class:refresh-ok={allSourcesOK && !refreshing}
        class:refresh-fail={anySourceFailing && !refreshing}
        class:refresh-pending={noHealthYet || refreshing}
        title={refreshing ? 'Refreshing…' : healthTitle}>●</span>
      {#if !noHealthYet}
        <span class="refresh-time">{formatTime(latestPoll)}</span>
      {/if}
      <button class="refresh-btn" on:click={handleRefresh} disabled={refreshing} title="Refresh alerts">
        <span class="refresh-icon" class:spinning={refreshing}>↻</span>
      </button>
    </div>
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
          <AlertGroup {groupName} alerts={visibleInGroup} config={$displayConfig} newKeys={$newAlertKeys} />
        {/if}
      {/each}
    {:else}
      {#each $filteredAlerts as alert (alert.source + ':' + alert.id)}
        <AlertCard {alert} config={$displayConfig} isNew={$newAlertKeys.has(alert.source + ':' + alert.id)} />
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
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 10px;
    font-size: 11px;
    color: #475569;
    background: #0a1120;
    border-bottom: 1px solid #1e293b;
    flex-shrink: 0;
  }
  .status-left { display: flex; align-items: center; }
  .status-right { display: flex; align-items: center; gap: 6px; }

  .verbose-toggle {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: #94a3b8;
    cursor: pointer;
    white-space: nowrap;
    user-select: none;
  }
  .verbose-toggle input { cursor: pointer; }

  .status-error { color: #ef4444; }
  .status-loading { color: #94a3b8; }
  .status-verbose {
    margin-left: 8px;
    font-size: 10px;
    font-weight: 600;
    color: #f59e0b;
    text-transform: uppercase;
  }

  .refresh-status { font-size: 9px; }
  .refresh-ok { color: #22c55e; }
  .refresh-fail { color: #ef4444; }
  .refresh-pending { color: #f59e0b; }
  .refresh-time { color: #475569; font-size: 10px; }

  .refresh-btn {
    background: none;
    border: 1px solid #334155;
    border-radius: 4px;
    color: #94a3b8;
    font-size: 14px;
    line-height: 1;
    padding: 1px 5px;
    cursor: pointer;
  }
  .refresh-btn:hover { background: #1e293b; color: #e2e8f0; }
  .refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .refresh-icon { display: inline-block; }
  .spinning { animation: spin 0.6s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

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
