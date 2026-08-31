<script lang="ts">
  import { onMount } from 'svelte';
  import { BrowserOpenURL, EventsOn } from '../wailsjs/runtime/runtime';
  import { isWails, waitForBridge } from './stores/alerts';
  import AlertList from './components/AlertList.svelte';
  import About from './components/About.svelte';
  import { initConfigDiagnostics } from './stores/diagnostics';
  import { syncPlatform } from './stores/platform';
  import { initUIScale, uiScale } from './stores/uiScale';
  import { safeExternalURL } from './utils/url';

  let view: 'list' | 'about' = 'list';

  $: {
    const factor = String($uiScale.factor);
    const interfaceScale = $uiScale.mode === 'interface' ? factor : '1';
    document.documentElement.style.setProperty('--font-scale', factor);
    document.documentElement.style.setProperty('--ui-scale', interfaceScale);
    document.getElementById('app')?.classList.toggle('scale-interface', $uiScale.mode === 'interface');
  }

  onMount(() => {
    let unlistenScale = () => {};
    let unlistenDiagnostics = () => {};
    let unlistenAbout = () => {};
    let clickInstalled = false;
    let disposed = false;

    // Fail closed: every in-app anchor click is cancelled, and only URLs that
    // validate as http/https are handed to the system browser. Letting a click
    // fall through would navigate the webview itself — which has no navigation
    // policy handler, so a `javascript:` or attacker-page href from a hostile
    // alert source would run with the Wails bridge in reach.
    const onClick = (e: MouseEvent) => {
      const anchor = (e.target as HTMLElement).closest('a[href]');
      if (!anchor) return;
      e.preventDefault();
      const href = safeExternalURL(anchor.getAttribute('href'));
      if (href) {
        BrowserOpenURL(href);
      }
    };

    const init = async () => {
      await waitForBridge();
      if (disposed || !isWails()) return;

      // Replaces main.ts's user-agent guess with the runtime's answer before
      // anything else touches the chrome variables.
      await syncPlatform();

      unlistenScale = initUIScale();
      unlistenDiagnostics = initConfigDiagnostics();
      unlistenAbout = EventsOn('about:show', () => {
        view = 'about';
      });
      document.addEventListener('click', onClick);
      clickInstalled = true;
    };

    void init();

    return () => {
      disposed = true;
      unlistenScale();
      unlistenDiagnostics();
      unlistenAbout();
      if (clickInstalled) document.removeEventListener('click', onClick);
    };
  });
</script>

<main>
  <div class="view">
    {#if view === 'about'}
      <About on:back={() => (view = 'list')} />
    {:else}
      <AlertList />
    {/if}
  </div>
</main>

<style>
  /* Document-level resets live in style.css so there is a single source of
     truth for the surface variables; duplicating them here would win on
     injection order and silently undo the macOS translucency. */

  /* The one layer that paints the window's tint. On macOS --surface-alpha is
     below 1 so the NSVisualEffectView blurs through; everywhere else this is
     solid slate. */
  main {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--surface-base);
  }

  .view {
    flex: 1 1 auto;
    min-height: 0;
  }
</style>
