<script lang="ts">
  import { onMount, tick } from 'svelte';
  import {
    groupedAlerts,
    loading,
    error,
    displayConfig,
    verbose,
    sourcesHealth,
    onCallStatus,
    newAlertKeys,
    resolvedAlertKeys,
    acknowledgeAllAlerts,
    acknowledgeAllResolvedAlerts,
    refreshAlerts,
    loadDisplayConfig,
    loadSeverityConfig,
    initEventListeners,
    waitForBridge,
    activeSortMode,
    activeSortCriteria,
    activeGroupMode,
    activeGroupBy,
    SORT_PRESET_OPTIONS,
    GROUP_PRESET_OPTIONS,
    sortByCriteria,
    criteriaEqual,
    stringArrayEqual,
    isWails,
  } from '../stores/alerts';
  import { filteredAlerts, filter, availableSources } from '../stores/filter';
  import { severityConfig, severityLabel } from '../stores/severity';
  import { GetNotificationPermissionStatus, GetUIConfig, LayoutPopup, OpenNotificationSettings } from '../../wailsjs/go/main/App';
  import { Environment, EventsOn, ScreenGetAll } from '../../wailsjs/runtime/runtime';
  import AlertGroup from './AlertGroup.svelte';
  import AlertCard from './AlertCard.svelte';

  const popupHorizontalMargin = 8;
  const popupTopMargin = 0;
  const popupBottomMargin = 16;
  const minPopupHeight = 220;
  const popupHeightBuffer = 25;
  let mounted = false;
  let layoutQueued = false;
  let notificationPermissionStatus = '';
  let notificationSettingsError = '';
  let environmentPlatform = '';
  let environmentBuildType = '';

  async function syncEnvironmentInfo() {
    if (!isWails()) return;
    const environment = await Environment();
    environmentPlatform = environment.platform;
    environmentBuildType = environment.buildType;
  }

  async function syncNotificationPermissionStatus() {
    if (!isWails()) return;
    notificationPermissionStatus = await GetNotificationPermissionStatus();
  }

  onMount(() => {
    let disposePopupOpening = () => {};
    let disposeConfigReloaded = () => {};
    let disposed = false;
    mounted = true;

    const syncUIConfig = async () => {
      if (!isWails()) return;
      const uiConfig = await GetUIConfig();
      filter.update(current => ({
        ...current,
        showSilenced: uiConfig.show_silenced ?? current.showSilenced,
      }));
    };

    const init = async () => {
      await waitForBridge();
      if (disposed) return;

      initEventListeners();
      await Promise.all([
        loadDisplayConfig(),
        loadSeverityConfig(),
        syncUIConfig(),
        syncEnvironmentInfo(),
        syncNotificationPermissionStatus(),
      ]);
      await refreshAlerts();

      if (!isWails()) return;
      disposeConfigReloaded = EventsOn('config:reloaded', () => {
        void loadSeverityConfig();
        void syncUIConfig();
        void syncEnvironmentInfo();
        void syncNotificationPermissionStatus();
      });
      disposePopupOpening = EventsOn('popup:opening', async () => {
        await layoutPopup();
      });
    };

    void init();

    return () => {
      disposed = true;
      mounted = false;
      disposeConfigReloaded();
      disposePopupOpening();
    };
  });

  $: hasGroups = $activeGroupBy.length > 0;
  $: totalCount = $filteredAlerts.length;
  $: newVisibleCount = $filteredAlerts.filter(alert => $newAlertKeys.has(alert.source + ':' + alert.id)).length;
  $: resolvedVisibleCount = $filteredAlerts.filter(alert => $resolvedAlertKeys.has(alert.source + ':' + alert.id)).length;
  $: sortedUngroupedAlerts = [...$filteredAlerts].sort(sortByCriteria($activeSortCriteria));

  let refreshing = false;
  let sortMenuOpen = false;
  let groupMenuOpen = false;
  async function handleRefresh() {
    refreshing = true;
    await refreshAlerts();
    refreshing = false;
  }

  function setSortMode(mode: string) {
    activeSortMode.set(mode);
    sortMenuOpen = false;
  }

  function setGroupMode(mode: string) {
    activeGroupMode.set(mode);
    groupMenuOpen = false;
  }

  function closeSortMenu() {
    sortMenuOpen = false;
    groupMenuOpen = false;
  }

  $: noHealthYet = $sourcesHealth.length === 0;
  $: allSourcesOK = $sourcesHealth.length > 0 && $sourcesHealth.every(h => h.ok);
  $: anySourceFailing = $sourcesHealth.length > 0 && $sourcesHealth.some(h => !h.ok);
  $: normalizedBuildType = environmentBuildType.trim().toLowerCase();
  $: isMacOSDevMode = environmentPlatform === 'darwin' && (
    normalizedBuildType === 'dev' ||
    normalizedBuildType === 'development'
  );
  $: showNotificationInfoCard = !isMacOSDevMode && (
    notificationPermissionStatus === 'denied' ||
    notificationPermissionStatus === 'not_determined' ||
    notificationPermissionStatus === 'unsupported_legacy'
  );
  $: notificationInfoTitle = notificationPermissionStatus === 'denied'
    ? 'Notifications are configured, but currently blocked'
    : 'Notifications are configured, but not allowed yet';
  $: notificationInfoText = notificationPermissionStatus === 'denied'
    ? 'Foghorn is not allowed to show notifications in macOS Notification Center.'
    : notificationPermissionStatus === 'unsupported_legacy'
      ? 'This macOS version does not expose notification permission status directly. Open Notification settings and make sure Foghorn is allowed.'
      : 'macOS has not granted notification permission to Foghorn yet.';
  $: healthTitle = noHealthYet
    ? 'Waiting for first poll…'
    : ['Per-source status:', ...$sourcesHealth.map(formatHealthLine)].join('\n');
  $: latestPoll = $sourcesHealth.reduce((latest, h) => {
    const t = new Date(h.lastPoll);
    return t > latest ? t : latest;
  }, new Date(0));
  $: onCallSummary = $onCallStatus.map(status => {
    const names = status.users.map(user => user.name || user.email).filter(Boolean).join(', ') || 'nobody assigned';
    return $onCallStatus.length === 1 ? names : `${status.source}: ${names}`;
  }).join(' | ');
  $: onCallTitle = $onCallStatus.map(status => {
    const schedule = status.scheduleName || status.scheduleID;
    const team = status.teamName ? ` (${status.teamName})` : '';
    const names = status.users.map(user => user.email ? `${user.name} <${user.email}>` : user.name).join(', ') || 'nobody assigned';
    return `${status.source} · ${schedule}${team}: ${names}`;
  }).join('\n');
  $: popupLayoutSignature = [
    showNotificationInfoCard ? '1' : '0',
    notificationSettingsError,
    $loading ? '1' : '0',
    $error ?? '',
    totalCount,
    newVisibleCount,
    resolvedVisibleCount,
    hasGroups ? '1' : '0',
    $filteredAlerts.length,
    $verbose ? '1' : '0',
    $onCallStatus.length,
    onCallSummary,
    $activeGroupBy.join('|'),
  ].join('::');
  $: if (mounted && isWails() && popupLayoutSignature) {
    void scheduleLayoutPopup();
  }

  function scheduleLayoutPopup(): Promise<void> {
    if (layoutQueued) {
      return Promise.resolve();
    }
    layoutQueued = true;

    return new Promise(resolve => {
      requestAnimationFrame(async () => {
        layoutQueued = false;
        await layoutPopup();
        resolve();
      });
    });
  }

  function formatTime(d: Date): string {
    if (d.getTime() === 0) return '';
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function formatHealthLine(health: {
    source: string;
    ok: boolean;
    lastPoll: string;
    lastError?: string;
    consecFails: number;
  }): string {
    const status = health.ok ? 'OK' : 'Failing';
    const lastPoll = health.lastPoll ? formatTime(new Date(health.lastPoll)) : 'never';
    const error = health.ok ? '' : `; error: ${health.lastError || 'unknown error'}`;
    const failures = health.consecFails > 0 ? `; consecutive failures: ${health.consecFails}` : '';
    return `${health.source}: ${status}; last poll: ${lastPoll}${error}${failures}`;
  }

  function currentSortLabel(): string {
    const matchingPreset = SORT_PRESET_OPTIONS.find(option =>
      option.criteria && criteriaEqual(option.criteria, $activeSortCriteria)
    );
    if (matchingPreset) return matchingPreset.label;
    return $activeSortMode === 'default' ? 'Default' : 'Custom';
  }

  function currentGroupLabel(): string {
    const matchingPreset = GROUP_PRESET_OPTIONS.find(option =>
      option.fields && stringArrayEqual(option.fields, $activeGroupBy)
    );
    if (matchingPreset) return matchingPreset.label;
    return $activeGroupMode === 'default' ? 'Default' : 'Custom';
  }

  async function layoutPopup(): Promise<void> {
    await tick();
    await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));

    const [uiConfig, screens, environment] = await Promise.all([
      GetUIConfig(),
      ScreenGetAll(),
      Environment(),
    ]);

    const screen = screens.find(s => s.isCurrent) ?? screens.find(s => s.isPrimary) ?? screens[0];
    if (!screen) return;

    const width = clamp(
      uiConfig.popup_width || 800,
      360,
      Math.max(360, screen.width - (popupHorizontalMargin * 2)),
    );
    const maxHeight = Math.max(minPopupHeight, screen.height - popupTopMargin - popupBottomMargin);
    const desiredHeight = measureDesiredPopupHeight();
    const height = clamp(desiredHeight, minPopupHeight, maxHeight);

    const horizontalArg = environment.platform === 'darwin'
      ? popupHorizontalMargin
      : Math.max(0, screen.width - width - popupHorizontalMargin);

    await LayoutPopup(width, height, horizontalArg, popupTopMargin, popupBottomMargin);
  }

  function measureDesiredPopupHeight(): number {
    const container = document.querySelector('.alert-list-container') as HTMLElement | null;
    const alertsScroll = document.querySelector('.alerts-scroll') as HTMLElement | null;

    if (!container || !alertsScroll) {
      return window.innerHeight;
    }

    const chromeHeight = Array.from(container.children)
      .filter((element): element is HTMLElement => element instanceof HTMLElement && !element.classList.contains('alerts-scroll'))
      .reduce((total, element) => total + outerHeight(element), 0);
    const contentHeight = alertsScroll.scrollHeight;
    const borders = 8;

    return chromeHeight + contentHeight + borders + popupHeightBuffer;
  }

  function outerHeight(element: HTMLElement): number {
    const style = window.getComputedStyle(element);
    const marginTop = Number.parseFloat(style.marginTop) || 0;
    const marginBottom = Number.parseFloat(style.marginBottom) || 0;
    return element.offsetHeight + marginTop + marginBottom;
  }

  function clamp(value: number, min: number, max: number): number {
    return Math.min(Math.max(value, min), max);
  }

  async function handleOpenNotificationSettings() {
    notificationSettingsError = '';
    try {
      await OpenNotificationSettings();
    } catch (e) {
      notificationSettingsError = String(e);
    }
  }
