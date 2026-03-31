<script lang="ts">
  import type { Alert, DisplayConfig } from '../stores/alerts';
  import { acknowledgeAlert, acknowledgeResolvedAlert, alertMatchesBadgeRule, fieldNameFromRef, resolveAlertFieldDisplay, sourceCapabilities, verbose } from '../stores/alerts';
  import { TestNotificationForAlert } from '../../wailsjs/go/main/App';
  import { severityColor, formatDuration } from '../utils/severity';
  import SilenceDialog from './SilenceDialog.svelte';

  export let alert: Alert;
  export let config: DisplayConfig;
  export let isNew: boolean = false;
  export let isResolved: boolean = false;

  function labelName(spec: string): string {
    return fieldNameFromRef(spec);
  }

  $: visibleLabels = $verbose
    ? Object.keys(alert.labels || {})
    : (config.visible_labels || []).filter(spec => {
        const name = labelName(spec);
        return name !== 'alertname' && name !== 'severity';
      });
  $: visibleAnnotations = $verbose
    ? Object.keys(alert.annotations || {})
    : (config.visible_annotations || []);
  $: betterStackVisibleAnnotations = (() => {
    const names = [...visibleAnnotations];
    if (alert.sourceType === 'betterstack' && alert.annotations?.comments && !names.includes('comments')) {
      names.push('comments');
    }
    return names;
  })();
  $: matchedBadges = (config.badges || []).filter(rule => alertMatchesBadgeRule(alert, rule));

  // Auto-pick a subtitle from configured annotations, falling back to distinguishing labels
  const skipLabels = new Set(['alertname', 'severity', 'cluster', 'namespace', 'prometheus', 'prometheus_replica']);
  $: subtitle = (() => {
    const sources = config.subtitle_annotations || ['summary', 'description'];
    for (const spec of sources) {
      const display = resolveAlertFieldDisplay(alert, spec.startsWith('annotation:') ? spec : `annotation:${spec}`);
      if (display?.text) return display.text;
    }
    // Fall back to distinguishing labels
    const parts: string[] = [];
    for (const [k] of Object.entries(alert.labels || {})) {
      const display = resolveAlertFieldDisplay(alert, `label:${k}`);
      if (!skipLabels.has(k) && display?.text) parts.push(`${k}=${display.text}`);
    }
    return parts.join(', ');
  })();

  let expanded = false;
  let silenceOpen = false;
  let testingNotification = false;
  let testNotificationStatus = '';
  let acknowledgeTimer: ReturnType<typeof setTimeout> | null = null;

  $: alertKey = alert.source + ':' + alert.id;
  $: supportsSilence = !!$sourceCapabilities[alert.source]?.supportsSilence;
  $: primaryLinkLabel = alert.sourceType === 'betterstack' ? 'Open Incident' : 'Open Reference';

  function scheduleAcknowledge() {
    if ((!isNew && !isResolved) || acknowledgeTimer) return;
    acknowledgeTimer = setTimeout(() => {
      if (isResolved) {
        acknowledgeResolvedAlert(alertKey);
      } else {
        acknowledgeAlert(alertKey);
      }
      acknowledgeTimer = null;
    }, 600);
  }

  function cancelAcknowledge() {
    if (!acknowledgeTimer) return;
    clearTimeout(acknowledgeTimer);
    acknowledgeTimer = null;
  }

  async function handleTestNotification() {
    testingNotification = true;
    testNotificationStatus = '';
    try {
      await TestNotificationForAlert(alert.id, alert.source);
      testNotificationStatus = 'Notification sent';
    } catch (e) {
      testNotificationStatus = `Notification failed: ${String(e)}`;
    } finally {
      testingNotification = false;
    }
  }
</script>

<div
  class="alert-card"
  style:border-left-color={severityColor(alert.severity)}
  class:silenced={alert.silencedBy?.length > 0}
  class:alert-new={isNew}
  class:alert-resolved={isResolved}
  on:pointerenter={scheduleAcknowledge}
  on:pointerleave={cancelAcknowledge}
