<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { GetAbout } from '../../wailsjs/go/main/App';
  import type { main } from '../../wailsjs/go/models';
  import logo from '../assets/images/logo-universal.png';

  const dispatch = createEventDispatcher<{ back: void }>();

  let info: main.AboutInfo = {
    name: 'Foghorn',
    version: '',
    description: '',
    repoURL: '',
    copyright: '',
  };

  onMount(async () => {
    try {
      info = await GetAbout();
    } catch (e) {
      // Non-Wails (browser dev) or bridge not ready: keep fallback defaults.
      console.warn('GetAbout failed', e);
    }
  });
</script>

<section class="about">
  <header>
    <button class="back" type="button" on:click={() => dispatch('back')}>← Back</button>
  </header>

  <div class="body">
    <img class="logo" src={logo} alt="Foghorn logo" width="72" height="72" />
    <h1>{info.name}{#if info.version} <span class="version">v{info.version}</span>{/if}</h1>
    {#if info.description}<p class="description">{info.description}</p>{/if}
    {#if info.repoURL}
      <p class="links"><a href={info.repoURL}>GitHub repository</a></p>
    {/if}
    {#if info.copyright}<p class="copyright">{info.copyright}</p>{/if}
  </div>
</section>

<style>
  .about {
    height: 100%;
    display: flex;
    flex-direction: column;
    color: var(--color-text);
  }

  header {
    padding: 8px 12px;
    border-bottom: 1px solid var(--color-border);
  }

  .back {
    background: transparent;
    border: none;
    color: var(--color-interactive);
    cursor: pointer;
    font-size: calc(13px * var(--font-scale, 1));
    padding: 0;
  }

  .back:hover {
    text-decoration: underline;
  }

  .body {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    gap: 8px;
    padding: 16px;
  }

  .logo {
    border-radius: 12px;
  }

  h1 {
    font-size: calc(18px * var(--font-scale, 1));
    margin: 4px 0 0;
    font-weight: 600;
  }

  .version {
    color: var(--color-text-muted);
    font-weight: 400;
    font-size: calc(14px * var(--font-scale, 1));
  }

  .description {
    margin: 0;
    color: var(--color-text);
    max-width: 260px;
  }

  .links {
    margin: 4px 0 0;
  }

  .links a {
    color: var(--color-interactive);
    text-underline-offset: 2px;
  }

  .copyright {
    margin: 8px 0 0;
    color: var(--color-text-faint);
    font-size: calc(12px * var(--font-scale, 1));
  }
</style>
