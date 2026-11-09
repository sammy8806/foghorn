<script lang="ts">
  // Wraps a silence control. When `reason` is set the control is disabled for
  // a permission reason: a small yellow triangle sits next to it, and hovering
  // either shows the reason at once (the native title waits ~1s and disabled
  // buttons get no mouse events in WebKit). Screen readers read `id`'s text via
  // the control's aria-describedby.
  import { hint } from '../utils/hint';

  export let reason: string = '';
  export let id: string;
</script>

<span class="permission-wrap" use:hint={reason}>
  <slot />
  {#if reason}
    <svg class="permission-warn" viewBox="0 0 24 24" width="13" height="13" aria-hidden="true">
      <path d="M12 3 2 21h20L12 3z" fill="#fbbf24" stroke="#fbbf24" stroke-width="1.5" stroke-linejoin="round" />
      <path d="M12 10v5" stroke="#1f2937" stroke-width="2.2" stroke-linecap="round" />
      <circle cx="12" cy="18" r="1.2" fill="#1f2937" />
    </svg>
    <span class="sr-only" {id}>{reason}</span>
  {/if}
</span>

<style>
  .permission-wrap {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .permission-warn {
    flex: none;
    cursor: help;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
</style>
