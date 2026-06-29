<script lang="ts">
  import { onMount } from 'svelte';
  import { BrowserOpenURL, EventsOn } from '../wailsjs/runtime/runtime';
  import { isWails } from './stores/alerts';
  import AlertList from './components/AlertList.svelte';
  import About from './components/About.svelte';
  import { initUIScale, uiScale } from './stores/uiScale';

  let view: 'list' | 'about' = 'list';

  $: {
    const factor = String($uiScale.factor);
    const interfaceScale = $uiScale.mode === 'interface' ? factor : '1';
    document.documentElement.style.setProperty('--font-scale', factor);
    document.documentElement.style.setProperty('--ui-scale', interfaceScale);
    document.getElementById('app')?.classList.toggle('scale-interface', $uiScale.mode === 'interface');
  }

  onMount(() => {
    if (!isWails()) return;

    const unlistenScale = initUIScale();
    const unlisten = EventsOn('about:show', () => {
      view = 'about';
    });

    const onClick = (e: MouseEvent) => {
      const anchor = (e.target as HTMLElement).closest('a[href]');
      if (!anchor) return;
      const href = anchor.getAttribute('href');
      if (href && /^https?:\/\//.test(href)) {
        e.preventDefault();
        BrowserOpenURL(href);
      }
    };
    document.addEventListener('click', onClick);

    return () => {
      unlistenScale();
      unlisten();
      document.removeEventListener('click', onClick);
    };
  });
</script>

<main>
  {#if view === 'about'}
    <About on:back={() => (view = 'list')} />
  {:else}
    <AlertList />
  {/if}
</main>

<style>
  :global(*) {
    box-sizing: border-box;
  }

  :global(html, body) {
    margin: 0;
    padding: 0;
    height: 100%;
    background: #0f172a;
    color: #e2e8f0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: calc(13px * var(--font-scale, 1));
  }

  :global(#app) {
    height: 100%;
  }

  main {
    height: 100%;
  }
</style>
