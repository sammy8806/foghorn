<script lang="ts">
  import type { Alert, DisplayConfig, SilenceInfo, VisibleEntry } from '../stores/alerts';
  import { acknowledgeAlert, acknowledgeResolvedAlert, alertMatchesBadgeRule, fieldNameFromRef, refreshAlerts, resolveAlertFieldDisplay, sourceCapabilities, verbose } from '../stores/alerts';
  import { TestNotificationForAlert, Unsilence } from '../../wailsjs/go/main/App';
  import { severityColor, formatDuration } from '../utils/severity';
  import { openSilenceCreate, openSilenceEdit } from '../stores/silenceEditor';

  export let alert: Alert;
  export let config: DisplayConfig;
  export let isNew: boolean = false;
  export let isResolved: boolean = false;

  function labelName(spec: string): string {
    return fieldNameFromRef(spec);
  }

  // hasRefPrefix returns true when a config-supplied ref already carries an
  // explicit kind prefix (field:/label:/annotation:). Used so visible_labels
  // and visible_annotations can hold cross-kind refs like `field:hiddenBy`
  // without being silently coerced into the loop's default kind.
  function hasRefPrefix(spec: string): boolean {
    return spec.startsWith('field:') || spec.startsWith('label:') || spec.startsWith('annotation:');
  }

  function entryLabel(entry: VisibleEntry, fallback: string): string {
    return entry.label && entry.label.length > 0 ? entry.label : fallback;
  }

  function entryClasses(base: string, entry: VisibleEntry): string {
    const styles = entry.style || [];
    return [base, ...styles.map(s => `${base}--${s}`)].join(' ');
  }

  // verboseEntries powers "Show all": it reveals every field without discarding
  // the configured presentation. The configured entries are kept verbatim (in
  // their backend-sorted order, with their label/style/order overrides and any
  // synthetic refs like field:hiddenBy intact), and the alert's remaining keys
  // are appended as bare entries. Verbose augments the config, it doesn't
  // replace it.
  function verboseEntries(configured: VisibleEntry[], alertKeys: string[]): VisibleEntry[] {
    const covered = new Set(configured.map(e => labelName(e.source)));
    const extras = alertKeys
      .filter(k => !covered.has(k))
      .map(k => ({ source: k, order: 0 }));
    return [...configured, ...extras];
  }

  $: visibleLabels = $verbose
    ? verboseEntries(config.visible_labels || [], Object.keys(alert.labels || {}))
    : (config.visible_labels || []).filter(entry => {
        const name = labelName(entry.source);
        return name !== 'alertname' && name !== 'severity';
      });
  $: visibleAnnotations = $verbose
    ? verboseEntries(config.visible_annotations || [], Object.keys(alert.annotations || {}))
    : (config.visible_annotations || []);
  $: betterStackVisibleAnnotations = (() => {
    const entries = [...visibleAnnotations];
    const hasComments = entries.some(e => labelName(e.source) === 'comments');
    if (alert.sourceType === 'betterstack' && alert.annotations?.comments && !hasComments) {
      entries.push({ source: 'comments', order: 0 });
    }
    return entries;
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

  function formatTimeRemaining(endsAt: string): string {
    const end = new Date(endsAt);
    const now = new Date();
    const diffMs = end.getTime() - now.getTime();
    if (diffMs <= 0) return 'expired';
    const mins = Math.floor(diffMs / 60000);
    if (mins < 60) return `${mins}m`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ${mins % 60}m`;
    const days = Math.floor(hours / 24);
    return `${days}d ${hours % 24}h`;
  }

  function silenceTooltip(silences: SilenceInfo[]): string {
    return silences.map(s => {
      const expires = formatTimeRemaining(s.endsAt);
      const line = `Silenced by ${s.createdBy}`;
      return s.comment ? `${line}: ${s.comment} (expires in ${expires})` : `${line} (expires in ${expires})`;
    }).join('\n');
  }

  $: silenceBadgeTitle = (alert.silences?.length)
    ? silenceTooltip(alert.silences)
    : alert.silencedBy?.length > 0
      ? `Silence IDs: ${alert.silencedBy.join(', ')}`
      : '';

  let expanded = false;
  let expireConfirmId: string | null = null;
  let expireError: Record<string, string> = {};
  let expiring: Record<string, boolean> = {};
  let testingNotification = false;
  let testNotificationStatus = '';
  let acknowledgeTimer: ReturnType<typeof setTimeout> | null = null;

  type CommentSegment = {
    kind: 'text' | 'mention';
    text: string;
    email?: string;
  };

  type ParsedComment = {
    author: string;
    email: string;
    time: string;
    body: string;
  };

  const commentHeaderPattern = /^(.+?)(?:\s+<([^>]+)>)?\s*-\s*(\d{4}-\d{2}-\d{2}T[\d:.]+Z?)$/;

  function parseCommentBlocks(text: string): ParsedComment[] {
    const comments: ParsedComment[] = [];
    let current: ParsedComment | null = null;

    for (const line of text.split('\n')) {
      const header = line.match(commentHeaderPattern);
      if (header) {
        if (current) comments.push(current);
        current = { author: header[1], email: header[2] || '', time: header[3], body: '' };
        continue;
      }

      if (current) {
        current.body = current.body ? `${current.body}\n${line}` : line;
        continue;
      }

      if (line.trim() !== '') {
        comments.push({ author: '', email: '', time: '', body: line });
      }
    }

    if (current) comments.push(current);
    return comments.length ? comments : [{ author: '', email: '', time: '', body: text }];
  }

  function commentBodySegments(text: string): CommentSegment[] {
    const segments: CommentSegment[] = [];
    const mentionPattern = /\[(@[^\]]+)\]\(mailto:([^)]+)\)/g;
    let cursor = 0;
    let match: RegExpExecArray | null;

    while ((match = mentionPattern.exec(text)) !== null) {
      if (match.index > cursor) {
        segments.push({ kind: 'text', text: text.slice(cursor, match.index) });
      }
      segments.push({ kind: 'mention', text: match[1], email: match[2] });
      cursor = match.index + match[0].length;
    }

    if (cursor < text.length) {
      segments.push({ kind: 'text', text: text.slice(cursor) });
    }
    return segments.length ? segments : [{ kind: 'text', text }];
  }

  function askExpire(s: SilenceInfo) {
    expireConfirmId = s.id;
    expireError = { ...expireError, [s.id]: '' };
  }

  function cancelExpire() {
    expireConfirmId = null;
  }

  async function confirmExpireSilence(s: SilenceInfo) {
    expiring = { ...expiring, [s.id]: true };
    expireError = { ...expireError, [s.id]: '' };
    try {
      await Unsilence(alert.source, s.id);
      expireConfirmId = null;
      void refreshAlerts();
    } catch (e) {
      expireError = { ...expireError, [s.id]: String(e) };
    } finally {
      expiring = { ...expiring, [s.id]: false };
    }
  }

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
      <span class="badge badge-silenced" title={silenceBadgeTitle}>silenced</span>
    {/if}
    {#if alert.inhibitedBy?.length > 0}
      <span class="badge badge-inhibited">inhibited</span>
    {/if}
    {#if alert.hiddenBy?.length}
      <span class="badge badge-hidden" title={`Hidden by rule(s): ${alert.hiddenBy.join(', ')}`}>hidden</span>
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
      {#each betterStackVisibleAnnotations as entry}
        {@const ref = hasRefPrefix(entry.source) ? entry.source : `annotation:${entry.source}`}
        {@const annotationName = fieldNameFromRef(ref)}
        {@const annotationDisplay = resolveAlertFieldDisplay(alert, ref)}
        {@const displayLabel = entryLabel(entry, annotationName)}
        {@const annotationClass = entryClasses('annotation', entry)}
        {#if annotationDisplay?.text}
          {#if annotationName === 'comments'}
            <div class="comments-section">
              <strong class="comments-label">{displayLabel}:</strong>
              {#each parseCommentBlocks(annotationDisplay.text) as comment}
                <div class="comment-card">
                  {#if comment.author}
                    <div class="comment-header">
                      <span class="comment-author" title={comment.email}>{comment.author}</span>
                      <span class="comment-time">{new Date(comment.time).toLocaleString()}</span>
                    </div>
                  {/if}
                  <div class="comment-body">
                    {#each commentBodySegments(comment.body.trim()) as segment}
                      {#if segment.kind === 'mention'}
                        <span class="comment-mention" title={segment.email}>{segment.text}</span>
                      {:else}
                        <span>{segment.text}</span>
                      {/if}
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          {:else}
            <p class={annotationClass}><strong>{displayLabel}:</strong>
              {#if annotationDisplay.text.match(/^https?:\/\//)}
                <a href={annotationDisplay.text} target="_blank" class="annotation-link">{annotationDisplay.text}</a>
              {:else}
                <span>{annotationDisplay.text}</span>
              {/if}
            </p>
          {/if}
        {/if}
      {/each}

      {#if alert.silences?.length}
        <div class="silence-details">
          {#each alert.silences as s}
            <div class="silence-card">
              <div class="silence-header">
                <span class="silence-author">{s.createdBy}</span>
                <span class="silence-expiry">expires in {formatTimeRemaining(s.endsAt)}</span>
                <div class="silence-actions">
                  {#if expireConfirmId === s.id}
                    <span class="expire-confirm-label">Expire?</span>
                    <button class="btn-link-expire" on:click|stopPropagation={() => confirmExpireSilence(s)} disabled={!!expiring[s.id]}>
                      {expiring[s.id] ? 'Expiring…' : 'Expire'}
                    </button>
                    <button class="btn-link-edit" on:click|stopPropagation={cancelExpire} disabled={!!expiring[s.id]}>Cancel</button>
                  {:else}
                    <button class="btn-link-edit" on:click|stopPropagation={() => openSilenceEdit(alert, s)}>Edit</button>
                    <button class="btn-link-expire" on:click|stopPropagation={() => askExpire(s)}>Expire now</button>
                  {/if}
                </div>
              </div>
              {#if s.comment}
                <div class="silence-comment">{s.comment}</div>
              {/if}
              {#if expireError[s.id]}
                <div class="silence-error">{expireError[s.id]}</div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}

      <div class="label-chips">
        {#each visibleLabels as entry}
          {@const ref = hasRefPrefix(entry.source) ? entry.source : `label:${entry.source}`}
          {@const label = entryLabel(entry, labelName(entry.source))}
          {@const labelDisplay = resolveAlertFieldDisplay(alert, ref)}
          {@const chipClass = entryClasses('chip', entry)}
          {#if labelDisplay?.text}
            <span class={chipClass}>
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
          {#if alert.hiddenBy?.length}
            <span class="meta-item"><strong>hiddenBy:</strong> {alert.hiddenBy.join(', ')}</span>
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
          <button class="btn-silence" on:click|stopPropagation={() => openSilenceCreate(alert)}>Silence…</button>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .alert-card {
    position: relative;
    border-left: 3px solid #6b7280;
    background: var(--card-bg, var(--color-surface));
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
    font-size: calc(12px * var(--font-scale, 1));
    white-space: nowrap;
    flex-shrink: 0;
  }

  .alert-subtitle {
    font-size: calc(11px * var(--font-scale, 1));
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
    font-size: calc(11px * var(--font-scale, 1));
    color: #94a3b8;
    white-space: nowrap;
  }

  .alert-duration {
    font-size: calc(11px * var(--font-scale, 1));
    color: #64748b;
    white-space: nowrap;
  }

  .badge {
    font-size: calc(10px * var(--font-scale, 1));
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
  .badge-hidden { background: #3f3f46; color: #d4d4d8; }
  .badge-custom {
    background: #0f766e;
    color: #ccfbf1;
    border: 1px solid rgba(94, 234, 212, 0.25);
  }

  .chevron { font-size: calc(10px * var(--font-scale, 1)); color: #64748b; }

  .alert-body {
    padding: 6px 10px 8px 22px;
    border-top: 1px solid rgba(255,255,255,0.05);
  }

  .annotation {
    font-size: calc(11px * var(--font-scale, 1));
    color: #cbd5e1;
    margin: 2px 0;
  }
  .annotation--muted { color: #64748b; font-size: calc(10px * var(--font-scale, 1)); }
  .annotation--pull {
    font-size: calc(12px * var(--font-scale, 1));
    padding: 5px 8px;
    border-left: 3px solid #334155;
    background: rgba(0, 0, 0, 0.25);
    border-radius: 3px;
    margin: 4px 0;
  }
  .annotation--danger { border-left-color: #ef4444; color: #ef4444; }
  .annotation--warning { border-left-color: #f59e0b; color: #f59e0b; }
  .annotation--info { border-left-color: #3b82f6; color: #3b82f6; }

  .chip--muted { opacity: 0.65; }
  .chip--danger { background: rgba(239, 68, 68, 0.15); color: #ef4444; }
  .chip--warning { background: rgba(245, 158, 11, 0.15); color: #f59e0b; }
  .chip--info { background: rgba(59, 130, 246, 0.15); color: #3b82f6; }
  /* .chip--pull is intentionally not styled — pull is an annotation-only treatment. */
  .comments-section {
    margin: 4px 0;
  }
  .comments-label {
    font-size: calc(11px * var(--font-scale, 1));
    color: #cbd5e1;
    display: block;
    margin-bottom: 4px;
  }
  .comment-card {
    background: rgba(0, 0, 0, 0.25);
    border-left: 2px solid #334155;
    border-radius: 2px;
    padding: 5px 8px;
    margin-bottom: 4px;
    font-size: calc(11px * var(--font-scale, 1));
  }
  .comment-card:last-child {
    margin-bottom: 0;
  }
  .comment-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 3px;
    gap: 8px;
  }
  .comment-author {
    color: #60a5fa;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .comment-time {
    color: #64748b;
    font-size: calc(10px * var(--font-scale, 1));
    white-space: nowrap;
    flex-shrink: 0;
  }
  .comment-body {
    color: #cbd5e1;
    white-space: pre-wrap;
    line-height: 1.4;
  }
  .comment-mention {
    display: inline;
    color: #bfdbfe;
    font-weight: 700;
  }

  .silence-details {
    margin: 4px 0;
  }
  .silence-card {
    background: rgba(245, 158, 11, 0.08);
    border-left: 2px solid #f59e0b;
    border-radius: 2px;
    padding: 5px 8px;
    margin-bottom: 4px;
    font-size: calc(11px * var(--font-scale, 1));
  }
  .silence-card:last-child {
    margin-bottom: 0;
  }
  .silence-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
  }
  .silence-author {
    color: #f59e0b;
    font-weight: 600;
  }
  .silence-expiry {
    color: #64748b;
    font-size: calc(10px * var(--font-scale, 1));
    white-space: nowrap;
    flex-shrink: 0;
  }
  .silence-comment {
    color: #cbd5e1;
    margin-top: 3px;
    white-space: pre-wrap;
    line-height: 1.4;
  }

  .silence-actions {
    display: flex;
    gap: 6px;
    align-items: center;
    margin-left: auto;
  }
  .btn-link-edit,
  .btn-link-expire {
    background: none;
    border: none;
    cursor: pointer;
    font-size: calc(11px * var(--font-scale, 1));
    padding: 1px 4px;
    border-radius: 3px;
  }
  .btn-link-edit { color: #60a5fa; }
  .btn-link-edit:hover:not(:disabled) { background: rgba(96, 165, 250, 0.15); }
  .btn-link-expire { color: #f87171; }
  .btn-link-expire:hover:not(:disabled) { background: rgba(248, 113, 113, 0.15); }
  .btn-link-edit:disabled, .btn-link-expire:disabled { opacity: 0.5; cursor: default; }
  .expire-confirm-label {
    color: #f87171;
    font-size: calc(10px * var(--font-scale, 1));
    margin-right: 2px;
  }
  .silence-error {
    color: #f87171;
    font-size: calc(10px * var(--font-scale, 1));
    margin-top: 2px;
  }

  .label-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
    margin-top: 6px;
  }

  .chip {
    font-size: calc(11px * var(--font-scale, 1));
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
    font-size: calc(11px * var(--font-scale, 1));
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
    font-size: calc(11px * var(--font-scale, 1));
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
    font-size: calc(11px * var(--font-scale, 1));
    padding: 2px 8px;
  }
  .btn-silence:disabled {
    color: #64748b;
    cursor: default;
  }
  .btn-silence:hover { border-color: #f59e0b; color: #f59e0b; }

  .action-status {
    font-size: calc(11px * var(--font-scale, 1));
    color: #94a3b8;
    white-space: nowrap;
  }
</style>
