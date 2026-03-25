<script lang="ts">
  import type { Alert, DisplayConfig } from '../stores/alerts';
  import { verbose } from '../stores/alerts';
  import { severityClass, severityColor, formatDuration } from '../utils/severity';
  import SilenceDialog from './SilenceDialog.svelte';

  export let alert: Alert;
  export let config: DisplayConfig;

  $: visibleLabels = $verbose
    ? Object.keys(alert.labels || {})
    : (config.visible_labels || []).filter(l => l !== 'alertname' && l !== 'severity');
  $: visibleAnnotations = $verbose
    ? Object.keys(alert.annotations || {})
    : (config.visible_annotations || []);

  let expanded = false;
  let silenceOpen = false;
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
          <p class="annotation"><strong>{key}:</strong>
            {#if alert.annotations[key].match(/^https?:\/\//)}
              <a href={alert.annotations[key]} target="_blank" class="annotation-link">{alert.annotations[key]}</a>
            {:else}
              {alert.annotations[key]}
            {/if}
          </p>
        {/if}
      {/each}

      <div class="label-chips">
        {#each visibleLabels as label}
          {#if alert.labels?.[label]}
            <span class="chip">{label}={alert.labels[label]}</span>
          {/if}
        {/each}
      </div>

      {#if $verbose}
        <div class="metadata">
          <span class="meta-item"><strong>id:</strong> {alert.id}</span>
          <span class="meta-item"><strong>source:</strong> {alert.source}</span>
          <span class="meta-item"><strong>sourceType:</strong> {alert.sourceType}</span>
          <span class="meta-item"><strong>state:</strong> {alert.state}</span>
          <span class="meta-item"><strong>startsAt:</strong> {alert.startsAt}</span>
          <span class="meta-item"><strong>updatedAt:</strong> {alert.updatedAt}</span>
          {#if alert.silencedBy?.length > 0}
            <span class="meta-item"><strong>silencedBy:</strong> {alert.silencedBy.join(', ')}</span>
          {/if}
          {#if alert.inhibitedBy?.length > 0}
            <span class="meta-item"><strong>inhibitedBy:</strong> {alert.inhibitedBy.join(', ')}</span>
          {/if}
          {#if alert.receivers?.length > 0}
            <span class="meta-item"><strong>receivers:</strong> {alert.receivers.join(', ')}</span>
          {/if}
        </div>
      {/if}

      <div class="alert-actions">
        {#if alert.generatorURL}
          <a href={alert.generatorURL} target="_blank" class="generator-link">Open in Prometheus</a>
        {/if}
        {#if !alert.silencedBy?.length}
          <button class="btn-silence" on:click|stopPropagation={() => (silenceOpen = true)}>Silence…</button>
        {/if}
      </div>
    </div>
  {/if}
</div>

<SilenceDialog {alert} open={silenceOpen} on:close={() => (silenceOpen = false)} />

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

  .metadata {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 12px;
    margin-top: 8px;
    padding: 6px 8px;
    background: rgba(0,0,0,0.2);
    border-radius: 3px;
    font-size: 11px;
    font-family: monospace;
    color: #64748b;
  }
  .meta-item strong { color: #94a3b8; }

  .alert-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 8px;
  }

  .annotation-link, .generator-link {
    font-size: 11px;
    color: #60a5fa;
    text-decoration: none;
  }
  .annotation-link:hover, .generator-link:hover { text-decoration: underline; }

  .btn-silence {
    background: none;
    border: 1px solid #334155;
    border-radius: 3px;
    color: #94a3b8;
    cursor: pointer;
    font-size: 11px;
    padding: 2px 8px;
  }
  .btn-silence:hover { border-color: #f59e0b; color: #f59e0b; }
</style>
