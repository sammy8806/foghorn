<script lang="ts">
  import type { Alert, AlertFieldDisplay, DisplayConfig, Matcher } from '../stores/alerts';
  import { sourceCapabilities } from '../stores/alerts';
  import { openSilenceFromMatchers } from '../stores/silenceEditor';
  import AlertCard from './AlertCard.svelte';
  import { severityColor, severityOrder } from '../utils/severity';

  export let groupParts: AlertFieldDisplay[] = [];
  export let alerts: Alert[];
  export let config: DisplayConfig;
  export let newKeys: Set<string> = new Set();
  export let resolvedKeys: Set<string> = new Set();

  $: maxSeverity = alerts.reduce((worst, a) => {
    return severityOrder(a.severity) < severityOrder(worst) ? a.severity : worst;
  }, alerts[0]?.severity ?? 'unknown');

  let collapsed = false;
  let menuOpen = false;
  let menuX = 0;
  let menuY = 0;

  $: groupMatchers = matchersFromGroupParts(groupParts);
  $: preferredSource = sourceForGroup(groupParts, alerts);
  $: canSilenceGroup = alerts.some((alert) => !!$sourceCapabilities[alert.source]?.supportsSilence);

  function matchersFromGroupParts(parts: AlertFieldDisplay[]): Matcher[] {
    const seen = new Set<string>();
    const matchers: Matcher[] = [];
    for (const part of parts) {
      if (part.kind !== 'label' || !part.raw) continue;
      const key = `${part.name}\0${part.raw}`;
      if (seen.has(key)) continue;
      seen.add(key);
      matchers.push({
        name: part.name,
        value: part.raw,
        isRegex: false,
        isEqual: true,
      });
    }
    return matchers;
  }

  function sourceForGroup(parts: AlertFieldDisplay[], groupAlerts: Alert[]): string | null {
    const sourcePart = parts.find((part) => part.kind === 'field' && part.name === 'source');
    if (sourcePart?.raw && $sourceCapabilities[sourcePart.raw]?.supportsSilence) {
      return sourcePart.raw;
    }

    const sources = [...new Set(groupAlerts.map((alert) => alert.source))]
      .filter((source) => !!$sourceCapabilities[source]?.supportsSilence);
    return sources.length === 1 ? sources[0] : null;
  }

  function openMenu(e: MouseEvent) {
    e.preventDefault();
    menuX = e.clientX;
    menuY = e.clientY;
    menuOpen = true;
  }

  function closeMenu() {
    menuOpen = false;
  }

  function silenceGroup() {
    if (!canSilenceGroup) return;
    openSilenceFromMatchers(groupMatchers, preferredSource);
    closeMenu();
  }
</script>

<svelte:window on:click={closeMenu} on:keydown={(e) => { if (e.key === 'Escape') closeMenu(); }} />

<div class="alert-group">
  <div
    class="group-header"
    on:click={() => (collapsed = !collapsed)}
    on:contextmenu={openMenu}
    role="button"
    tabindex="0"
    on:keydown={e => e.key === 'Enter' && (collapsed = !collapsed)}
  >
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

  {#if menuOpen}
    <div
      class="group-menu"
      style:left="{menuX}px"
      style:top="{menuY}px"
      on:click|stopPropagation
      on:keydown={(e) => { if (e.key === 'Escape') closeMenu(); }}
      role="menu"
      tabindex="-1"
    >
      <button class="group-menu-item" role="menuitem" disabled={!canSilenceGroup} on:click={silenceGroup}>
        Silence group…
      </button>
    </div>
  {/if}

  {#if !collapsed}
    <div class="group-alerts">
      {#each alerts as alert (alert.source + ':' + alert.id)}
        <AlertCard
          {alert}
          {config}
          isNew={newKeys.has(alert.source + ':' + alert.id)}
          isResolved={resolvedKeys.has(alert.source + ':' + alert.id)}
        />
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
    font-size: calc(12px * var(--font-scale, 1));
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
    font-size: calc(11px * var(--font-scale, 1));
    background: #1e293b;
    padding: 1px 7px;
    border-radius: 10px;
    color: #64748b;
  }

  .chevron { font-size: calc(10px * var(--font-scale, 1)); color: #475569; }

  .group-alerts { padding-left: 8px; }

  .group-menu {
    position: fixed;
    z-index: 1100;
    min-width: 150px;
    padding: 4px;
    background: #111827;
    border: 1px solid #334155;
    border-radius: 6px;
    box-shadow: 0 12px 28px rgba(0, 0, 0, 0.38);
  }

  .group-menu-item {
    width: 100%;
    padding: 6px 8px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: #dbe4f0;
    font-family: inherit;
    font-size: calc(12px * var(--font-scale, 1));
    text-align: left;
    cursor: pointer;
  }

  .group-menu-item:hover:not(:disabled) {
    background: rgba(47, 129, 247, 0.18);
    color: #f8fafc;
  }

  .group-menu-item:disabled {
    color: #64748b;
    cursor: not-allowed;
  }
</style>
