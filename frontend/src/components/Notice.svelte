<script context="module" lang="ts">
  // Disclosure bodies need a stable id for aria-controls, and there can be
  // several notices on screen at once.
  let noticeSeq = 0;
</script>

<script lang="ts">
  // One card anatomy for every in-window problem surface: config problems, the
  // notification-permission prompt, source-polling health. Severity is four
  // custom properties and nothing else — geometry, type ladder and hover
  // language are identical across all of them, which is what stops the three
  // from drifting apart again.
  //
  // The tint is the accent hue at low alpha, not an opaque dark fill: it warms
  // whatever sits behind it the same way --chrome-tint warms the toolbar band,
  // instead of stacking a second layer of opacity on the macOS vibrancy. Detail
  // rows use the same white overlay as .group-header for that reason, and the
  // card carries no drop shadow — it is inset in the window, not floating over
  // it. Geometry keys to the chrome: 7px radius (the icon toggles and the view
  // capsule) and --chrome-gutter for the side margins, so a notice's trailing
  // edge lands on the same vertical line as an alert card's.
  export let severity: 'caution' | 'critical' = 'caution';
  export let title: string;
  /** Rendered as a pill after the title. Omit when the title already counts. */
  export let count: number | null = null;
  /** Monospace list of what the notice is about — fields, source names. */
  export let subtitle = '';
  /** Turns the summary row into a disclosure control for the default slot. */
  export let collapsible = false;

  const bodyId = `notice-body-${++noticeSeq}`;

  // Collapsed by default: the summary line carries what is wrong and where,
  // the detail is one click away.
  let expanded = false;

  function toggle() {
    if (collapsible) expanded = !expanded;
  }
</script>

