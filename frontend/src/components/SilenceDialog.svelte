<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { GetUIConfig } from '../../wailsjs/go/main/App';
  import { SilenceAlert } from '../../wailsjs/go/main/App';
  import type { Alert } from '../stores/alerts';

  export let alert: Alert | null = null;
  export let open = false;

  const dispatch = createEventDispatcher<{ close: void; silenced: void }>();

  let duration = '2h';
  let createdBy = '';
  let comment = '';
  let loading = false;
  let error = '';
  let initializedForOpen = false;

  const presets = ['30m', '1h', '2h', '4h', '8h', '24h'];

  async function loadDefaults() {
    try {
      const uiConfig = await GetUIConfig();
      const uiConfigAny = uiConfig as any;
      const resolvedCreatedBy =
        uiConfig.default_created_by ??
        uiConfigAny?.DefaultCreatedBy ??
        uiConfigAny?.defaultCreatedBy ??
        '';
      createdBy = resolvedCreatedBy.trim();
    } catch {
      createdBy = '';
    }
  }

  async function submit() {
    if (!alert) return;
    loading = true;
    error = '';
    try {
      await SilenceAlert(alert.id, alert.source, duration, createdBy, comment);
      dispatch('silenced');
      dispatch('close');
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
    }
  }

  function close() {
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (e.key === 'Enter' && !loading) submit();
  }

  $: if (open && !initializedForOpen) {
    initializedForOpen = true;
    duration = '2h';
    comment = '';
    error = '';
    createdBy = '';
    void loadDefaults();
  }

  $: if (!open) {
    initializedForOpen = false;
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
        <h3 id="silence-title">Silence Alert</h3>
        <button class="btn-close" on:click={close} aria-label="Close">✕</button>
      </div>

      <div class="dialog-body">
        <div class="alert-summary">
          <span class="alert-name">{alert.name}</span>
          <span class="alert-source">{alert.source}</span>
        </div>

        <label class="field">
          <span>Duration</span>
          <div class="duration-row">
            <input
              class="input"
              type="text"
              bind:value={duration}
              placeholder="e.g. 2h, 30m"
            />
            <div class="presets">
              {#each presets as preset}
                <button
                  class="preset-btn"
                  class:active={duration === preset}
                  on:click={() => (duration = preset)}
                >{preset}</button>
              {/each}
            </div>
          </div>
        </label>

        <label class="field">
          <span>Created by</span>
          <input
            class="input"
            type="text"
            bind:value={createdBy}
            placeholder="Username"
          />
        </label>

        <label class="field">
          <span>Comment</span>
          <textarea
            class="input textarea"
            bind:value={comment}
            placeholder="Reason for silencing…"
            rows="3"
          />
        </label>

        {#if error}
          <p class="error">{error}</p>
        {/if}
      </div>

      <div class="dialog-footer">
        <button class="btn btn-cancel" on:click={close} disabled={loading}>Cancel</button>
        <button class="btn btn-primary" on:click={submit} disabled={loading || !duration || !createdBy.trim()}>
          {loading ? 'Silencing…' : 'Silence'}
        </button>
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
    width: 420px;
    max-width: 90vw;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  }

  .dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
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

  .dialog-body { padding: 16px 20px; }

  .alert-summary {
    display: flex;
    gap: 8px;
    align-items: baseline;
    margin-bottom: 16px;
    padding: 8px 12px;
    background: #0f172a;
    border-radius: 4px;
  }

  .alert-name {
    font-weight: 600;
    font-size: 13px;
    color: #f1f5f9;
  }

  .alert-source {
    font-size: 11px;
    color: #64748b;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 14px;
    font-size: 12px;
    color: #94a3b8;
  }

  .duration-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

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
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 20px;
    border-top: 1px solid #334155;
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

  .btn-cancel {
    background: #334155;
    color: #94a3b8;
  }
  .btn-cancel:hover:not(:disabled) { background: #475569; }

  .btn-primary {
    background: #3b82f6;
    color: #fff;
  }
  .btn-primary:hover:not(:disabled) { background: #2563eb; }
</style>
