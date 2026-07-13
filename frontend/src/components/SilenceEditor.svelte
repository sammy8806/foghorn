<script lang="ts">
  import { get } from 'svelte/store';
  import { createEventDispatcher } from 'svelte';
  import { GetUIConfig, CreateSilence, UpdateSilence, Unsilence } from '../../wailsjs/go/main/App';
  import type { Alert, Matcher, SilenceInfo } from '../stores/alerts';
  import { alerts, sourceCapabilities } from '../stores/alerts';
  import { filter } from '../stores/filter';
  import { queryToMatchers, type ParsedQuery, type DroppedTerm } from '../stores/query';
  import { matchesAllMatchers } from '../stores/matchers';
  import MatcherEditor from './MatcherEditor.svelte';

  export let alert: Alert | null = null;
  export let silence: SilenceInfo | null = null;
  export let mode: 'create' | 'edit' = 'create';
  export let open = false;
  export let query: ParsedQuery | null = null;
  export let seedMatchers: Matcher[] | null = null;
  export let preferredSource: string | null = null;

  const dispatch = createEventDispatcher<{ close: void; silenced: void }>();

  let editorMatchers: Matcher[] = [];
  let hiddenMatchers: Matcher[] = [];
  let expanded = false;
  let revealedAfterIndex: number | null = null;
  let revealedCount = 0;
  let alwaysVisible: string[] = ['alertname', 'cluster', 'severity', 'pod'];
  let collapseEnabled = true;
  let duration = '2h';
  let createdBy = '';
  let comment = '';
  let loading = false;
  let error = '';
  let confirmExpire = false;
  let initializedForOpen = false;
  let droppedTerms: DroppedTerm[] = [];
  let selectedSource = '';

  // Combined source of truth: hidden matchers are always part of the silence.
  $: allMatchers = [...editorMatchers, ...hiddenMatchers];

  // In alert-seeded / edit modes the source is fixed to the alert's source.
  // In query mode the user picks it.
  $: activeSource = alert ? alert.source : selectedSource;

  $: previewTotalOnSource = activeSource
    ? $alerts.filter((a) => a.source === activeSource).length
    : 0;
  $: previewMatchCount = activeSource
    ? $alerts.filter((a) => a.source === activeSource && matchesAllMatchers(a, allMatchers)).length
    : 0;
  $: previewValid = allMatchers.length > 0 && allMatchers.every((m) => m.name.trim() && m.value && regexValid(m));
  // Warn when the silence would catch nothing, or would catch every alert on the
  // source (likely too broad).
  $: previewWarn = previewValid && (previewMatchCount === 0 || (previewTotalOnSource > 0 && previewMatchCount === previewTotalOnSource));
  $: isAlertlessCreate = !!query || !!seedMatchers;
  $: isScratchCreate = (!!query && query.terms.length === 0) || (!!seedMatchers && seedMatchers.length === 0);
  $: hasQuerySeedMatchers = !!query && queryToMatchers(query).matchers.length > 0;

  // Candidate sources for the picker. Search-seeded silences stay scoped to
  // sources with matching alerts; scratch silences list all silence-capable
  // sources so the user can add matchers from an empty editor. Text-only
  // searches behave like scratch creates because text terms cannot be turned
  // into Alertmanager matchers.
  $: sourceCandidates = isAlertlessCreate
    ? (void $sourceCapabilities, sourcesForQuery($alerts, allMatchers, !hasQuerySeedMatchers && !seedMatchers?.length))
    : [];
  // Query-mode matchers can change after the source picker's initial default is
  // set (user edits matchers). If the current selection falls out of the
  // recomputed candidate list, re-default to the top candidate rather than
  // leaving a stale selection that no longer appears in the picker. Guarded to
  // non-empty candidates so a transient zero-match edit doesn't clobber the
  // pick to ''. Alert/edit modes never hit this since query is null there.
  $: if (isAlertlessCreate && !selectedSource && sourceCandidates.length) {
    selectedSource = preferredSource && sourceCandidates.some((c) => c.source === preferredSource)
      ? preferredSource
      : sourceCandidates[0].source;
  }
  $: if (isAlertlessCreate && selectedSource && sourceCandidates.length &&
         !sourceCandidates.some((c) => c.source === selectedSource)) {
    selectedSource = sourceCandidates[0].source;
  }
  // "Show N more" only when matchers are actually hidden (N > 0). "Hide matchers"
  // only while expanded and some visible matcher would collapse back out of the
  // whitelist. The two are mutually exclusive: expanding empties hiddenMatchers.
  $: canExpand = collapseEnabled && hiddenMatchers.length > 0;
  $: canCollapse =
    collapseEnabled &&
    expanded &&
    editorMatchers.some((m) => !alwaysVisible.includes(m.name));

  const basePresets = ['30m', '1h', '2h', '4h', '8h', '24h', '3d', '1w'];
  const extendPresets = ['+30m', '+1h', '+4h', '+1d'];

  $: canSubmit =
    !loading &&
    !!duration &&
    !!createdBy.trim() &&
    !!activeSource &&
    allMatchers.length > 0 &&
    allMatchers.every((m) => m.name.trim() && m.value && regexValid(m));

  function silenceableSources(): string[] {
    return Object.entries(get(sourceCapabilities))
      .filter(([, capabilities]) => capabilities.supportsSilence)
      .map(([source]) => source)
      .sort();
  }

  function sourcesWithMatches(all: Alert[], matchers: Matcher[]): { source: string; count: number }[] {
    const counts = new Map<string, number>();
    const allowed = new Set(silenceableSources());
    for (const a of all) {
      if (allowed.size > 0 && !allowed.has(a.source)) continue;
      if (matchers.length > 0 && !matchesAllMatchers(a, matchers)) continue;
      counts.set(a.source, (counts.get(a.source) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([source, count]) => ({ source, count }))
      .sort((x, y) => y.count - x.count);
  }

  function sourcesForQuery(all: Alert[], matchers: Matcher[], includeAllSilenceable: boolean): { source: string; count: number }[] {
    const withMatches = sourcesWithMatches(all, matchers);
    if (!includeAllSilenceable) return withMatches;

    const countsBySource = new Map(withMatches.map((c) => [c.source, c.count]));
    return silenceableSources()
      .map((source) => ({ source, count: countsBySource.get(source) ?? 0 }))
      .sort((a, b) => b.count - a.count || a.source.localeCompare(b.source));
  }

  function defaultSourceForQuery(seeded: Matcher[]): string {
    const f = get(filter);
    const candidates = sourcesForQuery(get(alerts), seeded, seeded.length === 0);
    if (f.source !== 'all' && candidates.some((c) => c.source === f.source)) {
      return f.source;
    }
    return candidates.length ? candidates[0].source : '';
  }

  function regexValid(m: Matcher): boolean {
    if (!m.isRegex) return true;
    try {
      new RegExp(m.value);
      return true;
    } catch {
      return false;
    }
  }

  async function loadDefaultCreatedBy(): Promise<string> {
    try {
      const uiConfig = await GetUIConfig();
      const uiConfigAny = uiConfig as any;
      const resolved =
        uiConfig.default_created_by ??
        uiConfigAny?.DefaultCreatedBy ??
        uiConfigAny?.defaultCreatedBy ??
        '';
      return (resolved || '').trim();
    } catch {
      return '';
    }
  }

  async function loadSilenceEditorConfig(): Promise<void> {
    try {
      const uiConfig = (await GetUIConfig()) as any;
      const se = uiConfig?.silence_editor ?? uiConfig?.SilenceEditor ?? {};
      const list = se?.always_visible_matchers ?? se?.AlwaysVisibleMatchers;
      if (Array.isArray(list)) alwaysVisible = list;
      const collapse = se?.collapse_matchers ?? se?.CollapseMatchers;
      if (typeof collapse === 'boolean') collapseEnabled = collapse;
    } catch {
      // Keep defaults (dev mode / no Wails).
    }
  }

  function splitMatchers(all: Matcher[]): { visible: Matcher[]; hidden: Matcher[] } {
    const visible: Matcher[] = [];
    const hidden: Matcher[] = [];
    for (const m of all) {
      if (alwaysVisible.includes(m.name)) visible.push(m);
      else hidden.push(m);
    }
    return { visible, hidden };
  }

  function applyCollapse(all: Matcher[]) {
    revealedAfterIndex = null;
    revealedCount = 0;
    if (!collapseEnabled) {
      editorMatchers = all;
      hiddenMatchers = [];
      expanded = true;
      return;
    }
    const { visible, hidden } = splitMatchers(all);
    editorMatchers = visible;
    hiddenMatchers = hidden;
    expanded = false;
  }

  function expandMatchers() {
    revealedAfterIndex = editorMatchers.length;
    revealedCount = hiddenMatchers.length;
    editorMatchers = [...editorMatchers, ...hiddenMatchers];
    hiddenMatchers = [];
    expanded = true;
  }

  function collapseMatchers() {
    const { visible, hidden } = splitMatchers(editorMatchers);
    editorMatchers = visible;
    hiddenMatchers = hidden;
    expanded = false;
    revealedAfterIndex = null;
    revealedCount = 0;
  }

  function replaceAllMatchers(e: CustomEvent<Matcher[]>) {
    applyCollapse(e.detail);
  }

  function matchersFromAlertLabels(a: Alert): Matcher[] {
    const entries = Object.entries(a.labels || {});
    return entries.map(([name, value]) => ({
      name,
      value,
      isRegex: false,
      isEqual: true,
    }));
  }

  function cloneMatchers(ms: Matcher[] | undefined): Matcher[] {
    return (ms || []).map((m) => ({ ...m }));
  }

  // DURATION_RE accepts 1w, 1d, 2h, 30m, 10s and any concatenation (e.g. "1w2d3h30m").
  // The regex requires at least one group present (enforced by post-match check).
  const DURATION_RE = /^\s*(?:(\d+)w)?\s*(?:(\d+)d)?\s*(?:(\d+)h)?\s*(?:(\d+)m)?\s*(?:(\d+)s)?\s*$/i;

  function parseDurationMs(s: string): number | null {
    const trimmed = (s || '').trim();
    if (!trimmed) return null;
    const match = trimmed.match(DURATION_RE);
    if (!match || match.slice(1).every((g) => !g)) return null;
    const [, w, d, h, m, sec] = match;
    const weeks = w ? parseInt(w, 10) : 0;
    const days = d ? parseInt(d, 10) : 0;
    const hours = h ? parseInt(h, 10) : 0;
    const mins = m ? parseInt(m, 10) : 0;
    const secs = sec ? parseInt(sec, 10) : 0;
    return ((((weeks * 7 + days) * 24 + hours) * 60 + mins) * 60 + secs) * 1000;
  }

  function roundDuration(ms: number): string {
    if (ms <= 0) return '0s';
    // Round to the nearest minute for a clean unit string.
    const totalMins = Math.max(1, Math.round(ms / 60000));
    const totalHours = Math.floor(totalMins / 60);
    const minutes = totalMins % 60;
    const totalDays = Math.floor(totalHours / 24);
    const hours = totalHours % 24;
    const weeks = Math.floor(totalDays / 7);
    const days = totalDays % 7;
    const parts: string[] = [];
    if (weeks) parts.push(`${weeks}w`);
    if (days) parts.push(`${days}d`);
    if (hours) parts.push(`${hours}h`);
    if (minutes) parts.push(`${minutes}m`);
    return parts.length ? parts.join('') : '1m';
  }

  function extendDuration(shortcut: string) {
    const raw = shortcut.startsWith('+') ? shortcut.slice(1) : shortcut;
    const shortcutMs = parseDurationMs(raw) || 0;
    const currentMs = parseDurationMs(duration) || 0;
    duration = roundDuration(currentMs + shortcutMs);
  }

  async function resetForOpen() {
    error = '';
    loading = false;
    confirmExpire = false;
    await loadSilenceEditorConfig();
    let all: Matcher[];
    if (mode === 'edit' && silence && alert) {
      all = cloneMatchers(silence.matchers);
      if (!all.length) {
        // Safety net: silence without matchers is unusual but don't nuke the editor.
        all = matchersFromAlertLabels(alert);
      }
      const endMs = new Date(silence.endsAt).getTime() - Date.now();
      duration = roundDuration(Math.max(0, endMs));
      if (duration === '0s') duration = '1m';
      comment = silence.comment || '';
      createdBy = (silence.createdBy || '').trim();
      droppedTerms = [];
    } else if (query) {
      const { matchers, dropped } = queryToMatchers(query);
      all = matchers.map((m) => ({ ...m }));
      droppedTerms = dropped;
      duration = '2h';
      comment = '';
      createdBy = '';
      selectedSource = preferredSource || defaultSourceForQuery(all);
      void loadDefaultCreatedBy().then((v) => {
        if (!createdBy) createdBy = v;
      });
    } else if (seedMatchers) {
      all = seedMatchers.map((m) => ({ ...m }));
      droppedTerms = [];
      duration = '2h';
      comment = '';
      createdBy = '';
      selectedSource = preferredSource || defaultSourceForQuery(all);
      void loadDefaultCreatedBy().then((v) => {
        if (!createdBy) createdBy = v;
      });
    } else {
      all = alert ? matchersFromAlertLabels(alert) : [];
      droppedTerms = [];
      duration = '2h';
      comment = '';
      createdBy = '';
      void loadDefaultCreatedBy().then((v) => {
        if (!createdBy) createdBy = v;
      });
    }
    applyCollapse(all);
  }

  $: if (open && !initializedForOpen) {
    initializedForOpen = true;
    void resetForOpen();
  }

  $: if (!open) {
    initializedForOpen = false;
  }

  function close() {
    dispatch('close');
  }

  function setDurationPreset(value: string) {
    duration = value;
  }

  async function submit() {
    if (!activeSource || !canSubmit) return;
    loading = true;
    error = '';
    try {
      if (mode === 'edit' && silence && alert) {
        await UpdateSilence(alert.source, silence.id, allMatchers, duration, createdBy, comment);
      } else {
        await CreateSilence(activeSource, allMatchers, duration, createdBy, comment);
      }
      dispatch('silenced');
      dispatch('close');
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
    }
  }

  async function doExpire() {
    if (!alert || !silence) return;
    loading = true;
    error = '';
    try {
      await Unsilence(alert.source, silence.id);
      dispatch('silenced');
      dispatch('close');
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
      confirmExpire = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }

  function formatRemaining(endsAt: string): string {
    const diffMs = new Date(endsAt).getTime() - Date.now();
    if (diffMs <= 0) return 'expired';
    const mins = Math.floor(diffMs / 60000);
    if (mins < 60) return `${mins}m`;
    const hours = Math.floor(mins / 60);
    return `${hours}h ${mins % 60}m`;
  }
</script>

{#if open && (alert || query || seedMatchers)}
  <div class="overlay" on:click={close} on:keydown={handleKeydown} role="presentation">
    <div
      class="dialog"
      on:click|stopPropagation
      on:keydown|stopPropagation
      role="dialog"
      aria-modal="true"
      aria-labelledby="silence-title"
    >
      <div class="dialog-header">
        <h3 id="silence-title">{mode === 'edit' ? 'Edit silence' : isScratchCreate ? 'New silence' : 'Silence alert'}</h3>
        <button class="btn-close" on:click={close} aria-label="Close">✕</button>
      </div>

      <div class="dialog-body">
        {#if alert || mode === 'edit'}
          <div class="context-strip">
            {#if mode === 'edit' && silence}
            <span class="ctx-item"><strong>id:</strong> {silence.id.slice(0, 10)}…</span>
            <span class="ctx-item"><strong>started:</strong> {new Date(silence.startsAt).toLocaleString()}</span>
            <span class="ctx-item"><strong>by:</strong> {silence.createdBy}</span>
            <span class="ctx-item"><strong>expires in:</strong> {formatRemaining(silence.endsAt)}</span>
            {:else if alert}
            <span class="alert-name">{alert.name}</span>
            <span class="alert-source">{alert.source}</span>
            {/if}
          </div>
        {/if}

        {#if isAlertlessCreate}
          <div class="field">
            <span class="field-label">Target source</span>
            <select class="input" bind:value={selectedSource}>
              {#each sourceCandidates as c}
                <option value={c.source}>{c.source} ({c.count})</option>
              {/each}
              {#if sourceCandidates.length === 0}
                <option value="" disabled>No silence-capable sources</option>
              {/if}
            </select>
          </div>
        {/if}

        <div class="field">
          <span class="field-label">Matchers ({allMatchers.length})</span>
          <MatcherEditor
            bind:matchers={editorMatchers}
            textMatchers={allMatchers}
            source={activeSource}
            revealedAfterIndex={revealedAfterIndex}
            revealedCount={revealedCount}
            on:replaceAll={replaceAllMatchers}
          >
            <svelte:fragment slot="actions">
              {#if canExpand}
                <button type="button" class="matcher-toggle" on:click={expandMatchers}>
                  ▸ Show {hiddenMatchers.length} more matcher{hiddenMatchers.length === 1 ? '' : 's'}
                </button>
              {:else if canCollapse}
                <button type="button" class="matcher-toggle" on:click={collapseMatchers}>
                  ▾ Hide matchers
                </button>
              {/if}
            </svelte:fragment>
          </MatcherEditor>
          {#if droppedTerms.length > 0}
            <p class="dropped-note">
              Not included in silence:
              {#each droppedTerms as d, i}
                <code>{d.label}</code><span class="dropped-reason"> ({d.reason})</span>{i < droppedTerms.length - 1 ? ', ' : ''}
              {/each}
            </p>
          {/if}
        </div>

        <div class="field">
          <span class="field-label">Ends in</span>
          <input
            class="input"
            type="text"
            bind:value={duration}
            placeholder="e.g. 2h, 1h30m, 45m"
          />
          <div class="presets">
            {#each basePresets as p}
              <button class="preset-btn" class:active={duration === p} on:click={() => setDurationPreset(p)}>{p}</button>
            {/each}
          </div>
          {#if mode === 'edit'}
            <div class="presets">
              {#each extendPresets as p}
                <button class="preset-btn" on:click={() => extendDuration(p)}>{p}</button>
              {/each}
            </div>
          {/if}
        </div>

        <label class="field">
          <span class="field-label">Comment</span>
          <textarea
            class="input textarea"
            bind:value={comment}
            placeholder="Reason for silencing…"
            rows="3"
          />
        </label>

        <label class="field">
          <span class="field-label">Created by</span>
          <input class="input" type="text" bind:value={createdBy} placeholder="Username" />
        </label>

        {#if error}
          <p class="error">{error}</p>
        {/if}
      </div>

      <div class="dialog-footer">
        <div class="footer-left">
          {#if previewValid && activeSource}
            <span class="match-preview" class:warn={previewWarn}>
              Matches {previewMatchCount} of {previewTotalOnSource} on {activeSource}
            </span>
          {/if}
          {#if mode === 'edit' && silence}
            {#if confirmExpire}
              <span class="expire-confirm-text">Expire now?</span>
              <button class="btn btn-expire" on:click={doExpire} disabled={loading}>
                {loading ? 'Expiring…' : 'Confirm'}
              </button>
              <button class="btn btn-cancel" on:click={() => (confirmExpire = false)} disabled={loading}>
                Cancel
              </button>
            {:else}
              <button class="btn btn-expire" on:click={() => (confirmExpire = true)} disabled={loading}>
                Expire now
              </button>
            {/if}
          {/if}
        </div>
        <div class="footer-right">
          <button class="btn btn-cancel" on:click={close} disabled={loading}>Cancel</button>
          <button class="btn btn-primary" on:click={submit} disabled={!canSubmit}>
            {loading ? (mode === 'edit' ? 'Saving…' : 'Silencing…') : mode === 'edit' ? 'Save changes' : 'Silence'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .dialog {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    width: 520px;
    max-width: 92vw;
    max-height: 92vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }
  .dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--color-border);
  }
  h3 {
    margin: 0;
    font-size: calc(15px * var(--font-scale, 1));
    font-weight: 600;
    color: var(--color-text);
  }
  .btn-close {
    background: none;
    border: none;
    color: var(--color-text-faint);
    cursor: pointer;
    font-size: calc(14px * var(--font-scale, 1));
    padding: 2px 6px;
  }
  .btn-close:hover { color: var(--color-text); }

  .matcher-toggle {
    background: none;
    border: none;
    color: var(--color-text-muted);
    font-size: calc(11px * var(--font-scale, 1));
    cursor: pointer;
    padding: 2px 0;
    white-space: nowrap;
  }
  .matcher-toggle:hover {
    color: var(--color-text);
  }

  .dialog-body {
    padding: 14px 18px;
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }

  .context-strip {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
    align-items: baseline;
    margin-bottom: 14px;
    padding: 6px 10px;
    background: var(--color-input-bg);
    border-radius: 4px;
    font-size: calc(11px * var(--font-scale, 1));
    color: var(--color-text-muted);
  }
  .ctx-item strong { color: var(--color-text); margin-right: 3px; font-weight: 600; }
  .alert-name { color: var(--color-text); font-weight: 600; font-size: calc(13px * var(--font-scale, 1)); }
  .alert-source { color: var(--color-text-faint); font-size: calc(11px * var(--font-scale, 1)); }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 12px;
    font-size: calc(12px * var(--font-scale, 1));
    color: var(--color-text-muted);
  }
  .field-label { font-weight: 500; color: var(--color-text-muted); }

  .input {
    background: var(--color-input-bg);
    border: 1px solid var(--color-control-border);
    border-radius: 4px;
    color: var(--color-text);
    font-size: calc(13px * var(--font-scale, 1));
    padding: 6px 10px;
    outline: none;
    width: 100%;
    box-sizing: border-box;
  }
  .input:focus { border-color: var(--color-focus); }
  .textarea { resize: vertical; font-family: inherit; }

  .presets {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .preset-btn {
    background: var(--color-input-bg);
    border: 1px solid var(--color-control-border);
    border-radius: 3px;
    color: var(--color-text-muted);
    cursor: pointer;
    font-size: calc(11px * var(--font-scale, 1));
    padding: 3px 8px;
  }
  .preset-btn:hover { border-color: var(--color-focus); color: var(--color-text); }
  .preset-btn.active { border-color: var(--color-focus); background: var(--color-interactive-bg); color: var(--color-interactive-strong); }

  .error {
    color: var(--color-danger);
    font-size: calc(12px * var(--font-scale, 1));
    margin: 8px 0 0;
  }

  .dropped-note {
    font-size: calc(11px * var(--font-scale, 1));
    color: var(--color-text-muted);
    margin: 6px 0 0;
  }
  .dropped-note code {
    background: var(--color-input-bg);
    border-radius: 3px;
    padding: 1px 4px;
    color: var(--color-text);
  }
  .dropped-reason { color: var(--color-text-faint); }

  .dialog-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 12px 18px;
    border-top: 1px solid var(--color-border);
  }
  .footer-left { display: flex; align-items: center; gap: 8px; }
  .footer-right { display: flex; align-items: center; gap: 8px; }
  .match-preview {
    font-size: calc(11px * var(--font-scale, 1));
    color: var(--color-text-muted);
  }
  .match-preview.warn { color: var(--color-warning); }
  .expire-confirm-text {
    font-size: calc(12px * var(--font-scale, 1));
    color: var(--color-danger);
  }

  .btn {
    border-radius: 4px;
    border: none;
    cursor: pointer;
    font-size: calc(13px * var(--font-scale, 1));
    font-weight: 500;
    padding: 7px 16px;
  }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .btn-cancel { background: var(--color-control-bg); color: var(--color-text); }
  .btn-cancel:hover:not(:disabled) { background: var(--color-hover); }

  .btn-primary { background: #3b82f6; color: #fff; }
  .btn-primary:hover:not(:disabled) { background: #2563eb; }

  .btn-expire { background: #7f1d1d; color: #fecaca; }
  .btn-expire:hover:not(:disabled) { background: #991b1b; color: #fff; }
</style>