>
  <div class="alert-header" on:click={() => (expanded = !expanded)} role="button" tabindex="0" on:keydown={e => e.key === 'Enter' && (expanded = !expanded)}>
    <span class="severity-dot" style="background: {severityColor(alert.severity)}" />
    {#if isNew}
      <span class="badge badge-new" title="New alert. Hover for a moment to mark as seen.">NEW</span>
    {/if}
    {#if isResolved}
      <span class="badge badge-resolved" title="Resolved alert. Hover briefly to mark as seen, or wait for it to expire.">RESOLVED</span>
    {/if}
    <span class="alert-name">{alert.name}</span>
    {#if subtitle}
      <span class="alert-subtitle" title={subtitle}>{subtitle}</span>
    {/if}
    {#if alert.silencedBy?.length > 0}
      <span class="badge badge-silenced">silenced</span>
    {/if}
    {#if alert.inhibitedBy?.length > 0}
      <span class="badge badge-inhibited">inhibited</span>
    {/if}
    {#each matchedBadges as badgeRule}
      <span class="badge badge-custom" title={`${fieldNameFromRef(badgeRule.field)} matches ${badgeRule.equals.join(', ')}`}>{badgeRule.label}</span>
    {/each}
    <span class="alert-source">{alert.source}</span>
    <span class="alert-duration">{formatDuration(alert.startsAt)}</span>
    <span class="chevron" class:expanded>{expanded ? '▲' : '▼'}</span>
  </div>

  {#if expanded}
    <div class="alert-body">
      {#each betterStackVisibleAnnotations as key}
        {@const annotationName = fieldNameFromRef(key)}
        {@const annotationDisplay = resolveAlertFieldDisplay(alert, key.startsWith('annotation:') ? key : `annotation:${key}`)}
        {#if annotationDisplay?.text}
          <p class="annotation"><strong>{annotationName}:</strong>
            {#if annotationDisplay.text.match(/^https?:\/\//)}
              <a href={annotationDisplay.text} target="_blank" class="annotation-link">{annotationDisplay.text}</a>
            {:else}
              <span class:annotation-multiline={annotationName === 'comments'}>{annotationDisplay.text}</span>
            {/if}
          </p>
        {/if}
      {/each}

      <div class="label-chips">
        {#each visibleLabels as spec}
          {@const label = labelName(spec)}
          {@const labelDisplay = resolveAlertFieldDisplay(alert, spec.startsWith('label:') ? spec : `label:${spec}`)}
          {#if labelDisplay?.text}
            <span class="chip">
              {#if labelDisplay.mode === 'both' && labelDisplay.raw && labelDisplay.resolved && labelDisplay.raw !== labelDisplay.resolved}
                <span>{label}={labelDisplay.raw}</span>
                <span class="chip-resolved">({labelDisplay.resolved})</span>
              {:else}
                <span>{label}={labelDisplay.text}</span>
              {/if}
            </span>
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
          <a href={alert.generatorURL} target="_blank" rel="noreferrer" class="generator-link">{primaryLinkLabel}</a>
        {/if}
        {#if $verbose}
          <button
            class="btn-silence"
            on:click|stopPropagation={handleTestNotification}
            disabled={testingNotification}
            title="Send a notification preview using this alert"
          >
            {testingNotification ? 'Sending…' : 'Test notification'}
          </button>
          {#if testNotificationStatus}
            <span class="action-status">{testNotificationStatus}</span>
          {/if}
        {/if}
        {#if supportsSilence && !alert.silencedBy?.length && !isResolved}
          <button class="btn-silence" on:click|stopPropagation={() => (silenceOpen = true)}>Silence…</button>
        {/if}
      </div>
    </div>
  {/if}
</div>

<SilenceDialog {alert} open={silenceOpen} on:close={() => (silenceOpen = false)} />

<style>
  .alert-card {
    position: relative;
    border-left: 3px solid #6b7280;
    background: var(--card-bg, #1e293b);
    border-radius: 3px;
    margin-bottom: 2px;
    overflow: hidden;
    transition: border-color 0.15s, box-shadow 0.15s, transform 0.15s;
  }
  .silenced { opacity: 0.6; }
  .alert-new {
    border-left-width: 8px;
    box-shadow: inset 0 0 0 1px rgba(250, 204, 21, 0.35), 0 0 0 1px rgba(250, 204, 21, 0.2);
    background:
      linear-gradient(90deg, rgba(250, 204, 21, 0.18), rgba(250, 204, 21, 0.05) 28%, rgba(15, 23, 42, 0) 60%),
      var(--card-bg, #1e293b);
    animation: alert-new-pulse 1.2s ease-in-out 3 forwards;
  }
  .alert-new:hover {
    transform: translateX(1px);
    box-shadow: inset 0 0 0 1px rgba(250, 204, 21, 0.55), 0 0 0 1px rgba(250, 204, 21, 0.4), 0 0 18px rgba(250, 204, 21, 0.18);
  }
  .alert-resolved {
    border-left-width: 8px;
    box-shadow: inset 0 0 0 1px rgba(34, 197, 94, 0.28), 0 0 0 1px rgba(34, 197, 94, 0.16);
    background:
      linear-gradient(90deg, rgba(34, 197, 94, 0.14), rgba(34, 197, 94, 0.05) 26%, rgba(15, 23, 42, 0) 58%),
      var(--card-bg, #1e293b);
    animation: alert-resolved-hover 4.2s ease-in-out infinite;
  }
  .alert-resolved:hover {
    transform: translateX(1px);
    box-shadow: inset 0 0 0 1px rgba(34, 197, 94, 0.42), 0 0 0 1px rgba(34, 197, 94, 0.26), 0 0 14px rgba(34, 197, 94, 0.12);
  }
  @keyframes alert-new-pulse {
    0%, 100% { box-shadow: inset 0 0 0 1px rgba(250, 204, 21, 0.35), 0 0 0 1px rgba(250, 204, 21, 0.2); }
    50% { box-shadow: inset 0 0 0 1px rgba(250, 204, 21, 0.65), 0 0 0 1px rgba(250, 204, 21, 0.45), 0 0 20px rgba(250, 204, 21, 0.2); }
  }
  @keyframes alert-resolved-hover {
    0%, 100% {
      box-shadow: inset 0 0 0 1px rgba(34, 197, 94, 0.24), 0 0 0 1px rgba(34, 197, 94, 0.14), 0 0 8px rgba(34, 197, 94, 0.06);
      background:
        linear-gradient(90deg, rgba(34, 197, 94, 0.12), rgba(34, 197, 94, 0.04) 26%, rgba(15, 23, 42, 0) 58%),
        var(--card-bg, #1e293b);
    }
    50% {
      box-shadow: inset 0 0 0 1px rgba(34, 197, 94, 0.36), 0 0 0 1px rgba(34, 197, 94, 0.22), 0 0 14px rgba(34, 197, 94, 0.1);
      background:
        linear-gradient(90deg, rgba(34, 197, 94, 0.18), rgba(34, 197, 94, 0.06) 28%, rgba(15, 23, 42, 0) 60%),
        var(--card-bg, #1e293b);
    }
  }

  .alert-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    cursor: pointer;
    user-select: none;
    min-height: 0;
  }
  .alert-header:hover { background: rgba(255,255,255,0.05); }

  .severity-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .alert-name {
    font-weight: 600;
    font-size: 12px;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .alert-subtitle {
    font-size: 11px;
    color: #64748b;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
    min-width: 0;
  }
  .alert-subtitle::before {
    content: '— ';
    color: #475569;
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
  .badge-new {
    background: #facc15;
    color: #1f2937;
    letter-spacing: 0.08em;
    box-shadow: 0 0 10px rgba(250, 204, 21, 0.28);
  }
  .badge-resolved {
    background: #22c55e;
    color: #052e16;
    letter-spacing: 0.08em;
    box-shadow: 0 0 10px rgba(34, 197, 94, 0.24);
  }
  .badge-silenced { background: #334155; color: #94a3b8; }
  .badge-inhibited { background: #292524; color: #a8a29e; }
  .badge-custom {
    background: #0f766e;
    color: #ccfbf1;
    border: 1px solid rgba(94, 234, 212, 0.25);
  }

  .chevron { font-size: 10px; color: #64748b; }

  .alert-body {
    padding: 6px 10px 8px 22px;
    border-top: 1px solid rgba(255,255,255,0.05);
  }

  .annotation {
    font-size: 11px;
    color: #cbd5e1;
    margin: 2px 0;
  }
  .annotation-multiline {
    white-space: pre-wrap;
  }

  .label-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
    margin-top: 6px;
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

  .chip-resolved {
    color: #64748b;
    margin-left: 0.35rem;
  }

  .metadata {
    display: flex;
    flex-wrap: wrap;
    gap: 3px 10px;
    margin-top: 6px;
    padding: 4px 6px;
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
    gap: 10px;
    margin-top: 6px;
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
  .btn-silence:disabled {
    color: #64748b;
    cursor: default;
  }
  .btn-silence:hover { border-color: #f59e0b; color: #f59e0b; }

  .action-status {
    font-size: 11px;
    color: #94a3b8;
    white-space: nowrap;
  }
</style>
