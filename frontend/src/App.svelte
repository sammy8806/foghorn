<script lang="ts">
  import AlertList from './components/AlertList.svelte';
</script>

<main>
  <AlertList />
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