</script>

<svelte:window on:click={closeSortMenu} />

<div class="alert-list-container">
  {#if showNotificationInfoCard}
    <div class="info-card info-card-warning">
      <div class="info-card-copy">
        <div class="info-card-title">{notificationInfoTitle}</div>
        <div class="info-card-text">{notificationInfoText}</div>
        {#if notificationSettingsError}
          <div class="info-card-error">{notificationSettingsError}</div>
        {/if}
      </div>
      <button class="info-card-action" on:click={handleOpenNotificationSettings}>
        Open Notification Settings
      </button>
    </div>
  {/if}

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
      {#each $severityConfig.levels as level}
        <option value={level.name}>{severityLabel(level.name)}</option>
      {/each}
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
      <input type="checkbox" bind:checked={$filter.showAll} />
      Show all
    </label>
    <label class="verbose-toggle">
      <input type="checkbox" bind:checked={$verbose} />
      Verbose
    </label>
  </div>

  <!-- Status bars -->
  <div class="status-stack">
    <div class="status-bar">
      <div class="status-left">
        {#if $loading}
          <span class="status-loading">Loading…</span>
        {:else if $error}
          <span class="status-error">Error: {$error}</span>
        {:else}
          <span class="status-count">{totalCount} alert{totalCount !== 1 ? 's' : ''}</span>
          {#if newVisibleCount > 0}
            <span class="status-new" title="New alerts stay highlighted until you hover them briefly.">
              {newVisibleCount} new
            </span>
            <button
              class="status-link-btn"
              on:click={acknowledgeAllAlerts}
              title="Mark all new alerts as seen"
            >
              Unmark all
            </button>
          {/if}
          {#if resolvedVisibleCount > 0}
            <span class="status-resolved" title="Resolved alerts stay visible for 30 seconds, or until you mark them seen.">
              {resolvedVisibleCount} resolved
            </span>
            <button
              class="status-link-btn status-link-btn-resolved"
              on:click={acknowledgeAllResolvedAlerts}
              title="Mark all resolved alerts as seen"
            >
              Mark resolved seen
            </button>
          {/if}
          <div class="sort-toggle-wrap">
            <button
              class="sort-toggle"
              class:active={groupMenuOpen}
              on:click|stopPropagation={() => {
                groupMenuOpen = !groupMenuOpen;
                sortMenuOpen = false;
              }}
              title="Change alert grouping"
            >
              Group: {currentGroupLabel()} ▾
            </button>
            {#if groupMenuOpen}
              <div class="sort-menu">
                {#each GROUP_PRESET_OPTIONS as option}
                  <button
                    class="sort-option"
                    class:selected={$activeGroupMode === option.mode}
                    on:click|stopPropagation={() => setGroupMode(option.mode)}
                  >
                    <span>{option.label}</span>
                    {#if $activeGroupMode === option.mode}
                      <span class="sort-check">✓</span>
                    {/if}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
          <div class="sort-toggle-wrap">
            <button
              class="sort-toggle"
              class:active={sortMenuOpen}
              on:click|stopPropagation={() => {
                sortMenuOpen = !sortMenuOpen;
                groupMenuOpen = false;
              }}
              title="Change alert sort order"
            >
              Sort: {currentSortLabel()} ▾
            </button>
            {#if sortMenuOpen}
              <div class="sort-menu">
                {#each SORT_PRESET_OPTIONS as option}
                  <button
                    class="sort-option"
                    class:selected={$activeSortMode === option.mode}
                    on:click|stopPropagation={() => setSortMode(option.mode)}
                  >
                    <span>{option.label}</span>
                    {#if $activeSortMode === option.mode}
                      <span class="sort-check">✓</span>
                    {/if}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
          {#if $verbose}<span class="status-verbose">VERBOSE</span>{/if}
        {/if}
      </div>
      <div class="status-right" title={refreshing ? 'Refreshing…' : healthTitle}>
        <span class="refresh-status"
          class:refresh-ok={allSourcesOK && !refreshing}
          class:refresh-fail={anySourceFailing && !refreshing}
          class:refresh-pending={noHealthYet || refreshing}
        >●</span>
        {#if !noHealthYet}
          <span class="refresh-time">{formatTime(latestPoll)}</span>
        {/if}
        <button class="refresh-btn" on:click={handleRefresh} disabled={refreshing} title={refreshing ? 'Refreshing…' : `Refresh alerts\n\n${healthTitle}`}>
          <span class="refresh-icon" class:spinning={refreshing}>↻</span>
        </button>
      </div>
    </div>
    {#if $onCallStatus.length > 0}
      <div class="status-bar status-bar-secondary">
        <div class="status-left">
          <span class="status-oncall-label">On call</span>
          <span class="status-oncall" title={onCallTitle}>{onCallSummary}</span>
        </div>
      </div>
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
      {#each $groupedAlerts as group}
        {@const visibleInGroup = group.alerts.filter(a => $filteredAlerts.find(f => f.source === a.source && f.id === a.id))}
        {#if visibleInGroup.length > 0}
          <AlertGroup
            groupParts={group.parts}
            alerts={visibleInGroup}
            config={$displayConfig}
            newKeys={$newAlertKeys}
            resolvedKeys={$resolvedAlertKeys}
          />
        {/if}
      {/each}
    {:else}
      {#each sortedUngroupedAlerts as alert (alert.source + ':' + alert.id)}
        <AlertCard
          {alert}
          config={$displayConfig}
          isNew={$newAlertKeys.has(alert.source + ':' + alert.id)}
          isResolved={$resolvedAlertKeys.has(alert.source + ':' + alert.id)}
        />
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

  .info-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin: 8px 8px 0;
    padding: 10px 12px;
    border-radius: 8px;
    border: 1px solid #7c2d12;
    background: linear-gradient(135deg, rgba(120, 53, 15, 0.22), rgba(30, 41, 59, 0.92));
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
  }

  .info-card-copy {
    min-width: 0;
  }

  .info-card-title {
    color: #fed7aa;
    font-size: 12px;
    font-weight: 700;
  }

  .info-card-text {
    color: #fdba74;
    font-size: 11px;
    margin-top: 2px;
  }

  .info-card-error {
    color: #fecaca;
    font-size: 11px;
    margin-top: 4px;
  }

  .info-card-action {
    flex-shrink: 0;
    border: 1px solid #fb923c;
    background: rgba(251, 146, 60, 0.12);
    color: #ffedd5;
    border-radius: 6px;
    padding: 6px 10px;
    font-size: 11px;
    cursor: pointer;
    white-space: nowrap;
  }

  .info-card-action:hover {
    background: rgba(251, 146, 60, 0.2);
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

  .status-stack {
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
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
  .status-bar-secondary {
    padding-top: 3px;
    padding-bottom: 3px;
    background: #0d1627;
    border-top: 1px solid rgba(51, 65, 85, 0.35);
  }
  .status-left { display: flex; align-items: center; gap: 6px; position: relative; }
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
    font-size: 10px;
    font-weight: 600;
    color: #f59e0b;
    text-transform: uppercase;
  }
  .status-new {
    font-size: 10px;
    font-weight: 700;
    color: #111827;
    background: #facc15;
    border-radius: 999px;
    padding: 2px 8px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    box-shadow: 0 0 14px rgba(250, 204, 21, 0.22);
  }
  .status-resolved {
    font-size: 10px;
    font-weight: 700;
    color: #dbeafe;
    background: #1d4ed8;
    border-radius: 999px;
    padding: 2px 8px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
  }
  .status-oncall-label {
    color: #7dd3fc;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .status-oncall {
    color: #cbd5e1;
    max-width: 640px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status-link-btn {
    background: none;
    border: none;
    color: #cbd5e1;
    cursor: pointer;
    font-size: 11px;
    padding: 0;
  }
  .status-link-btn:hover {
    color: #f8fafc;
    text-decoration: underline;
  }
  .sort-toggle-wrap {
    position: relative;
  }

  .sort-toggle {
    background: none;
    border: none;
    color: #94a3b8;
    cursor: pointer;
    font-size: 11px;
    padding: 0;
  }
  .sort-toggle:hover,
  .sort-toggle.active {
    color: #e2e8f0;
  }

  .sort-menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    min-width: 140px;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 6px;
    box-shadow: 0 12px 30px rgba(0, 0, 0, 0.35);
    padding: 4px;
    z-index: 10;
  }

  .sort-option {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    background: none;
    border: none;
    border-radius: 4px;
    color: #cbd5e1;
    cursor: pointer;
    font-size: 11px;
    padding: 6px 8px;
    text-align: left;
  }
  .sort-option:hover,
  .sort-option.selected {
    background: #1e293b;
  }

  .sort-check {
    color: #22c55e;
    font-weight: 700;
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
