<script lang="ts">
  import {
    dismissConfigDiagnostics,
    visibleConfigDiagnostics,
    type ConfigDiagnostic,
  } from '../stores/diagnostics';
  import { isWails } from '../stores/alerts';
  import { platform } from '../stores/platform';
  import { RevealConfigFile } from '../../wailsjs/go/main/App';
  import Notice from './Notice.svelte';

  let revealError = '';

  $: items = $visibleConfigDiagnostics?.items ?? [];
  $: path = $visibleConfigDiagnostics?.path ?? '';

  // A config that would not parse at all arrives as a single item filed under
  // "config". It is a different card from a list of field problems: there is
  // nothing to enumerate, so the title carries the outcome and the body opens
  // straight onto the one thing that went wrong.
  $: configUnusable = items.length === 1 && items[0].field === 'config';
  $: title = configUnusable
    ? 'Config not loaded'
    : items.length === 1 ? 'Config problem' : 'Config problems';

  // A single problem has nothing to count and nothing to hide behind a
  // disclosure; the summary would just repeat the one row underneath it.
  $: enumerated = items.length > 1;

  $: pathCut = path.lastIndexOf('/');
  $: directory = pathCut > 0 ? path.slice(0, pathCut + 1) : '';
  $: filename = pathCut > 0 ? path.slice(pathCut + 1) : path;

  $: revealLabel = $platform === 'darwin' ? 'Reveal in Finder' : 'Show in Folder';

  function locatorOf(item: ConfigDiagnostic): string {
    if (item.locator) return item.locator;
    // "config" is an identity, not a place — an unreadable file has no position
    // to point at, so the cell stays empty rather than showing a filler token.
    return item.field === 'config' ? '' : item.field;
  }

  function outcomeOf(item: ConfigDiagnostic): string {
    // The title already says the config did not load; repeating it per row is
    // the duplication this layout exists to remove.
    if (item.field === 'config') return '';
    return item.dropped ? 'Entry not loaded' : 'Using default';
  }

  async function handleReveal() {
    revealError = '';
    try {
      await RevealConfigFile();
    } catch (e) {
      revealError = String(e);
    }
  }
</script>

{#if $visibleConfigDiagnostics}
  <Notice
    severity="caution"
    {title}
    count={enumerated ? items.length : null}
    subtitle={enumerated ? items.map(item => item.field).join(', ') : ''}
    collapsible={enumerated}
  >
    <button slot="action" type="button" class="notice-action" on:click={dismissConfigDiagnostics}>
      Dismiss
    </button>

    <!-- One grid for every problem, so the locator column lines up down the
       list instead of each row boxing its own. -->
    <div class="notice-rows">
      {#each items as item}
        <span class="notice-row-locator">{locatorOf(item)}</span>
        <p class="notice-row-text">{item.message}</p>
        <span class="notice-row-tag">{outcomeOf(item)}</span>
      {/each}
    </div>

    {#if path}
      <div class="notice-footer">
        <span class="notice-footer-path" title={path}>
          <span class="notice-footer-dir">{directory}</span>{filename}
        </span>
        <span class="notice-footer-hint">Rechecks on save</span>
        {#if isWails()}
          <button type="button" class="notice-action" on:click={handleReveal}>{revealLabel}</button>
        {/if}
      </div>
    {/if}
    {#if revealError}
      <p class="notice-row-text">{revealError}</p>
    {/if}
  </Notice>
{/if}
