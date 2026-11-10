<script lang="ts">
  // Wraps a silence control. When `reason` is set the control is disabled for
  // a permission reason (the control shows a PermissionIcon inside), and
  // hovering shows the reason at once (the native title waits ~1s and disabled
  // buttons get no mouse events in WebKit). Screen readers read `id`'s text via
  // the control's aria-describedby.
  import { hint } from '../utils/hint';

  export let reason: string = '';
  export let id: string;
</script>

<span class="permission-wrap" use:hint={reason}>
  <slot />
  {#if reason}
    <span class="sr-only" {id}>{reason}</span>
  {/if}
</span>

<style>
  .permission-wrap {
    display: inline-flex;
    align-items: center;
    gap: 4px;
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