<section class="notice {severity}" class:expanded role="alert">
  <!-- A disclosure has to be a real button (focus, Enter/Space, announcement);
     a notice that only states something must not be one. Svelte cannot wrap
     markup conditionally, so the element itself is what varies. -->
  <svelte:element
    this={collapsible ? 'button' : 'div'}
    class="notice-summary"
    class:interactive={collapsible}
    type={collapsible ? 'button' : undefined}
    aria-expanded={collapsible ? expanded : undefined}
    aria-controls={collapsible ? bodyId : undefined}
    on:click={toggle}
  >
    <span class="notice-dot" aria-hidden="true"></span>
    <span class="notice-title">{title}</span>
    {#if count !== null}
      <span class="notice-count">{count}</span>
    {/if}
    <!-- The subtitle exists to say what the hidden rows are about, so once
       they are on screen it is the same list read twice. -->
    {#if subtitle && !expanded}
      <span class="notice-subtitle">{subtitle}</span>
    {/if}
    {#if collapsible}
      <svg class="notice-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <polyline points="6 9 12 15 18 9"></polyline>
      </svg>
    {/if}
  </svelte:element>

  <slot name="action" />

  {#if !collapsible || expanded}
    <div class="notice-body" class:divided={collapsible} id={bodyId}>
      <slot />
    </div>
  {/if}
</section>

<style>
  .notice {
    display: grid;
    /* Summary takes the row, the action slot lands beside it, the body spans
       both. One template for the collapsed and expanded states alike. */
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 7px 10px;
    margin: 8px var(--chrome-gutter) 0;
    padding: 8px 10px;
    border: 1px solid var(--notice-hairline);
    border-radius: 7px;
    background: var(--notice-tint);
  }

  .notice.caution {
    --notice-tint: rgba(251, 191, 36, 0.08);
    --notice-hairline: rgba(251, 191, 36, 0.22);
    --notice-accent: #fbbf24;
    --notice-count-bg: rgba(251, 191, 36, 0.16);
  }

  .notice.critical {
    --notice-tint: rgba(248, 113, 113, 0.08);
    --notice-hairline: rgba(248, 113, 113, 0.24);
    --notice-accent: #f87171;
    --notice-count-bg: rgba(248, 113, 113, 0.16);
  }

  /* Reset covers the button form. padding:0 matters on WebKit in particular:
     its UA sheet gives <button> asymmetric block padding, which would sit the
     summary off the row that the div form establishes. */
  .notice-summary {
    display: flex;
    align-items: center;
    gap: 7px;
    min-width: 0;
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: left;
  }

  .notice-summary.interactive {
    cursor: pointer;
  }

  .notice-summary:focus-visible {
    outline: 2px solid var(--notice-accent);
    outline-offset: 3px;
    border-radius: 3px;
  }

  /* The severity lives here, in the count pill and in the hairline. Prose stays
     in the window's normal text hierarchy — colouring the copy is what made
     these cards read as web alerts rather than window furniture. */
  .notice-dot {
    flex-shrink: 0;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--notice-accent);
  }

  .notice-title {
    flex-shrink: 0;
    color: #e2e8f0;
    font-size: calc(11.5px * var(--font-scale, 1));
    font-weight: 600;
  }

  .notice-summary.interactive:hover .notice-title {
    color: #f8fafc;
  }

  .notice-count {
    flex-shrink: 0;
    min-width: 17px;
    padding: 1px 5px;
    border-radius: 999px;
    background: var(--notice-count-bg);
    color: var(--notice-accent);
    font-size: calc(9.5px * var(--font-scale, 1));
    font-weight: 700;
    line-height: 1.45;
    text-align: center;
  }

  .notice-subtitle {
    min-width: 0;
    overflow: hidden;
    color: #94a3b8;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10px * var(--font-scale, 1));
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .notice-chevron {
    flex-shrink: 0;
    margin-left: auto;
    /* Same dim as .segment-caret: the chevron is affordance, not severity. */
    color: #64748b;
    transition: transform 120ms ease;
  }

  .notice.expanded .notice-chevron {
    transform: rotate(180deg);
  }

  .notice-body {
    grid-column: 1 / -1;
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }

  /* Only a disclosure body gets the rule: it is closing off detail that was
     hidden a moment ago. Copy that was always visible just continues. */
  .notice-body.divided {
    padding-top: 8px;
    border-top: 1px solid var(--notice-hairline);
  }

  /* Body markup comes from the call site, so the shared sub-language is exposed
     with :global — scoped under .notice, so none of it leaks past this card. */

  /* Plain-text action in the chrome's own hover language (see .icon-toggle),
     rather than a bordered pill: it belongs to the card, not on top of it. */
  .notice :global(.notice-action) {
    flex-shrink: 0;
    align-self: center;
    border: 0;
    border-radius: 7px;
    padding: 4px 7px;
    background: transparent;
    color: var(--ctrl-fg-dim);
    font-family: inherit;
    font-size: calc(11px * var(--font-scale, 1));
    font-weight: 600;
    white-space: nowrap;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }

  .notice :global(.notice-action:hover:not(:disabled)) {
    background: var(--chrome-ctrl-hover);
    color: var(--ctrl-fg);
  }

  .notice :global(.notice-action:disabled) {
    opacity: 0.5;
    cursor: default;
  }

  /* Aligned rail: every problem contributes three cells to one grid, so the
     "where" column lines up down the list. Boxing each row instead made a card
     inside a card, and around a single problem it was pure overhead. */
  .notice-body :global(.notice-rows) {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: baseline;
    gap: 7px 10px;
    min-width: 0;
  }

  .notice-body :global(.notice-row-locator) {
    min-width: 0;
    color: #dbe4f0;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10.5px * var(--font-scale, 1));
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .notice-body :global(.notice-row-body) {
    min-width: 0;
  }

  .notice-body :global(.notice-row-text),
  .notice-body :global(.notice-text) {
    margin: 0;
    color: #cbd5e1;
    font-size: calc(11px * var(--font-scale, 1));
    line-height: 1.4;
    overflow-wrap: anywhere;
  }

  .notice-body :global(.notice-row-meta) {
    display: block;
    margin-top: 3px;
    color: #94a3b8;
    font-size: calc(10px * var(--font-scale, 1));
  }

  .notice-body :global(.notice-row-tag) {
    color: #94a3b8;
    font-size: calc(9px * var(--font-scale, 1));
    /* Matches .group-name and .segment-label. */
    letter-spacing: 0.05em;
    text-transform: uppercase;
    white-space: nowrap;
  }

  /* The file is the actionable part of the card, so it gets a row of its own
     with a real control, set off from the problem above it. */
  .notice-body :global(.notice-footer) {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    padding-top: 7px;
    border-top: 1px solid rgba(255, 255, 255, 0.06);
  }

  .notice-body :global(.notice-footer-path) {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    color: #dbe4f0;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10.5px * var(--font-scale, 1));
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* The filename is the anchor; the directory only has to be recognisable. */
  .notice-body :global(.notice-footer-dir) {
    color: #64748b;
  }

  .notice-body :global(.notice-footer-hint) {
    flex-shrink: 0;
    color: #64748b;
    font-size: calc(10px * var(--font-scale, 1));
  }
</style>
