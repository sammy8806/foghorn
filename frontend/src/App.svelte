<script lang="ts">
  import { onMount } from 'svelte';
  import { BrowserOpenURL, EventsOn } from '../wailsjs/runtime/runtime';
  import { isWails } from './stores/alerts';
  import AlertList from './components/AlertList.svelte';
  import About from './components/About.svelte';

  let view: 'list' | 'about' = 'list';

  onMount(() => {
    if (!isWails()) return;

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
    font-size: 13px;
  }

  :global(#app) {
    height: 100%;
  }

  main {
    height: 100%;
  }
</style>
