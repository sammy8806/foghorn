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
    loadSourceCapabilities,
    initEventListeners,
    waitForBridge,
    activeSortMode,
    activeSortCriteria,
    activeGroupMode,
    activeGroupBy,
    SORT_PRESET_OPTIONS,
    GROUP_PRESET_OPTIONS,
    sortByCriteria,
    sourceCapabilities,
    isWails,
  } from '../stores/alerts';
  import {
    filteredAlerts,
    filter,
    availableSources,
    parsedQuery,
    hiddenCount,
    showAllFilterState,
    hasContentFilters,
    type FilterState,
  } from '../stores/filter';
  import { queryToMatchers } from '../stores/query';
  import { severityConfig, severityLabel } from '../stores/severity';
  import { GetNotificationPermissionStatus, GetUIConfig, LayoutPopup, OpenNotificationSettings } from '../../wailsjs/go/main/App';
  import { Environment, EventsOn, ScreenGetAll } from '../../wailsjs/runtime/runtime';
  import AlertGroup from './AlertGroup.svelte';
  import AlertCard from './AlertCard.svelte';
  import SilenceEditor from './SilenceEditor.svelte';
  import SearchHelpPopover from './SearchHelpPopover.svelte';
  import { silenceEditor, closeSilenceEditor, openSilenceFromQuery } from '../stores/silenceEditor';
  import defaultIdleImage from '../assets/images/this-is-fine.webp';

  const popupHorizontalMargin = 8;
  const popupTopMargin = 0;
  const popupBottomMargin = 16;
  const minPopupHeight = 220;
  const popupHeightBuffer = 25;
  type PopupPosition = 'top_right' | 'top_left' | 'bottom_right' | 'bottom_left';
  let notificationPermissionStatus = '';
  let notificationSettingsError = '';
  let environmentPlatform = '';
  let environmentBuildType = '';
  let idleImage = defaultIdleImage;
  let healthBannerExpanded = false;

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

    const syncUIConfig = async () => {
      if (!isWails()) return;
      const uiConfig = await GetUIConfig();
      filter.update(current => ({
        ...current,
        showSilenced: uiConfig.show_silenced ?? current.showSilenced,
      }));
      idleImage = uiConfig.idle_image || defaultIdleImage;
    };

    const init = async () => {
      await waitForBridge();
      if (disposed) return;

      initEventListeners();
      await Promise.all([
        loadDisplayConfig(),
        loadSeverityConfig(),
        loadSourceCapabilities(),
        syncUIConfig(),
        syncEnvironmentInfo(),
        syncNotificationPermissionStatus(),
      ]);
      await refreshAlerts();

      if (!isWails()) return;
      disposeConfigReloaded = EventsOn('config:reloaded', async () => {
        await Promise.all([
          loadSeverityConfig(),
          loadSourceCapabilities(),
          syncUIConfig(),
          syncEnvironmentInfo(),
          syncNotificationPermissionStatus(),
        ]);
        await layoutPopup();
      });
      disposePopupOpening = EventsOn('popup:opening', async () => {
        await layoutPopup();
      });
    };

    void init();

    return () => {
      disposed = true;
      disposeConfigReloaded();
      disposePopupOpening();
    };
  });

  $: hasGroups = $activeGroupBy.length > 0;
  $: totalCount = $filteredAlerts.length;
  $: hiddenByFiltersCount = $hiddenCount;
  $: filtersHideAlerts = hiddenByFiltersCount > 0;
  $: hasExplicitContentFilters = hasContentFilters($filter);
  $: contentFiltersHideAlerts = hasExplicitContentFilters && filtersHideAlerts;
  $: showAllTitle = $filter.showAll
    ? 'Showing all alerts. Click to return to the default view.'
    : hiddenByFiltersCount > 0
      ? `Show all alerts (${hiddenByFiltersCount} hidden).`
      : 'Show all alerts';
  $: newVisibleCount = $filteredAlerts.filter(alert => $newAlertKeys.has(alert.source + ':' + alert.id)).length;
  $: resolvedVisibleCount = $filteredAlerts.filter(alert => $resolvedAlertKeys.has(alert.source + ':' + alert.id)).length;
  $: sortedUngroupedAlerts = [...$filteredAlerts].sort(sortByCriteria($activeSortCriteria));

  let refreshing = false;
  let sortMenuOpen = false;
  let groupMenuOpen = false;
  let severityMenuOpen = false;
  let sourceMenuOpen = false;
  let filtersBeforeShowAll: FilterState | null = null;
  let searchHelpOpen = false;

  // Expanding search: collapsed to a single icon; click expands into a field,
  // and it stays open while it holds text (collapses on blur when empty).
  let searchExpanded = false;
  let searchInputEl: HTMLInputElement | null = null;
  $: hasSearchText = $filter.text.trim().length > 0;
  $: searchOpen = searchExpanded || hasSearchText;
  $: silenceableMatchers = queryToMatchers($parsedQuery).matchers;
  $: hasSilenceableSources = Object.values($sourceCapabilities).some(c => c.supportsSilence);
  $: canOpenSilenceEditor = hasSilenceableSources;
  $: silenceFromSearchTitle = canOpenSilenceEditor
    ? silenceableMatchers.length > 0
      ? 'Create a silence from this search'
      : 'Create a new silence'
    : 'No configured source supports silences';

  function silenceFromSearch() {
    if (canOpenSilenceEditor) openSilenceFromQuery($parsedQuery);
  }

  function clearAllFilters() {
    filtersBeforeShowAll = null;
    filter.update(f => ({
      ...f,
      text: '',
      severity: 'all',
      source: 'all',
      showSilenced: true,
      showAll: false,
    }));
    searchExpanded = false;
  }

  function toggleShowAll() {
    filter.update(f => {
      if (f.showAll) {
        const previous = filtersBeforeShowAll;
        filtersBeforeShowAll = null;
        return previous ?? { ...f, showAll: false };
      }

      filtersBeforeShowAll = { ...f };
      return showAllFilterState(f);
    });
    searchExpanded = false;
  }
  // When the filter row would overflow, drop the segment values (captions only)
  // to keep it on a single line.
  // Each pass re-evaluates from the *expanded* layout (show values, measure,
  // then hide only if they actually overflow) so it never latches compact.
  let filterBarEl: HTMLElement | null = null;
  let widthCompact = false;
  let measuring = false;
  const SEARCH_ANIM_MS = 180;
  const COLLAPSED_SEARCH_WIDTH = 28;
  const EXPANDED_SEARCH_WIDTH = 200;
  let measureTimer: ReturnType<typeof setTimeout>;

  function fullFilterBarWouldOverflow(el: HTMLElement): boolean {
    const clone = el.cloneNode(true) as HTMLElement;
    clone.style.position = 'fixed';
    clone.style.left = '-10000px';
    clone.style.top = '0';
    clone.style.width = `${el.clientWidth}px`;
    clone.style.visibility = 'hidden';
    clone.style.pointerEvents = 'none';
    clone.querySelector('.view-block')?.classList.remove('compact');
    document.body.appendChild(clone);
    const overflow = clone.scrollWidth > clone.clientWidth + 1;
    clone.remove();
    return overflow;
  }

  function measureFilterBar() {
    const el = filterBarEl;
    if (measuring || !el) return;
    measuring = true;
    try {
      const overflow = fullFilterBarWouldOverflow(el);
      if (overflow !== widthCompact) widthCompact = overflow;
    } finally {
      measuring = false;
    }
  }

  function queueFilterBarMeasure() {
    void tick().then(() => {
      requestAnimationFrame(() => { void measureFilterBar(); });
    });
    clearTimeout(measureTimer);
    measureTimer = setTimeout(() => { void measureFilterBar(); }, SEARCH_ANIM_MS + 40);
  }

  // Re-measure when the available width changes (ResizeObserver), when a
  // segment is added/removed, or when search opens/collapses.
  $: { void $availableSources.length; void searchOpen; if (filterBarEl) queueFilterBarMeasure(); }
  onMount(() => {
    if (!filterBarEl || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => { queueFilterBarMeasure(); });
    observer.observe(filterBarEl);
    return () => observer.disconnect();
  });

  function onSearchTransitionEnd(e: TransitionEvent) {
    if (e.propertyName === 'width') {
      queueFilterBarMeasure();
    }
  }

  function compactBeforeSearchExpansion() {
    const el = filterBarEl;
    if (!el || searchOpen || widthCompact) return;
    const expandedSearchDelta = EXPANDED_SEARCH_WIDTH - COLLAPSED_SEARCH_WIDTH;
    if (el.scrollWidth + expandedSearchDelta > el.clientWidth + 1) {
      widthCompact = true;
    }
  }

  async function openSearch() {
    compactBeforeSearchExpansion();
    searchExpanded = true;
    await tick();
    searchInputEl?.focus();
  }

  function onSearchBlur() {
    if (!hasSearchText) searchExpanded = false;
  }

  async function clearSearch() {
    filter.update(f => ({ ...f, text: '' }));
    searchExpanded = true;
    await tick();
    searchInputEl?.focus();
  }

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

  function setSeverityFilter(value: string) {
    filter.update(f => ({ ...f, severity: value }));
    severityMenuOpen = false;
  }

  function setSourceFilter(value: string) {
    filter.update(f => ({ ...f, source: value }));
    sourceMenuOpen = false;
  }

  function closeAllMenus() {
    sortMenuOpen = false;
    groupMenuOpen = false;
    severityMenuOpen = false;
    sourceMenuOpen = false;
  }

  function openMenu(menu: 'severity' | 'source' | 'group' | 'sort') {
    severityMenuOpen = menu === 'severity' ? !severityMenuOpen : false;
    sourceMenuOpen = menu === 'source' ? !sourceMenuOpen : false;
    groupMenuOpen = menu === 'group' ? !groupMenuOpen : false;
    sortMenuOpen = menu === 'sort' ? !sortMenuOpen : false;
  }

  $: noHealthYet = $sourcesHealth.length === 0;
  $: allSourcesOK = $sourcesHealth.length > 0 && $sourcesHealth.every(h => h.ok);
  // A pending source (first poll still in flight) is neither OK nor failing, so
  // it must not turn the status bubble red.
  $: anySourcePending = $sourcesHealth.some(h => h.pending);
  $: failingSources = $sourcesHealth.filter(h => !h.ok && !h.pending);
  $: anySourceFailing = failingSources.length > 0;
  $: showHealthBanner = anySourceFailing && !$loading;
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

  function formatTime(d: Date): string {
    if (d.getTime() === 0) return '';
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function formatHealthLastPoll(health: { pending: boolean; lastPoll: string }): string {
    if (health.pending) return 'waiting for first poll';
    if (!health.lastPoll) return 'never polled';
    return `last poll ${formatTime(new Date(health.lastPoll))}`;
  }

  function formatHealthLine(health: {
    source: string;
    ok: boolean;
    pending: boolean;
    lastPoll: string;
    lastError?: string;
    consecFails: number;
  }): string {
    const status = health.pending ? 'Pending (waiting for first poll)' : health.ok ? 'OK' : 'Failing';
    const lastPoll = health.pending
      ? 'not yet'
      : health.lastPoll ? formatTime(new Date(health.lastPoll)) : 'never';
    const error = (health.ok || health.pending) ? '' : `; error: ${health.lastError || 'unknown error'}`;
    const failures = health.consecFails > 0 ? `; consecutive failures: ${health.consecFails}` : '';
    return `${health.source}: ${status}; last poll: ${lastPoll}${error}${failures}`;
  }

  $: currentSortLabel = SORT_PRESET_OPTIONS.find(o => o.mode === $activeSortMode)?.label ?? 'Custom';
  $: currentGroupLabel = GROUP_PRESET_OPTIONS.find(o => o.mode === $activeGroupMode)?.label ?? 'Custom';

  // The value column tracks the width of the *currently selected* label —
  // small by default, growing only when a longer value is actually picked —
  // clamped to each field's realistic min/max so the animation stays bounded
  // and a stray long value (e.g. a long source name) just ellipsizes instead
  // of blowing out the box.
  function widthChFor(text: string, minCh: number, maxCh: number): number {
    return Math.min(maxCh, Math.max(minCh, text.length)) + 0.6;
  }

  const groupLabelLengths = GROUP_PRESET_OPTIONS.map(o => o.label.length);
  const groupMinCh = Math.min(...groupLabelLengths);
  const groupMaxCh = Math.max(...groupLabelLengths);
  $: groupValueWidthCh = widthChFor(currentGroupLabel, groupMinCh, groupMaxCh);

  const sortLabelLengths = SORT_PRESET_OPTIONS.map(o => o.label.length);
  const sortMinCh = Math.min(...sortLabelLengths);
  const sortMaxCh = Math.max(...sortLabelLengths);
  $: sortValueWidthCh = widthChFor(currentSortLabel, sortMinCh, sortMaxCh);

  $: severityText = $filter.severity === 'all' ? 'All' : severityLabel($filter.severity);
  $: severityLabelLengths = ['All', ...$severityConfig.levels.map(l => severityLabel(l.name))].map(l => l.length);
  $: severityMinCh = Math.min(...severityLabelLengths);
  $: severityMaxCh = Math.max(...severityLabelLengths);
  $: severityValueWidthCh = widthChFor(severityText, severityMinCh, severityMaxCh);

  // Source names are free text (not a fixed option list), so cap the growth
  // instead of deriving a max from every possible value.
  const sourceMinCh = 'All'.length;
  const sourceMaxCh = 16;
  $: sourceText = $filter.source === 'all' ? 'All' : $filter.source;
  $: sourceValueWidthCh = widthChFor(sourceText, sourceMinCh, sourceMaxCh);

  async function layoutPopup(): Promise<void> {
    await tick();
    await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));

    const uiConfig = await GetUIConfig();
    if (uiConfig.auto_position === false) return;

    const screens = await ScreenGetAll();
    const screen = screens.find(s => s.isCurrent) ?? screens.find(s => s.isPrimary) ?? screens[0];
    if (!screen) return;

    const popupScale = uiConfig.scale?.mode === 'interface' && uiConfig.scale.apply_to_popup
      ? uiConfig.scale.factor || 1
      : 1;
    const width = clamp(
      Math.round((uiConfig.popup_width || 800) * popupScale),
      360,
      Math.max(360, screen.width - (popupHorizontalMargin * 2)),
    );
    const maxHeight = Math.max(minPopupHeight, screen.height - popupTopMargin - popupBottomMargin);
    const desiredHeight = measureDesiredPopupHeight();
    const height = clamp(Math.round(desiredHeight * popupScale), minPopupHeight, maxHeight);

    const popupPosition = normalizePopupPosition(uiConfig.popup_position);
    await LayoutPopup(width, height, popupHorizontalMargin, popupTopMargin, popupBottomMargin, popupPosition);
  }

  function normalizePopupPosition(position: string | undefined): PopupPosition {
    switch (position) {
      case 'top_left':
      case 'bottom_right':
      case 'bottom_left':
        return position;
      default:
        return 'top_right';
    }
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

<svelte:window on:click={closeAllMenus} />

<div class="alert-list-container">
  {#if showNotificationInfoCard}
    <div class="info-card info-card-warning">
      <div class="info-card-copy">
        <div class="info-card-title">{notificationInfoTitle}</div>
        <div class="info-card-text">{notificationInfoText}</div>
        {#if notificationSettingsError}
          <div class="info-card-detail-error">{notificationSettingsError}</div>
        {/if}
      </div>
      <button class="info-card-action" on:click={handleOpenNotificationSettings}>
        Open Notification Settings
      </button>
    </div>
  {/if}

  {#if showHealthBanner}
    <div class="health-banner" class:expanded={healthBannerExpanded} role="alert">
      <button
        class="health-banner-summary"
        on:click={() => healthBannerExpanded = !healthBannerExpanded}
        aria-expanded={healthBannerExpanded}
        aria-controls="health-banner-details"
      >
        <span class="health-banner-heading">
          <span>{failingSources.length === 1 ? 'Source polling failed' : `${failingSources.length} sources are failing`}</span>
        </span>
        <span class="health-banner-source-list">{failingSources.map(health => health.source).join(', ')}</span>
        <svg class="health-banner-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </button>
      <button class="health-banner-action" on:click={handleRefresh} disabled={refreshing}>
        {refreshing ? 'Retrying…' : 'Retry'}
      </button>
      {#if healthBannerExpanded}
        <div class="health-banner-sources" id="health-banner-details">
        {#each failingSources as health}
          <div class="health-banner-source">
            <div class="health-banner-source-title">
              <span class="health-banner-source-name">{health.source}</span>
              {#if health.consecFails > 1}<span class="health-banner-fail-count">{health.consecFails} consecutive failures</span>{/if}
            </div>
            <div class="health-banner-source-error">{health.lastError || 'Poll failed'}</div>
            <span class="health-banner-source-meta">
              {formatHealthLastPoll(health)}
            </span>
          </div>
        {/each}
      </div>
      {/if}
    </div>
  {/if}

  <!-- Filter & view controls -->
  <div class="filter-bar" bind:this={filterBarEl}>
    <!-- Expanding search: collapsed to an icon, click to expand. -->
    <div
      class="search"
      class:open={searchOpen}
      on:click={openSearch}
      on:keydown={(e) => { if (!searchOpen && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); openSearch(); } }}
      on:transitionend={onSearchTransitionEnd}
      role="button"
      aria-label="Filter alerts"
      tabindex={searchOpen ? -1 : 0}
    >
      <svg class="search-icon" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke-width="2.4" stroke-linecap="round"><circle cx="11" cy="11" r="7"></circle><line x1="21" y1="21" x2="16.5" y2="16.5"></line></svg>
      <input
        class="search-input"
        type="text"
        placeholder="Filter alerts…"
        bind:this={searchInputEl}
        bind:value={$filter.text}
        on:blur={onSearchBlur}
      />
      {#if hasSearchText}
        <button class="search-clear" title="Clear search" on:click|stopPropagation={clearSearch}>×</button>
      {/if}
    </div>

    <button
      class="icon-toggle search-help-btn"
      class:active={searchHelpOpen}
      on:click|stopPropagation={() => searchHelpOpen = !searchHelpOpen}
      title="Search syntax help"
      aria-label="Search syntax help"
      aria-expanded={searchHelpOpen}
    >?</button>

    <!-- Create a silence from the current search, or start one from scratch. -->
    <button
      class="icon-toggle"
      disabled={!canOpenSilenceEditor}
      on:click={silenceFromSearch}
      title={silenceFromSearchTitle}
      aria-label="Silence from search"
    >
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"></path><path d="M13.7 21a2 2 0 0 1-3.4 0"></path></svg>
    </button>

    <!-- Icon-button toggles -->
    <button
      class="icon-toggle"
      class:active={$filter.showAll}
      on:click={toggleShowAll}
      title={showAllTitle}
      aria-label={$filter.showAll
        ? 'Showing all alerts; return to default view'
        : hiddenByFiltersCount > 0
          ? `Show all alerts, ${hiddenByFiltersCount} hidden`
          : 'Show all alerts'}
    >
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7-11-7-11-7z"></path><circle cx="12" cy="12" r="3"></circle></svg>
      {#if hiddenByFiltersCount > 0}
        <span class="icon-toggle-badge">{hiddenByFiltersCount > 99 ? '99+' : hiddenByFiltersCount}</span>
      {/if}
    </button>
    <button
      class="icon-toggle"
      class:active={$verbose}
      on:click={() => verbose.update(v => !v)}
      title="Toggle verbose display"
    >
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="4" y1="7" x2="20" y2="7"></line><line x1="4" y1="12" x2="20" y2="12"></line><line x1="4" y1="17" x2="14" y2="17"></line></svg>
    </button>

    <div class="filter-spacer"></div>

    <!-- Fused view block: Severity · [Source] · Group · Sort -->
    <div class="view-block" class:compact={widthCompact}>
      <div class="segment-wrap">
        <button
          class="segment"
          class:active={severityMenuOpen}
          class:filtered={$filter.severity !== 'all'}
          on:click|stopPropagation={() => openMenu('severity')}
          title="Filter by severity"
        >
          <span class="segment-label">Severity</span>
          <span class="segment-value" style="--value-w: {severityValueWidthCh}">{severityText}</span>
          <svg class="segment-caret" width="9" height="9" viewBox="0 0 12 12"><path d="M2 4.5l4 4 4-4z"></path></svg>
        </button>
        {#if severityMenuOpen}
          <div class="filter-menu">
            <button class="filter-menu-option" class:selected={$filter.severity === 'all'} on:click|stopPropagation={() => setSeverityFilter('all')}>
              <span>All severities</span>
              {#if $filter.severity === 'all'}<span class="filter-menu-check">✓</span>{/if}
            </button>
            {#each $severityConfig.levels as level}
              <button class="filter-menu-option" class:selected={$filter.severity === level.name} on:click|stopPropagation={() => setSeverityFilter(level.name)}>
                <span>{severityLabel(level.name)}</span>
                {#if $filter.severity === level.name}<span class="filter-menu-check">✓</span>{/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      {#if $availableSources.length > 1}
        <div class="segment-wrap">
          <button
            class="segment"
            class:active={sourceMenuOpen}
            class:filtered={$filter.source !== 'all'}
            on:click|stopPropagation={() => openMenu('source')}
            title="Filter by source"
          >
            <span class="segment-label">Source</span>
            <span class="segment-value" style="--value-w: {sourceValueWidthCh}">{sourceText}</span>
            <svg class="segment-caret" width="9" height="9" viewBox="0 0 12 12"><path d="M2 4.5l4 4 4-4z"></path></svg>
          </button>
          {#if sourceMenuOpen}
            <div class="filter-menu">
              <button class="filter-menu-option" class:selected={$filter.source === 'all'} on:click|stopPropagation={() => setSourceFilter('all')}>
                <span>All sources</span>
                {#if $filter.source === 'all'}<span class="filter-menu-check">✓</span>{/if}
              </button>
              {#each $availableSources as src}
                <button class="filter-menu-option" class:selected={$filter.source === src} on:click|stopPropagation={() => setSourceFilter(src)}>
                  <span>{src}</span>
                  {#if $filter.source === src}<span class="filter-menu-check">✓</span>{/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      <div class="segment-wrap">
        <button
          class="segment"
          class:active={groupMenuOpen}
          class:filtered={$activeGroupMode !== 'default'}
          on:click|stopPropagation={() => openMenu('group')}
          title="Change alert grouping"
        >
          <span class="segment-label">Group</span>
          <span class="segment-value" style="--value-w: {groupValueWidthCh}">{currentGroupLabel}</span>
          <svg class="segment-caret" width="9" height="9" viewBox="0 0 12 12"><path d="M2 4.5l4 4 4-4z"></path></svg>
        </button>
        {#if groupMenuOpen}
          <div class="filter-menu">
            {#each GROUP_PRESET_OPTIONS as option}
              <button class="filter-menu-option" class:selected={$activeGroupMode === option.mode} on:click|stopPropagation={() => setGroupMode(option.mode)}>
                <span>{option.label}</span>
                {#if $activeGroupMode === option.mode}<span class="filter-menu-check">✓</span>{/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <div class="segment-wrap">
        <button
          class="segment"
          class:active={sortMenuOpen}
          class:filtered={$activeSortMode !== 'default'}
          on:click|stopPropagation={() => openMenu('sort')}
          title="Change alert sort order"
        >
          <span class="segment-label">Sort</span>
          <span class="segment-value" style="--value-w: {sortValueWidthCh}">{currentSortLabel}</span>
          <svg class="segment-caret" width="9" height="9" viewBox="0 0 12 12"><path d="M2 4.5l4 4 4-4z"></path></svg>
        </button>
        {#if sortMenuOpen}
          <div class="filter-menu">
            {#each SORT_PRESET_OPTIONS as option}
              <button class="filter-menu-option" class:selected={$activeSortMode === option.mode} on:click|stopPropagation={() => setSortMode(option.mode)}>
                <span>{option.label}</span>
                {#if $activeSortMode === option.mode}<span class="filter-menu-check">✓</span>{/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </div>

  <!-- Status bar -->
  <div class="status-bar">
    {#if $loading}
      <span class="status-loading">Loading…</span>
    {:else if $error}
      <span class="status-error">Error: {$error}</span>
    {:else}
      <span class="status-count">{totalCount} alert{totalCount !== 1 ? 's' : ''}</span>
      {#if newVisibleCount > 0}
        <button class="status-chip status-chip-new" title="New alerts stay highlighted until you hover them briefly. Click to mark all as seen." on:click={acknowledgeAllAlerts}>
          <span class="status-chip-x" aria-hidden="true">×</span>
          {newVisibleCount} New
        </button>
      {/if}
      {#if resolvedVisibleCount > 0}
        <button class="status-chip status-chip-resolved" title="Resolved alerts stay visible for 30 seconds, or until you mark them seen. Click to clear them now." on:click={acknowledgeAllResolvedAlerts}>
          <span class="status-chip-x" aria-hidden="true">×</span>
          {resolvedVisibleCount} Resolved
        </button>
      {/if}

      <div class="status-spacer"></div>

      {#if $onCallStatus.length > 0}
        <span class="status-oncall-label">On call</span>
        <span class="status-oncall" title={onCallTitle}>{onCallSummary}</span>
      {/if}
      <span class="refresh-status" title={refreshing ? 'Refreshing…' : healthTitle}
        class:refresh-ok={allSourcesOK && !refreshing}
        class:refresh-fail={anySourceFailing && !refreshing}
        class:refresh-pending={!anySourceFailing && (noHealthYet || refreshing || anySourcePending)}
      >●</span>
      <button class="refresh-btn" on:click={handleRefresh} disabled={refreshing} title={refreshing ? 'Refreshing…' : `Refresh alerts\n\n${healthTitle}`}>
        <svg class="refresh-icon" class:spinning={refreshing} viewBox="0 0 640 640" width="14" height="14" fill="currentColor">
          <path d="M129.9 292.5C143.2 199.5 223.3 128 320 128C373 128 421 149.5 455.8 184.2C456 184.4 456.2 184.6 456.4 184.8L464 192L416.1 192C398.4 192 384.1 206.3 384.1 224C384.1 241.7 398.4 256 416.1 256L544.1 256C561.8 256 576.1 241.7 576.1 224L576.1 96C576.1 78.3 561.8 64 544.1 64C526.4 64 512.1 78.3 512.1 96L512.1 149.4L500.8 138.7C454.5 92.6 390.5 64 320 64C191 64 84.3 159.4 66.6 283.5C64.1 301 76.2 317.2 93.7 319.7C111.2 322.2 127.4 310 129.9 292.6zM573.4 356.5C575.9 339 563.7 322.8 546.3 320.3C528.9 317.8 512.6 330 510.1 347.4C496.8 440.4 416.7 511.9 320 511.9C267 511.9 219 490.4 184.2 455.7C184 455.5 183.8 455.3 183.6 455.1L176 447.9L223.9 447.9C241.6 447.9 255.9 433.6 255.9 415.9C255.9 398.2 241.6 383.9 223.9 383.9L96 384C87.5 384 79.3 387.4 73.3 393.5C67.3 399.6 63.9 407.7 64 416.3L65 543.3C65.1 561 79.6 575.2 97.3 575C115 574.8 129.2 560.4 129 542.7L128.6 491.2L139.3 501.3C185.6 547.4 249.5 576 320 576C449 576 555.7 480.6 573.4 356.5z" />
        </svg>
      </button>
    {/if}
  </div>

  <!-- Alert content -->
  <div class="alerts-scroll">
    {#if $loading}
      <div class="empty-state">Loading alerts…</div>
    {:else if totalCount === 0}
      <div class="empty-state">
        {#if idleImage && !hasExplicitContentFilters}
          <img class="idle-image" src={idleImage} alt="No active alerts" />
        {/if}
        {#if contentFiltersHideAlerts}
          <p>No alerts match the current filters.</p>
          <p class="empty-state-hint">{hiddenByFiltersCount} alert{hiddenByFiltersCount !== 1 ? 's are' : ' is'} hidden.</p>
          <button class="empty-state-action" on:click={toggleShowAll}>Show all alerts</button>
        {:else if $filter.text.trim().length > 0}
          <p>No alerts match filter</p>
          <button class="empty-state-action" on:click={clearAllFilters}>Clear search</button>
        {:else}
          <p>No active alerts</p>
        {/if}
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

<SearchHelpPopover open={searchHelpOpen} on:close={() => searchHelpOpen = false} />

<!-- Single top-level silence editor, driven by the silenceEditor store. Lives
     outside the alert list so it stays open across refreshes/regrouping/sorting
     that destroy and recreate AlertCard instances. -->
<SilenceEditor
  alert={$silenceEditor.alert}
  silence={$silenceEditor.silence}
  mode={$silenceEditor.mode}
  open={$silenceEditor.open}
  query={$silenceEditor.query}
  seedMatchers={$silenceEditor.matchers}
  preferredSource={$silenceEditor.source}
  on:close={closeSilenceEditor}
  on:silenced={() => refreshAlerts()}
/>

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
    font-size: calc(12px * var(--font-scale, 1));
    font-weight: 700;
  }

  .info-card-text {
    color: #fdba74;
    font-size: calc(11px * var(--font-scale, 1));
    margin-top: 2px;
  }

  .info-card-detail-error {
    color: #fecaca;
    font-size: calc(11px * var(--font-scale, 1));
    margin-top: 4px;
  }

  .info-card-action {
    flex-shrink: 0;
    border: 1px solid #fb923c;
    background: rgba(251, 146, 60, 0.12);
    color: #ffedd5;
    border-radius: 6px;
    padding: 6px 10px;
    font-size: calc(11px * var(--font-scale, 1));
    cursor: pointer;
    white-space: nowrap;
  }

  .info-card-action:hover {
    background: rgba(251, 146, 60, 0.2);
  }

  .health-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 8px 8px 0;
    padding: 7px 9px;
    border: 1px solid #7f1d1d;
    border-radius: 6px;
    background: #1e1821;
  }

  .health-banner.expanded {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 7px 10px;
  }

  .health-banner-summary {
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

  .health-banner-summary:hover .health-banner-heading {
    color: #fff1f2;
  }

  .health-banner-heading {
    display: flex;
    align-items: center;
    flex-shrink: 0;
    color: #fecaca;
    font-size: calc(11px * var(--font-scale, 1));
    font-weight: 700;
  }

  .health-banner-source-list {
    min-width: 0;
    overflow: hidden;
    color: #fda4af;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10px * var(--font-scale, 1));
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .health-banner-chevron {
    flex-shrink: 0;
    color: #f87171;
    transition: transform 120ms ease;
  }

  .health-banner.expanded .health-banner-chevron {
    transform: rotate(180deg);
  }

  .health-banner-action {
    flex-shrink: 0;
    border: 0;
    border-radius: 4px;
    padding: 3px 5px;
    background: transparent;
    color: #fca5a5;
    font-size: calc(10px * var(--font-scale, 1));
    font-weight: 700;
    cursor: pointer;
    white-space: nowrap;
  }

  .health-banner-action:hover:not(:disabled) {
    color: #fff1f2;
    background: rgba(248, 113, 113, 0.16);
  }

  .health-banner-action:disabled {
    opacity: 0.55;
    cursor: default;
  }

  .health-banner-sources {
    grid-column: 1 / -1;
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 7px;
    border-top: 1px solid rgba(248, 113, 113, 0.2);
  }

  .health-banner-source {
    padding: 6px 7px;
    border-radius: 4px;
    background: rgba(15, 23, 42, 0.45);
    border: 1px solid rgba(248, 113, 113, 0.12);
    font-size: calc(10px * var(--font-scale, 1));
  }

  .health-banner-source-title {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
  }

  .health-banner-source-name {
    color: #fda4af;
    font-weight: 700;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .health-banner-fail-count {
    flex-shrink: 0;
    color: #94a3b8;
    font-size: calc(9px * var(--font-scale, 1));
  }

  .health-banner-source-error {
    color: #fca5a5;
    margin-top: 3px;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .health-banner-source-meta {
    display: block;
    color: #94a3b8;
    margin-top: 3px;
  }

  .filter-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    background: #0f172a;
    border-bottom: 1px solid #1b2740;
    flex-shrink: 0;
    /* Stay on one line; when it would overflow we strip the segment values
       (captions only) rather than wrapping onto a second row. */
    flex-wrap: nowrap;
  }

  .filter-spacer {
    flex: 1;
  }

  /* Expanding search: collapsed to a 28px icon button; expands to a field. */
  .search {
    display: flex;
    align-items: center;
    gap: 7px;
    height: 28px;
    width: 28px;
    box-sizing: border-box;
    padding: 0 0 0 7px;
    border-radius: 6px;
    border: 1px solid #2a3650;
    background: #162033;
    overflow: hidden;
    cursor: text;
    flex-shrink: 0;
    transition: width 0.18s cubic-bezier(0.2, 0, 0.2, 1), padding 0.18s;
  }
  .search.open {
    width: 200px;
    padding-right: 6px;
  }
  .search:not(.open) {
    justify-content: center;
    gap: 0;
    padding: 0;
    cursor: pointer;
  }
  .search-icon {
    stroke: #7c8aa3;
    flex-shrink: 0;
  }
  .search-input {
    flex: 1;
    min-width: 0;
    width: 0;
    border: none;
    background: transparent;
    outline: none;
    color: #e2e8f0;
    font-family: inherit;
    font-size: calc(12.5px * var(--font-scale, 1));
    padding: 0;
  }
  .search:not(.open) .search-input {
    /* Keep the input mounted (so bind:value works) but out of the icon box. */
    width: 0;
    flex: 0;
    padding: 0;
  }
  .search-input::placeholder { color: #5b6b83; }
  .search-clear {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: #2a3650;
    color: #cbd5e1;
    font-family: inherit;
    font-size: calc(12px * var(--font-scale, 1));
    line-height: 1;
    cursor: pointer;
  }
  .search-clear:hover { background: #34425f; color: #f1f5f9; }

  .search-help-btn {
    font-size: calc(13px * var(--font-scale, 1));
    font-weight: 700;
  }

  /* Square icon-button toggles (Show all, Verbose) */
  .icon-toggle {
    position: relative;
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    border: 1px solid #2a3650;
    background: #162033;
    color: #94a3b8;
    cursor: pointer;
    transition: all 0.15s;
  }
  .icon-toggle:hover {
    color: #dbe4f0;
    border-color: #3a496a;
  }
  .icon-toggle.active {
    color: #bcd9ff;
    background: rgba(47, 129, 247, 0.18);
    border-color: rgba(47, 129, 247, 0.45);
  }
  .icon-toggle:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .icon-toggle-badge {
    position: absolute;
    top: -5px;
    right: -5px;
    min-width: 14px;
    height: 14px;
    padding: 0 3px;
    border-radius: 7px;
    background: #3b82f6;
    color: #eff6ff;
    font-size: calc(9px * var(--font-scale, 1));
    font-weight: 700;
    line-height: 14px;
    text-align: center;
    pointer-events: none;
    box-shadow: 0 0 0 1px #0f172a;
  }

  /* Fused view block: Severity · [Source] · Group · Sort */
  .view-block {
    display: inline-flex;
    height: 28px;
    border: 1px solid #2a3650;
    border-radius: 6px;
    background: #162033;
    flex-shrink: 0;
  }
  .view-block.compact .segment {
    gap: 0;
  }
  .segment-wrap {
    position: relative;
    display: inline-flex;
  }
  .segment-wrap + .segment-wrap::before {
    content: '';
    width: 1px;
    background: #2a3650;
  }
  .segment {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 100%;
    padding: 0 10px;
    background: transparent;
    border: none;
    color: #dbe4f0;
    font-family: inherit;
    font-size: calc(12px * var(--font-scale, 1));
    font-weight: 600;
    cursor: pointer;
    transition: background 0.15s;
  }
  .segment-wrap:first-child .segment { border-radius: 5px 0 0 5px; }
  .segment-wrap:last-child .segment { border-radius: 0 5px 5px 0; }
  .segment:hover,
  .segment.active {
    background: rgba(47, 129, 247, 0.12);
    color: #f1f5f9;
  }
  .segment.filtered {
    background: rgba(47, 129, 247, 0.18);
    color: #bcd9ff;
  }
  .segment-label {
    font-size: calc(8.5px * var(--font-scale, 1));
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: #7c8aa3;
  }
  .segment.filtered .segment-label { color: #9fc2f5; }
  /* Width tracks the current value's length (via --value-w, in ch), clamped
     per-field to a realistic min/max — small by default, animating wider
     only when a longer value is actually selected. */
  .segment-value {
    color: #f1f5f9;
    display: inline-block;
    width: calc(var(--value-w, 7) * 1ch);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    opacity: 1;
    transition:
      width 0.18s cubic-bezier(0.2, 0, 0.2, 1),
      opacity 0.12s ease;
  }
  .view-block.compact .segment-value {
    width: 0;
    opacity: 0;
  }
  .segment.filtered .segment-value { color: #bcd9ff; }
  .segment-caret {
    fill: #64748b;
    width: 9px;
    opacity: 1;
    transition:
      width 0.18s cubic-bezier(0.2, 0, 0.2, 1),
      opacity 0.12s ease;
  }
  .view-block.compact .segment-caret {
    width: 0;
    opacity: 0;
  }
  /* Anchor dropdowns to the block's right edge so the rightmost (Sort) menu
     never overflows the popup's right edge. */
  .view-block .filter-menu {
    left: auto;
    right: 0;
  }

  .status-bar {
    --status-item-height: 20px;
    display: flex;
    align-items: center;
    align-content: center;
    gap: 8px;
    padding: 7px 10px;
    box-sizing: border-box;
    font-size: calc(11px * var(--font-scale, 1));
    line-height: 1.119;
    color: #475569;
    background: #0f172a;
    border-bottom: 1px solid #1e293b;
    flex-shrink: 0;
    /* Stay on one line: the on-call name truncates rather than wrapping the
       clock/refresh onto a second row. */
    flex-wrap: nowrap;
  }
  /* Everything on the status row keeps its size; only the on-call name gives. */
  .status-count,
  .status-chip,
  .status-oncall-label,
  .refresh-status,
  .refresh-btn {
    flex-shrink: 0;
  }
  .status-spacer {
    flex: 1;
  }

  .status-error { color: #ef4444; }
  .status-loading { color: #94a3b8; }
  .status-count {
    display: inline-flex;
    align-items: center;
    height: var(--status-item-height);
    min-height: var(--status-item-height);
    box-sizing: border-box;
    color: #e2e8f0;
    font-size: calc(12.5px * var(--font-scale, 1));
    font-weight: 600;
    white-space: nowrap;
  }
  /* Self-clearing status chips (New / Resolved) with a left-side × */
  .status-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 6px;
    border-radius: 10px;
    font-size: calc(10.5px * var(--font-scale, 1));
    line-height: 1;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    white-space: nowrap;
    border: none;
    font-family: inherit;
    cursor: pointer;
  }
  .status-chip:hover {
    filter: brightness(1.05);
  }
  .status-chip-x {
    display: inline-flex;
    align-items: center;
    padding: 0;
    font-size: calc(12px * var(--font-scale, 1));
    line-height: 1;
  }
  .status-chip-new {
    color: #1f2937;
    background: #facc15;
    box-shadow: 0 0 10px rgba(250, 204, 21, 0.28);
  }
  .status-chip-new .status-chip-x { color: #1f2937; }
  .status-chip-resolved {
    color: #052e16;
    background: #22c55e;
    box-shadow: 0 0 10px rgba(34, 197, 94, 0.24);
  }
  .status-chip-resolved .status-chip-x { color: #052e16; }
  .status-oncall-label {
    display: inline-flex;
    align-items: center;
    height: var(--status-item-height);
    min-height: var(--status-item-height);
    color: #5f9fd0;
    font-size: calc(9px * var(--font-scale, 1));
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    white-space: nowrap;
  }
  .status-oncall {
    display: inline-flex;
    align-items: center;
    height: var(--status-item-height);
    min-height: var(--status-item-height);
    color: #cbd5e1;
    font-size: calc(12px * var(--font-scale, 1));
    font-weight: 600;
    flex: 0 1 auto;
    min-width: 0;
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .filter-menu {
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

  .filter-menu-option {
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
    font-size: calc(11px * var(--font-scale, 1));
    padding: 6px 8px;
    text-align: left;
  }
  .filter-menu-option:hover,
  .filter-menu-option.selected {
    background: #1e293b;
  }

  .filter-menu-check {
    color: #22c55e;
    font-weight: 700;
  }

  .refresh-status {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: var(--status-item-height);
    min-height: var(--status-item-height);
    font-size: calc(9px * var(--font-scale, 1));
  }
  .refresh-ok { color: #22c55e; }
  .refresh-fail { color: #ef4444; }
  .refresh-pending { color: #f59e0b; }

  .refresh-btn {
    appearance: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    color: #94a3b8;
    font-size: calc(14px * var(--font-scale, 1));
    line-height: 1;
    height: var(--status-item-height);
    min-height: var(--status-item-height);
    padding: 0 2px;
    cursor: pointer;
  }
  .refresh-btn:hover { background: #1e293b; color: #e2e8f0; }
  .refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .refresh-icon { display: block; }
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
    font-size: calc(13px * var(--font-scale, 1));
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
  }

  .empty-state p {
    margin: 0;
  }

  .empty-state-hint {
    color: #64748b;
    font-size: calc(12px * var(--font-scale, 1));
  }

  .empty-state-action {
    margin-top: 4px;
    border: 1px solid #3b82f6;
    background: rgba(59, 130, 246, 0.12);
    color: #bfdbfe;
    border-radius: 6px;
    padding: 6px 12px;
    font-size: calc(12px * var(--font-scale, 1));
    cursor: pointer;
  }

  .empty-state-action:hover {
    background: rgba(59, 130, 246, 0.22);
  }

  .idle-image {
    max-width: 320px;
    max-height: 280px;
    border-radius: 8px;
    opacity: 0.85;
    user-select: none;
    -webkit-user-drag: none;
  }

</style>
