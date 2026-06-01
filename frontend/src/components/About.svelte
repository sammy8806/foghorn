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
    color: #e2e8f0;
  }

  header {
    padding: 8px 12px;
    border-bottom: 1px solid #1e293b;
  }

  .back {
    background: transparent;
    border: none;
    color: #93c5fd;
    cursor: pointer;
    font-size: 13px;
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
    font-size: 18px;
    margin: 4px 0 0;
    font-weight: 600;
  }

  .version {
    color: #94a3b8;
    font-weight: 400;
    font-size: 14px;
  }

  .description {
    margin: 0;
    color: #cbd5e1;
    max-width: 260px;
  }

  .links {
    margin: 4px 0 0;
  }

  .links a {
    color: #93c5fd;
  }

  .copyright {
    margin: 8px 0 0;
    color: #64748b;
    font-size: 12px;
  }
</style>
