<script lang="ts">
  import { get } from 'svelte/store';
  import { afterUpdate, createEventDispatcher } from 'svelte';
  import { GetUIConfig, CreateSilence, UpdateSilence, Unsilence } from '../../wailsjs/go/main/App';
  import type { Alert, Matcher, SilenceInfo } from '../stores/alerts';
  import { alerts, sourceCapabilities } from '../stores/alerts';
  import { filter } from '../stores/filter';
  import { queryToMatchers, type ParsedQuery, type DroppedTerm } from '../stores/query';
  import { matchesAllMatchers } from '../stores/matchers';
  import MatcherEditor from './MatcherEditor.svelte';
  import SelectMenu from './SelectMenu.svelte';
  import { cubicBezier } from '../utils/easing';

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

  // Present and dismiss both run as Svelte transitions rather than CSS
  // animations: a keyframe animation can't play on a node Svelte is removing,
  // which is why closing used to just vanish. easePanel is the system's own
  // panel curve — fast out of the gate, long settle, no overshoot; dismissal
  // uses its mirror and a shorter duration, so the dialog settles into place on
  // the way in and accelerates away on the way out.
  const easePanel = cubicBezier(0.32, 0.72, 0, 1);
  const easeDismiss = cubicBezier(0.4, 0, 1, 1);
  const PRESENT_MS = 280;
  const DISMISS_MS = 160;

  function reducedMotion(): boolean {
    return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
  }

  function panel(_node: Element, { duration, easing }: { duration: number; easing: (t: number) => number }) {
    return {
      duration: reducedMotion() ? 0 : duration,
      easing,
      css: (t: number, u: number) => `opacity: ${t}; transform: translateY(${u * 8}px) scale(${1 - u * 0.03});`,
    };
  }

  function scrim(_node: Element, { duration }: { duration: number }) {
    return {
      duration: reducedMotion() ? 0 : duration,
      // pointer-events is inherited, so this makes the whole dialog inert while
      // it moves — no clicking a target that is still sliding into place, and no
      // clicking through one that is on its way out.
      css: (t: number) => `opacity: ${t}; pointer-events: none;`,
    };
  }

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

  // Header/footer dividers only exist to say "there is more content past this
  // edge", so they're tied to the body's actual scroll position: a dialog whose
  // content fits shows none and reads as one uninterrupted surface. Recomputed
  // after every update because the body's height changes as matchers are added,
  // revealed or removed — not just when the user scrolls.
  let bodyEl: HTMLDivElement | null = null;
  let scrolledPastTop = false;
  let scrolledBeforeEnd = false;

  function updateScrollEdges() {
    if (!bodyEl) return;
    scrolledPastTop = bodyEl.scrollTop > 1;
    scrolledBeforeEnd = bodyEl.scrollTop + bodyEl.clientHeight < bodyEl.scrollHeight - 1;
  }

  afterUpdate(updateScrollEdges);

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
  <div
    class="overlay"
    in:scrim={{ duration: PRESENT_MS }}
    out:scrim={{ duration: DISMISS_MS }}
    on:click={close}
    on:keydown={handleKeydown}
    role="presentation"
  >
    <div
      class="dialog"
      in:panel={{ duration: PRESENT_MS, easing: easePanel }}
      out:panel={{ duration: DISMISS_MS, easing: easeDismiss }}
      on:click|stopPropagation
      on:keydown|stopPropagation
      role="dialog"
      aria-modal="true"
      aria-labelledby="silence-title"
    >
      <div class="dialog-header" class:divided={scrolledPastTop}>
        <h3 id="silence-title">{mode === 'edit' ? 'Edit silence' : isScratchCreate ? 'New silence' : 'Silence alert'}</h3>
        <button class="btn-close" on:click={close} aria-label="Close">
          <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round">
            <path d="M18 6 6 18M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div class="dialog-body" bind:this={bodyEl} on:scroll={updateScrollEdges}>
        {#if mode === 'edit' && silence}
          <section class="section">
            <span class="section-label">Silence</span>
            <div class="group">
              <div class="row">
                <span class="row-label">ID</span>
                <span class="row-value mono">{silence.id.slice(0, 10)}…</span>
              </div>
              <div class="row">
                <span class="row-label">Started</span>
                <span class="row-value">{new Date(silence.startsAt).toLocaleString()}</span>
              </div>
              <div class="row">
                <span class="row-label">Created by</span>
                <span class="row-value">{silence.createdBy}</span>
              </div>
              <div class="row">
                <span class="row-label">Expires in</span>
                <span class="row-value">{formatRemaining(silence.endsAt)}</span>
              </div>
            </div>
          </section>
        {:else if alert}
          <section class="section">
            <span class="section-label">Alert</span>
            <div class="group">
              <div class="row">
                <span class="alert-name">{alert.name}</span>
                <span class="alert-source">{alert.source}</span>
              </div>
            </div>
          </section>
        {/if}

        {#if isAlertlessCreate}
          <section class="section">
            <span class="section-label">Target source</span>
            <div class="group">
              <SelectMenu
                ariaLabel="Target source"
                placeholder="No silence-capable sources"
                options={sourceCandidates.map((c) => ({ value: c.source, label: `${c.source} (${c.count})` }))}
                bind:value={selectedSource}
              />
            </div>
          </section>
        {/if}

        <section class="section">
          <span class="section-label">
            Matchers
            {#if allMatchers.length}<span class="section-count">{allMatchers.length}</span>{/if}
          </span>
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
                  Show {hiddenMatchers.length} more
                </button>
              {:else if canCollapse}
                <button type="button" class="matcher-toggle" on:click={collapseMatchers}>
                  Hide matchers
                </button>
              {/if}
            </svelte:fragment>
          </MatcherEditor>
          {#if previewValid && activeSource}
            <p class="section-note preview" class:warn={previewWarn}>
              Matches {previewMatchCount} of {previewTotalOnSource} on {activeSource}
            </p>
          {/if}
          {#if droppedTerms.length > 0}
            <p class="section-note">
              Not included in silence:
              {#each droppedTerms as d, i}
                <code>{d.label}</code><span class="dropped-reason"> ({d.reason})</span>{i < droppedTerms.length - 1 ? ', ' : ''}
              {/each}
            </p>
          {/if}
        </section>

        <section class="section">
          <span class="section-label">Ends in</span>
          <div class="group group-fields">
            <div class="row row-field">
              <input
                class="bare-input"
                type="text"
                aria-label="Duration"
                bind:value={duration}
                placeholder="e.g. 2h, 1h30m, 45m"
              />
            </div>
          </div>
          <div class="segmented" role="group" aria-label="Duration presets">
            {#each basePresets as p}
              <button
                type="button"
                class="segment"
                class:selected={duration === p}
                aria-pressed={duration === p}
                on:click={() => setDurationPreset(p)}
              >{p}</button>
            {/each}
          </div>
          {#if mode === 'edit'}
            <div class="steppers">
              {#each extendPresets as p}
                <button type="button" class="stepper" on:click={() => extendDuration(p)}>{p}</button>
              {/each}
            </div>
          {/if}
        </section>

        <section class="section">
          <span class="section-label">Details</span>
          <div class="group group-fields">
            <div class="row row-block">
              <textarea
                class="bare-input textarea"
                aria-label="Comment"
                bind:value={comment}
                placeholder="Reason for silencing…"
                rows="3"
              />
            </div>
            <label class="row">
              <span class="row-label">Created by</span>
              <input class="bare-input align-end" type="text" bind:value={createdBy} placeholder="Username" />
            </label>
          </div>
        </section>

        {#if error}
          <p class="error">{error}</p>
        {/if}
      </div>

      <div class="dialog-footer" class:divided={scrolledBeforeEnd}>
        <div class="footer-left">
          {#if mode === 'edit' && silence}
            {#if confirmExpire}
              <span class="expire-confirm-text">Expire this silence now?</span>
              <button class="btn btn-danger" on:click={doExpire} disabled={loading}>
                {loading ? 'Expiring…' : 'Expire'}
              </button>
              <button class="btn btn-quiet" on:click={() => (confirmExpire = false)} disabled={loading}>
                Keep
              </button>
            {:else}
              <button class="btn btn-quiet btn-destructive" on:click={() => (confirmExpire = true)} disabled={loading}>
                Expire now
              </button>
            {/if}
          {/if}
        </div>
        <!-- Hidden while the expire confirmation is armed: leaving Save next to
             a live Expire invites the exact misclick the confirmation exists to
             prevent. -->
        {#if !confirmExpire}
          <div class="footer-right">
            <button class="btn btn-quiet" on:click={close} disabled={loading}>Cancel</button>
            <button class="btn btn-primary" on:click={submit} disabled={!canSubmit}>
              {loading ? (mode === 'edit' ? 'Saving…' : 'Silencing…') : mode === 'edit' ? 'Save changes' : 'Silence'}
            </button>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  /* The scrim frosts the list rather than just dimming it, so the panel reads
     as floating above a live window instead of over a flat grey sheet. */
  .overlay {
    position: fixed;
    inset: 0;
    background: var(--panel-scrim);
    -webkit-backdrop-filter: blur(20px) saturate(140%);
    backdrop-filter: blur(20px) saturate(140%);
    display: flex;
    /* Top-anchored, NOT centred. Centring re-positions the panel every time its
       height changes, so removing a matcher moved the panel down while the rows
       moved up — the two don't cancel, and the next row's ✕ landed somewhere
       other than under the pointer. Anchored, a removal shifts the rows below
       by exactly one row height, putting the next ✕ where the last one was.
       (It also matches how a macOS sheet hangs from the top of its window.) */
    align-items: flex-start;
    justify-content: center;
    /* The overlay covers the whole window, titlebar included, so on macOS the
       panel would otherwise start under the traffic lights. Reserve the same
       inset the chrome band uses; --titlebar-min-h is 0 on platforms that draw
       their own titlebar, where the plain 16px wins. */
    padding: max(16px, calc(var(--titlebar-min-h) + 10px)) 16px 16px;
    z-index: 1000;
  }
  .dialog {
    background: var(--panel-bg);
    border: 1px solid var(--panel-border);
    border-radius: 12px;
    /* Fluid, not fixed. Matcher rows are the widest thing in here and long
       label names (app_kubernetes_io_component) truncate at the old 520px, so
       the panel takes the width the window can spare — capped, because a modal
       that keeps growing stops reading as a modal. */
    width: 100%;
    max-width: 720px;
    max-height: 100%;
    display: flex;
    flex-direction: column;
    /* The inset highlight is the top edge catching light; without it a large
       radius on a dark fill just looks like a hole. */
    box-shadow: var(--panel-shadow), inset 0 1px 0 rgba(255, 255, 255, 0.06);
  }

  .dialog-header {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    padding: 10px 40px;
    border-bottom: 1px solid transparent;
    transition: border-color 0.15s;
  }
  .dialog-header.divided { border-bottom-color: var(--chrome-hairline); }
  h3 {
    margin: 0;
    font-size: calc(13.5px * var(--font-scale, 1));
    font-weight: 600;
    letter-spacing: -0.01em;
    color: #f1f5f9;
    text-align: center;
  }
  /* Same ghost circle as the search field's clear button, so the two dismiss
     affordances in the app are one control. */
  .btn-close {
    position: absolute;
    top: 50%;
    right: 12px;
    transform: translateY(-50%);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: rgba(148, 163, 184, 0.16);
    color: #b6c4d6;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .btn-close:hover { background: rgba(148, 163, 184, 0.3); color: #f1f5f9; }

  .dialog-body {
    padding: 2px 16px 13px;
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }

  /* Grouping, not boxing: a small dim caption over a hairline card. This is
     what replaces eight individually-bordered fields stacked on each other. */
  .section {
    display: flex;
    flex-direction: column;
    gap: 5px;
    margin-top: 11px;
  }
  .section-label {
    display: flex;
    align-items: center;
    gap: 6px;
    padding-left: 2px;
    font-size: calc(11px * var(--font-scale, 1));
    font-weight: 500;
    letter-spacing: 0.01em;
    color: var(--ctrl-fg-dim);
  }
  .section-count {
    min-width: 15px;
    padding: 0 4px;
    border-radius: 7px;
    background: rgba(148, 163, 184, 0.16);
    color: var(--ctrl-fg-dim);
    font-size: calc(9.5px * var(--font-scale, 1));
    font-weight: 600;
    line-height: 15px;
    text-align: center;
  }

  .group {
    background: var(--group-bg);
    border: 1px solid var(--chrome-hairline);
    border-radius: 9px;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  /* Deliberately not `overflow: hidden` — the source picker's menu opens out
     of this card. Round the end rows instead so row fills still stop at the
     corners. */
  .group > :first-child {
    border-top-left-radius: 8px;
    border-top-right-radius: 8px;
  }
  .group > :last-child {
    border-bottom-left-radius: 8px;
    border-bottom-right-radius: 8px;
  }
  /* Only for cards holding bare inputs, which have no ring of their own. A card
     whose control rings itself (the source picker) stays plain — two nested
     rings read as one heavy blue slab. */
  .group-fields:focus-within {
    border-color: rgba(96, 165, 250, 0.6);
    box-shadow: var(--focus-ring);
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 5px 10px;
    min-height: 26px;
    box-sizing: border-box;
  }
  .row + .row { border-top: 1px solid var(--group-divider); }
  .row-block { display: block; padding: 3px 4px; }
  .row-field { display: block; padding: 2px 4px; }
  .row-label {
    flex-shrink: 0;
    font-size: calc(12px * var(--font-scale, 1));
    color: var(--ctrl-fg-dim);
  }
  .row-value {
    min-width: 0;
    font-size: calc(12px * var(--font-scale, 1));
    color: var(--ctrl-fg);
    text-align: right;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
  .alert-name {
    min-width: 0;
    font-size: calc(12.5px * var(--font-scale, 1));
    font-weight: 600;
    color: #f1f5f9;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .alert-source {
    flex-shrink: 0;
    font-size: calc(11px * var(--font-scale, 1));
    color: #64748b;
  }

  /* Bare fields: the group card supplies the border and the focus ring, so the
     controls inside it carry none of their own. */
  .bare-input {
    width: 100%;
    border: none;
    background: transparent;
    outline: none;
    color: var(--ctrl-fg);
    font-family: inherit;
    font-size: calc(12.5px * var(--font-scale, 1));
    padding: 3px 6px;
    box-sizing: border-box;
  }
  .align-end { text-align: right; }
  .textarea {
    display: block;
    /* No grip: on a borderless field inside a card it reads as a stray
       artifact, and the dialog lives in a fixed-height popup anyway. */
    resize: none;
    height: 46px;
    line-height: 1.45;
  }
  .bare-input::placeholder { color: #5b6b83; }

  /* Segmented control: one track, and the selection is a raised pill inside it
     rather than eight separate buttons each drawing its own border. */
  .segmented {
    display: flex;
    height: 26px;
    padding: 2px;
    gap: 2px;
    border: 1px solid var(--chrome-hairline);
    border-radius: 8px;
    background: var(--chrome-capsule-bg);
    box-sizing: border-box;
  }
  .segment {
    flex: 1;
    min-width: 0;
    padding: 0 2px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--ctrl-fg-dim);
    font-family: inherit;
    font-size: calc(11px * var(--font-scale, 1));
    font-variant-numeric: tabular-nums;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .segment:hover { background: var(--chrome-ctrl-hover); color: var(--ctrl-fg); }
  .segment.selected {
    background: rgba(255, 255, 255, 0.13);
    color: #f1f5f9;
    font-weight: 600;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.09);
  }
  .segment.selected:hover { background: rgba(255, 255, 255, 0.16); }

  /* Extend shortcuts are actions, not a selection — so they stay discrete
     buttons and deliberately do NOT look like the segmented control above. */
  .steppers { display: flex; gap: 5px; }
  .stepper {
    height: 22px;
    padding: 0 9px;
    border: 1px solid var(--chrome-hairline);
    border-radius: 6px;
    background: transparent;
    color: var(--ctrl-fg-dim);
    font-family: inherit;
    font-size: calc(11px * var(--font-scale, 1));
    font-variant-numeric: tabular-nums;
    cursor: pointer;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
  }
  .stepper:hover {
    background: var(--chrome-ctrl-hover);
    border-color: var(--chrome-field-border);
    color: var(--ctrl-fg);
  }

  .matcher-toggle {
    background: none;
    border: none;
    color: var(--ctrl-fg-dim);
    font-size: calc(11px * var(--font-scale, 1));
    cursor: pointer;
    padding: 2px 0;
    white-space: nowrap;
  }
  .matcher-toggle:hover { color: var(--ctrl-fg); }

  .section-note {
    font-size: calc(11px * var(--font-scale, 1));
    color: var(--ctrl-fg-dim);
    margin: 0;
    padding-left: 2px;
    line-height: 1.5;
  }
  .section-note code {
    background: rgba(148, 163, 184, 0.14);
    border-radius: 4px;
    padding: 1px 5px;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    color: #cbd5e1;
  }
  .dropped-reason { color: #64748b; }
  /* Sits under the matcher card because that is what it reports on — in the
     footer it was just another thing competing with the buttons. */
  .preview.warn { color: #fbbf24; }

  .error {
    color: var(--danger);
    font-size: calc(12px * var(--font-scale, 1));
    margin: 11px 0 0;
  }

  .dialog-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-shrink: 0;
    gap: 10px;
    padding: 10px 14px;
    border-top: 1px solid transparent;
    transition: border-color 0.15s;
  }
  .dialog-footer.divided { border-top-color: var(--chrome-hairline); }
  .footer-left { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .footer-right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
  .expire-confirm-text {
    font-size: calc(11.5px * var(--font-scale, 1));
    color: var(--danger);
  }

  .btn {
    height: 26px;
    padding: 0 13px;
    border-radius: 7px;
    border: 1px solid transparent;
    cursor: pointer;
    font-family: inherit;
    font-size: calc(12.5px * var(--font-scale, 1));
    font-weight: 500;
    white-space: nowrap;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }
  .btn:disabled { opacity: 0.45; cursor: not-allowed; }
  .btn:focus-visible { outline: none; box-shadow: var(--focus-ring); }

  .btn-quiet {
    background: var(--chrome-capsule-bg);
    border-color: var(--chrome-hairline);
    color: var(--ctrl-fg);
  }
  .btn-quiet:hover:not(:disabled) {
    background: var(--chrome-ctrl-hover);
    border-color: var(--chrome-field-border);
  }

  .btn-primary {
    background: var(--accent);
    color: #fff;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16);
  }
  .btn-primary:hover:not(:disabled) { background: var(--accent-hover); }

  /* Destructive weight lands on the step that actually destroys: arming the
     confirmation is quiet red text, only Confirm goes solid. */
  .btn-destructive { color: var(--danger); }
  .btn-destructive:hover:not(:disabled) {
    background: rgba(248, 113, 113, 0.12);
    border-color: rgba(248, 113, 113, 0.4);
    color: #fca5a5;
  }
  .btn-danger { background: #dc2626; color: #fff; }
  .btn-danger:hover:not(:disabled) { background: #b91c1c; }
</style>
