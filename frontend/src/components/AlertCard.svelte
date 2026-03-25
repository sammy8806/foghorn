<script lang="ts">
  import type { Alert, DisplayConfig } from '../stores/alerts';
  import { severityClass, severityColor, formatDuration } from '../utils/severity';

  export let alert: Alert;
  export let config: DisplayConfig;

  $: visibleLabels = (config.visible_labels || []).filter(l => l !== 'alertname' && l !== 'severity');
  $: visibleAnnotations = config.visible_annotations || [];

  let expanded = false;
</script>

<div class="alert-card {severityClass(alert.severity)}" class:silenced={alert.silencedBy?.length > 0}>
  <div class="alert-header" on:click={() => (expanded = !expanded)} role="button" tabindex="0" on:keydown={e => e.key === 'Enter' && (expanded = !expanded)}>
    <span class="severity-dot" style="background: {severityColor(alert.severity)}" />
    <span class="alert-name">{alert.name}</span>
    <span class="alert-source">{alert.source}</span>
    <span class="alert-duration">{formatDuration(alert.startsAt)}</span>
    {#if alert.silencedBy?.length > 0}
      <span class="badge badge-silenced">silenced</span>
    {/if}
    {#if alert.inhibitedBy?.length > 0}
      <span class="badge badge-inhibited">inhibited</span>
    {/if}
    <span class="chevron" class:expanded>{expanded ? '▲' : '▼'}</span>
  </div>

  {#if expanded}
    <div class="alert-body">
      {#each visibleAnnotations as key}
        {#if alert.annotations?.[key]}
          <p class="annotation"><strong>{key}:</strong> {alert.annotations[key]}</p>
        {/if}
      {/each}

      <div class="label-chips">
        {#each visibleLabels as label}
          {#if alert.labels?.[label]}
            <span class="chip">{label}={alert.labels[label]}</span>
          {/if}
        {/each}
      </div>

      {#if alert.generatorURL}
        <a href={alert.generatorURL} target="_blank" class="generator-link">Open in Prometheus</a>
      {/if}
    </div>
  {/if}
</div>

<style>
  .alert-card {
    border-left: 3px solid #6b7280;
    background: var(--card-bg, #1e293b);
    border-radius: 4px;
    margin-bottom: 4px;
    overflow: hidden;
    transition: border-color 0.15s;
  }
  .severity-critical { border-left-color: #ef4444; }
  .severity-warning { border-left-color: #f59e0b; }
  .severity-info { border-left-color: #3b82f6; }
  .silenced { opacity: 0.6; }

  .alert-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    cursor: pointer;
    user-select: none;
  }
  .alert-header:hover { background: rgba(255,255,255,0.05); }

  .severity-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .alert-name {
    font-weight: 600;
    font-size: 13px;
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .alert-source {
    font-size: 11px;
    color: #94a3b8;
    white-space: nowrap;
  }

  .alert-duration {
    font-size: 11px;
    color: #64748b;
    white-space: nowrap;
  }

  .badge {
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 10px;
    font-weight: 600;
    text-transform: uppercase;
  }
  .badge-silenced { background: #334155; color: #94a3b8; }
  .badge-inhibited { background: #292524; color: #a8a29e; }

  .chevron { font-size: 10px; color: #64748b; }

  .alert-body {
    padding: 8px 12px 12px 28px;
    border-top: 1px solid rgba(255,255,255,0.05);
  }

  .annotation {
    font-size: 12px;
    color: #cbd5e1;
    margin: 4px 0;
  }

  .label-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-top: 8px;
  }

  .chip {
    font-size: 11px;
    background: #0f172a;
    border: 1px solid #1e293b;
    padding: 2px 6px;
    border-radius: 3px;
    color: #94a3b8;
    font-family: monospace;
  }

  .generator-link {
    display: inline-block;
    margin-top: 8px;
    font-size: 11px;
    color: #60a5fa;
    text-decoration: none;
  }
  .generator-link:hover { text-decoration: underline; }
</style>
