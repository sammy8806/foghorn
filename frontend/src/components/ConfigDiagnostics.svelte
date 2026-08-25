<script lang="ts">
  import {
    dismissConfigDiagnostics,
    visibleConfigDiagnostics,
  } from '../stores/diagnostics';
</script>

{#if $visibleConfigDiagnostics}
  <section class="diagnostics" role="alert" aria-labelledby="config-diagnostics-title">
    <div class="heading">
      <div>
        <strong id="config-diagnostics-title">Config problems</strong>
        <span>{$visibleConfigDiagnostics.items.length}</span>
      </div>
      <button type="button" aria-label="Dismiss config problems" on:click={dismissConfigDiagnostics}>×</button>
    </div>
    <div class="items">
      {#each $visibleConfigDiagnostics.items as item}
        <div class="item">
          <code>{item.field}</code>
          <div>
            <p>{item.message}</p>
            <small>{item.field === 'config' ? 'Configuration not loaded' : item.dropped ? 'Entry not loaded' : 'Using default'}</small>
          </div>
        </div>
      {/each}
    </div>
    {#if $visibleConfigDiagnostics.path}
      <footer>Edit <code>{$visibleConfigDiagnostics.path}</code> and save to recheck.</footer>
    {/if}
  </section>
{/if}

<style>
  .diagnostics {
    flex: 0 0 auto;
    max-height: 45%;
    overflow: auto;
    color: #fef3c7;
    background: #422006;
    border-bottom: 1px solid #92400e;
    padding: 9px 12px 8px;
  }

  .heading,
  .heading > div,
  .item {
    display: flex;
    align-items: center;
  }

  .heading {
    justify-content: space-between;
    margin-bottom: 7px;
  }

  .heading > div {
    gap: 7px;
  }

  .heading span {
    min-width: 20px;
    padding: 1px 6px;
    border-radius: 999px;
    text-align: center;
    background: #78350f;
    font-size: calc(11px * var(--font-scale, 1));
  }

  button {
    border: 0;
    background: transparent;
    color: #fde68a;
    cursor: pointer;
    font-size: calc(20px * var(--font-scale, 1));
    line-height: 1;
  }

  .items {
    display: grid;
    gap: 7px;
  }

  .item {
    align-items: flex-start;
    gap: 10px;
  }

  .item > code {
    flex: 0 0 auto;
    max-width: 42%;
    overflow-wrap: anywhere;
    color: #fcd34d;
  }

  p {
    margin: 0;
    color: #fef3c7;
  }

  small {
    color: #fbbf24;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  footer {
    margin-top: 8px;
    padding-top: 7px;
    border-top: 1px solid #78350f;
    color: #fcd34d;
    font-size: calc(11px * var(--font-scale, 1));
    overflow-wrap: anywhere;
  }
</style>
