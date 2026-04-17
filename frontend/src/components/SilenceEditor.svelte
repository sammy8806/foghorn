<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { GetUIConfig, CreateSilence, UpdateSilence, Unsilence } from '../../wailsjs/go/main/App';
  import type { Alert, Matcher, SilenceInfo } from '../stores/alerts';
  import MatcherEditor from './MatcherEditor.svelte';

  export let alert: Alert | null = null;
  export let silence: SilenceInfo | null = null;
  export let mode: 'create' | 'edit' = 'create';
  export let open = false;

  const dispatch = createEventDispatcher<{ close: void; silenced: void }>();

  let matchers: Matcher[] = [];
  let duration = '2h';
  let createdBy = '';
  let comment = '';
  let loading = false;
  let error = '';
  let confirmExpire = false;
  let initializedForOpen = false;

  const basePresets = ['30m', '1h', '2h', '4h', '8h', '24h'];
  const extendPresets = ['+30m', '+1h', '+4h', '+1d'];

  $: canSubmit =
    !loading &&
    !!duration &&
    !!createdBy.trim() &&
    matchers.length > 0 &&
    matchers.every((m) => m.name.trim() && m.value && regexValid(m));

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

  // DURATION_RE accepts 1d, 2h, 30m, 10s and any concatenation (e.g. "1d2h30m").
  // The regex requires at least one group present (enforced by post-match check).
  const DURATION_RE = /^\s*(?:(\d+)d)?\s*(?:(\d+)h)?\s*(?:(\d+)m)?\s*(?:(\d+)s)?\s*$/i;

  function parseDurationMs(s: string): number | null {
    const trimmed = (s || '').trim();
    if (!trimmed) return null;
    const match = trimmed.match(DURATION_RE);
    if (!match || match.slice(1).every((g) => !g)) return null;
    const [, d, h, m, sec] = match;
    const days = d ? parseInt(d, 10) : 0;
    const hours = h ? parseInt(h, 10) : 0;
    const mins = m ? parseInt(m, 10) : 0;
    const secs = sec ? parseInt(sec, 10) : 0;
    return (((days * 24 + hours) * 60 + mins) * 60 + secs) * 1000;
  }

  function roundDuration(ms: number): string {
    if (ms <= 0) return '0s';
    // Round to the nearest minute for a clean unit string.
    const totalMins = Math.max(1, Math.round(ms / 60000));
    const hours = Math.floor(totalMins / 60);
    const minutes = totalMins % 60;
    const parts: string[] = [];
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

  function resetForOpen() {
    error = '';
    loading = false;
    confirmExpire = false;
    if (mode === 'edit' && silence && alert) {
      matchers = cloneMatchers(silence.matchers);
      if (!matchers.length) {
        // Safety net: silence without matchers is unusual but don't nuke the editor.
        matchers = matchersFromAlertLabels(alert);
      }
      const endMs = new Date(silence.endsAt).getTime() - Date.now();
      duration = roundDuration(Math.max(0, endMs));
      if (duration === '0s') duration = '1m';
      comment = silence.comment || '';
      createdBy = (silence.createdBy || '').trim();
    } else {
      matchers = alert ? matchersFromAlertLabels(alert) : [];
      duration = '2h';
      comment = '';
      createdBy = '';
      void loadDefaultCreatedBy().then((v) => {
        if (!createdBy) createdBy = v;
      });
    }
  }

  $: if (open && !initializedForOpen) {
    initializedForOpen = true;
    resetForOpen();
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
    if (!alert || !canSubmit) return;
    loading = true;
    error = '';
    try {
      if (mode === 'edit' && silence) {
        await UpdateSilence(alert.source, silence.id, matchers, duration, createdBy, comment);
      } else {
        await CreateSilence(alert.source, matchers, duration, createdBy, comment);
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

{#if open && alert}
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
        <h3 id="silence-title">{mode === 'edit' ? 'Edit silence' : 'Silence alert'}</h3>
        <button class="btn-close" on:click={close} aria-label="Close">✕</button>
      </div>

      <div class="dialog-body">
        <div class="context-strip">
          {#if mode === 'edit' && silence}
            <span class="ctx-item"><strong>id:</strong> {silence.id.slice(0, 10)}…</span>
            <span class="ctx-item"><strong>started:</strong> {new Date(silence.startsAt).toLocaleString()}</span>
            <span class="ctx-item"><strong>by:</strong> {silence.createdBy}</span>
            <span class="ctx-item"><strong>expires in:</strong> {formatRemaining(silence.endsAt)}</span>
          {:else}
            <span class="alert-name">{alert.name}</span>
            <span class="alert-source">{alert.source}</span>
          {/if}
        </div>

        <div class="field">
          <span class="field-label">Matchers</span>
          <MatcherEditor bind:matchers source={alert.source} />
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
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    width: 520px;
    max-width: 92vw;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }
  .dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid #334155;
  }
  h3 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    color: #f1f5f9;
  }
  .btn-close {
    background: none;
    border: none;
    color: #64748b;
    cursor: pointer;
    font-size: 14px;
    padding: 2px 6px;
  }
  .btn-close:hover { color: #e2e8f0; }

  .dialog-body { padding: 14px 18px; }

  .context-strip {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
    align-items: baseline;
    margin-bottom: 14px;
    padding: 6px 10px;
    background: #0f172a;
    border-radius: 4px;
    font-size: 11px;
    color: #94a3b8;
  }
  .ctx-item strong { color: #cbd5e1; margin-right: 3px; font-weight: 600; }
  .alert-name { color: #f1f5f9; font-weight: 600; font-size: 13px; }
  .alert-source { color: #64748b; font-size: 11px; }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 12px;
    font-size: 12px;
    color: #94a3b8;
  }
  .field-label { font-weight: 500; color: #94a3b8; }

  .input {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 4px;
    color: #e2e8f0;
    font-size: 13px;
    padding: 6px 10px;
    outline: none;
    width: 100%;
    box-sizing: border-box;
  }
  .input:focus { border-color: #3b82f6; }
  .textarea { resize: vertical; font-family: inherit; }

  .presets {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .preset-btn {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 3px;
    color: #94a3b8;
    cursor: pointer;
    font-size: 11px;
    padding: 3px 8px;
  }
  .preset-btn:hover { border-color: #3b82f6; color: #e2e8f0; }
  .preset-btn.active { border-color: #3b82f6; background: #1e40af; color: #fff; }

  .error {
    color: #f87171;
    font-size: 12px;
    margin: 8px 0 0;
  }

  .dialog-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 12px 18px;
    border-top: 1px solid #334155;
  }
  .footer-left { display: flex; align-items: center; gap: 8px; }
  .footer-right { display: flex; align-items: center; gap: 8px; }
  .expire-confirm-text {
    font-size: 12px;
    color: #f87171;
  }

  .btn {
    border-radius: 4px;
    border: none;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    padding: 7px 16px;
  }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .btn-cancel { background: #334155; color: #94a3b8; }
  .btn-cancel:hover:not(:disabled) { background: #475569; }

  .btn-primary { background: #3b82f6; color: #fff; }
  .btn-primary:hover:not(:disabled) { background: #2563eb; }

  .btn-expire { background: #7f1d1d; color: #fecaca; }
  .btn-expire:hover:not(:disabled) { background: #991b1b; color: #fff; }
</style>
