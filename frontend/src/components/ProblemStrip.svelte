<script context="module" lang="ts">
  // The disclosure body needs a stable id for aria-controls.
  let stripSeq = 0;
</script>

<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { summarizeProblems, type Problem, type ProblemActionKind } from '../stores/problems';

  // Row zero of the alert list: everything wrong with the workspace, on one
  // line, above the alerts it affects. Not a card floating on the window — a
  // full-bleed band with its own hairlines, so it reads as "the first thing
  // wrong" rather than as more window chrome.
  //
  // There is exactly one coloured mark in the collapsed state, and it is the
  // mono keyword: no dot, and the prose stays in the window's normal text
  // hierarchy. Colouring the message is what made the old cards read as web
  // banners instead of window furniture.
  //
  // Opened, the detail is an INSET panel — darker ground, its own hairlines —
  // so a problem row can never be mistaken for an alert row below it. The
  // darkening is an alpha, not an opaque fill: on macOS it has to compose with
  // the vibrancy behind the window rather than stack a second layer on it.
  export let problems: Problem[] = [];
  /** Disables the retry action while a poll is already in flight. */
  export let retrying = false;
  /** Whatever the last invoked action failed with. */
  export let actionError = '';

  const dispatch = createEventDispatcher<{
    action: { kind: ProblemActionKind };
    dismiss: void;
  }>();

  const panelId = `problem-panel-${++stripSeq}`;

  let expanded = false;
  let copied = '';
  let copyTimer: ReturnType<typeof setTimeout> | undefined;

  $: summary = summarizeProblems(problems);
  // With one problem the strip is already that problem's row, so the panel must
  // not draw the row again — it holds the evidence and nothing else. Two rows
  // reading the same sentence is what the old stacked cards did wrong.
  $: single = problems.length === 1;
  // Which leaves nothing to open when that one problem has no evidence: it is
  // fully stated by the strip, and its fix is on the strip too.
  $: hasDetail = !single || problems[0].frame.length > 0 || problems[0].meta.length > 0 || Boolean(actionError);
  $: if (!hasDetail) expanded = false;

  function toggle() {
    if (hasDetail) expanded = !expanded;
  }

  async function copyError(problem: Problem) {
    try {
      await navigator.clipboard.writeText(problem.copy);
      copied = problem.key;
      clearTimeout(copyTimer);
      copyTimer = setTimeout(() => (copied = ''), 1500);
    } catch (e) {
      console.error('failed to copy error text', e);
    }
  }
</script>

<section class="strip-root {summary.severity}" role="alert">
  <div class="strip">
    <!-- The × is its own control, so the row that holds it cannot be a button
       around a button. The band lights on hover as one surface regardless. -->
    <button
      type="button"
      class="strip-disclosure"
      class:inert={!hasDetail}
      aria-expanded={hasDetail ? expanded : undefined}
      aria-controls={hasDetail ? panelId : undefined}
      on:click={toggle}
    >
      <span class="strip-keyword">{summary.keyword}</span>
      <span class="strip-headline" title={summary.tail ? `${summary.headline} — ${summary.tail}` : summary.headline}>
        {summary.headline}{#if summary.tail}<span class="strip-tail"> — {summary.tail}</span>{/if}
      </span>
    </button>
    <!-- Trailing cluster, in a fixed order: fix, disclose, dismiss. Show/Hide
       keeps the same place whether or not there is an action beside it, so it
       is never where the action was a moment ago. -->
    {#if summary.action}
      <button
        type="button"
        class="problem-action {summary.severity}"
        disabled={summary.action.kind === 'retry' && retrying}
        on:click={() => summary.action && dispatch('action', { kind: summary.action.kind })}
      >
        {#if summary.action.kind === 'retry' && retrying}
          Retrying…
        {:else}
          {summary.action.label}{#if summary.action.glyph}&nbsp;{summary.action.glyph}{/if}
        {/if}
      </button>
    {/if}
    <!-- The whole headline region is already the disclosure, announced and
       focusable. This is the same control drawn where the eye looks for it, so
       it is a mouse affordance only and stays out of the tab order and the
       accessibility tree rather than being announced twice. -->
    {#if hasDetail}
      <button type="button" class="strip-toggle" class:expanded tabindex="-1" aria-hidden="true" on:click={toggle}>
        {expanded ? 'Hide' : 'Show'}
        <!-- One mark, flipped, rather than the ⌄/⌃ pair: those are separate
           glyphs and system-ui draws them at noticeably different weights and
           sizes, so the control changed shape as well as state. -->
        <svg class="strip-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </button>
    {/if}
    <button type="button" class="strip-dismiss" title="Dismiss" aria-label="Dismiss" on:click={() => dispatch('dismiss')}>×</button>
  </div>

  {#if expanded}
    <!-- One grid for every problem, so the message column lines up down the
       list and the evidence under a row indents to it on its own — the keyword
       column sizes to the longest keyword instead of to a guessed width. -->
    <div class="panel" id={panelId}>
      {#each problems as problem, index (problem.key)}
        {#if index > 0}
          <div class="problem-rule" aria-hidden="true"></div>
        {/if}

        {#if !single}
          <span class="problem-keyword {problem.severity}">{problem.keyword}</span>
          <span class="problem-line" title="{problem.headline}{problem.detail ? ` — ${problem.detail}` : ''}">
            <strong>{problem.headline}</strong>{#if problem.detail}<span class="problem-detail"> — {problem.detail}</span>{/if}
          </span>
          {#if problem.action}
            <button
              type="button"
              class="problem-action {problem.severity}"
              disabled={problem.action.kind === 'retry' && retrying}
              on:click={() => problem.action && dispatch('action', { kind: problem.action.kind })}
            >
              <!-- Retry is the only action that keeps running after the click,
                 so it is the only one with something to say while it does. -->
              {#if problem.action.kind === 'retry' && retrying}
                Retrying…
              {:else}
                {problem.action.label}{#if problem.action.glyph}&nbsp;{problem.action.glyph}{/if}
              {/if}
            </button>
          {:else}
            <span></span>
          {/if}
        {/if}

        {#if problem.frame.length > 0 || problem.meta.length > 0 || problem.copy}
          <div class="problem-evidence" class:full={single}>
            {#if problem.frame.length > 0}
              <div class="frame" class:excerpt={problem.frame[0].gutter !== ''}>
                {#each problem.frame as line}
                  <div class="frame-line {problem.severity}" class:marked={line.marked}>
                    {#if line.gutter}<span class="frame-gutter">{line.gutter}</span>{/if}
                    {#if line.lead}<span class="frame-lead">{line.lead}</span>{/if}
                    <span class="frame-text">{line.text}</span>
                  </div>
                {/each}
              </div>
            {/if}
            {#if problem.meta.length > 0 || problem.copy}
              <div class="problem-meta">
                {#each problem.meta as note}
                  <span class="problem-note">{note}</span>
                {/each}
                {#if problem.copy}
                  <button type="button" class="problem-copy" on:click={() => copyError(problem)}>
                    {copied === problem.key ? 'copied' : 'copy'}
                  </button>
                {/if}
              </div>
            {/if}
          </div>
        {/if}
      {/each}

      {#if actionError}
        <p class="panel-error">{actionError}</p>
      {/if}
    </div>
  {/if}
</section>

<style>
  /* Severity is these three properties and nothing else, so a critical problem
     and a caution one cannot drift apart in geometry or type. The class goes on
     every element that shows severity rather than only on the root: the strip
     takes the worst of the set, but inside the panel a stale config keeps its
     own amber next to a source's red. */
  .critical {
    --problem-rgb: 239, 68, 68;
    --problem-keyword: #f87171;
    --problem-mark: #ef4444;
  }

  .caution {
    --problem-rgb: 245, 158, 11;
    --problem-keyword: #fbbf24;
    --problem-mark: #f59e0b;
  }

  .strip-root {
    flex-shrink: 0;
  }

  .strip {
    display: flex;
    align-items: center;
    gap: 12px;
    /* Left edge is the shared text column, so the keyword hangs off the same
       line as the alert count above it and the group labels below. */
    padding: 9px var(--chrome-gutter) 9px var(--chrome-text-x);
    /* A wash anchored to the leading edge rather than a filled bar: it fades
       out before it reaches the controls, so the row still reads as part of
       the window instead of a coloured slab laid across it. */
    background: linear-gradient(
      90deg,
      rgba(var(--problem-rgb), 0.17),
      rgba(var(--problem-rgb), 0.04) 55%,
      transparent
    );
    border-top: 1px solid rgba(var(--problem-rgb), 0.15);
    border-bottom: 1px solid rgba(var(--problem-rgb), 0.15);
    /* The strip takes over the chrome band's hairline instead of adding a
       second rule under it. */
    margin-top: -1px;
    user-select: none;
  }

  .strip:hover {
    background: linear-gradient(
      90deg,
      rgba(var(--problem-rgb), 0.24),
      rgba(var(--problem-rgb), 0.07) 55%,
      rgba(255, 255, 255, 0.02)
    );
  }

  /* Reset covers the button form. padding:0 matters on WebKit in particular:
     its UA sheet gives <button> asymmetric block padding, which would sit the
     summary off the row the strip establishes. */
  .strip-disclosure,
  .strip-dismiss,
  .strip-toggle,
  .problem-action,
  .problem-copy {
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .strip-disclosure {
    display: flex;
    align-items: center;
    gap: 12px;
    flex: 1;
    min-width: 0;
  }

  /* Nothing to open, nothing to click. */
  .strip-disclosure.inert {
    cursor: default;
  }

  .strip-disclosure:focus-visible,
  .strip-dismiss:focus-visible,
  .problem-action:focus-visible,
  .problem-copy:focus-visible {
    outline: 2px solid var(--problem-keyword);
    outline-offset: 2px;
    border-radius: 3px;
  }

  /* The keyword is the single coloured mark in the collapsed row: it names what
     is broken and carries the severity that a dot used to. */
  .strip-keyword {
    flex: none;
    color: var(--problem-keyword);
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10.5px * var(--font-scale, 1));
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .strip-headline {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    color: #e2e8f0;
    font-size: calc(12px * var(--font-scale, 1));
    line-height: 1.4;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* What the problems cost you, which is the one thing the messages never say
     themselves — secondary to the messages, so it is set back. */
  .strip-tail {
    color: #94a3b8;
  }

  .strip-toggle {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    flex: none;
    /* Both words are four characters, so the control keeps its width across the
       state change and the row does not shuffle under the pointer. */
    justify-content: flex-end;
    min-width: 4.4em;
    white-space: nowrap;
    color: #e2e8f0;
    font-size: calc(11.5px * var(--font-scale, 1));
    font-weight: 600;
  }

  .strip-chevron {
    flex: none;
    transition: transform 120ms ease;
  }

  .strip-toggle.expanded .strip-chevron {
    transform: rotate(180deg);
  }

  .strip-dismiss {
    flex: none;
    /* Same dim as .status-bar: the × is affordance, not severity. */
    color: #64748b;
    font-size: calc(15px * var(--font-scale, 1));
    line-height: 1;
    transition: color 0.15s;
  }

  .strip-dismiss:hover {
    color: #e2e8f0;
  }

  .panel {
    display: grid;
    /* Keyword, message, action. The keyword column sizes to the longest keyword
       — a field path is a great deal longer than "config" — with a floor that
       keeps the short ones from collapsing onto the message. */
    grid-template-columns: minmax(58px, max-content) minmax(0, 1fr) auto;
    align-items: center;
    column-gap: 12px;
    padding: 0 var(--chrome-gutter) 0 var(--chrome-text-x);
    /* Inset, not raised: a darker ground with its own shadow under each edge,
       so the detail belongs to the strip and not to the alert rows under it. */
    background: rgba(0, 0, 0, 0.16);
    box-shadow:
      inset 0 8px 12px -10px rgba(0, 0, 0, 0.75),
      inset 0 -8px 12px -10px rgba(0, 0, 0, 0.75);
    border-bottom: 1px solid var(--chrome-hairline);
  }

  .problem-rule {
    grid-column: 1 / -1;
    height: 1px;
    /* Full bleed, so the rule separates two problems rather than boxing the
       padded column they happen to sit in. */
    margin: 0 calc(-1 * var(--chrome-gutter)) 0 calc(-1 * var(--chrome-text-x));
    background: rgba(255, 255, 255, 0.05);
  }

  /* Opens a row in the panel grid. Scoped to the panel: .problem-action is also
     the strip's action, where this padding would drop it off the strip's row. */
  .panel > .problem-keyword,
  .panel > .problem-line,
  .panel > .problem-action {
    padding-top: 11px;
  }

  .problem-keyword {
    align-self: baseline;
    color: var(--problem-keyword);
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10.5px * var(--font-scale, 1));
    font-weight: 600;
    letter-spacing: 0.08em;
    line-height: 1.4;
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }

  .problem-line {
    min-width: 0;
    overflow: hidden;
    color: #e2e8f0;
    font-size: calc(12px * var(--font-scale, 1));
    line-height: 1.4;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .problem-detail {
    color: #94a3b8;
  }

  .problem-action {
    /* Grid items stretch by default, so a short action kept a box the width of
       the longest one and sat at its left edge — "Retry" floating in from the
       trailing edge that "Reveal in Finder" defined. Every action now ends on
       the same line, which is the one the strip's own controls end on. */
    justify-self: end;
    white-space: nowrap;
    color: #e2e8f0;
    font-size: calc(11.5px * var(--font-scale, 1));
    font-weight: 600;
    transition: color 0.15s;
  }

  .problem-action:hover:not(:disabled) {
    color: var(--problem-keyword);
  }

  .problem-action:disabled {
    opacity: 0.5;
    cursor: default;
  }

  /* Evidence indents to the message column on its own, so it stays aligned
     whatever the keyword column ends up measuring. */
  .problem-evidence {
    grid-column: 2 / -1;
    min-width: 0;
    padding: 7px 0 12px;
  }

  /* Nothing to indent past: with no keyword beside it, the evidence hangs off
     the window's text column like every other left edge in the list. */
  .problem-evidence.full {
    grid-column: 1 / -1;
    padding-top: 11px;
  }

  .frame {
    /* Lighter than the panel it sits in, matching the toolbar's own capsules:
       an overlay rather than an opaque fill, so it composes on the vibrancy. */
    background: var(--chrome-capsule-bg);
    border: 1px solid var(--chrome-hairline);
    border-radius: 7px;
    padding: 7px 0;
    overflow: hidden;
    color: #94a3b8;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(11px * var(--font-scale, 1));
    line-height: 1.65;
  }

  .frame-line {
    display: flex;
    gap: 12px;
    padding: 0 12px;
  }

  /* Only a file excerpt highlights its marked row: there the mark says WHICH
     line, and the surrounding lines are context. Free-form evidence has no
     context to pick the line out of, so the colour alone carries it. */
  .frame.excerpt .frame-line.marked {
    margin-left: -2px;
    padding-left: 12px;
    border-left: 2px solid var(--problem-mark);
    background: rgba(var(--problem-rgb), 0.09);
  }

  .frame.excerpt .frame-line {
    color: #64748b;
  }

  .frame-line.marked .frame-text {
    color: var(--problem-keyword);
  }

  .frame-gutter {
    flex: none;
    width: 2ch;
    text-align: right;
    color: #64748b;
  }

  .frame-line.marked .frame-gutter {
    color: var(--problem-mark);
  }

  .frame-lead {
    flex: none;
    color: #64748b;
  }

  .frame-text {
    min-width: 0;
    /* A config line keeps its indentation; an error or a URL wraps rather than
       forcing the panel wide. */
    white-space: pre-wrap;
    word-break: break-all;
  }

  .problem-meta {
    display: flex;
    align-items: baseline;
    gap: 14px;
    min-width: 0;
    margin-top: 7px;
    color: #64748b;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: calc(10.5px * var(--font-scale, 1));
  }

  .problem-note {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* The path is the only note long enough to need the room, and the only one
     that can be given up on — the rest are short facts. */
  .problem-note:not(:first-child) {
    flex: none;
  }

  .problem-copy {
    flex: none;
    color: #94a3b8;
    font-family: inherit;
    font-size: inherit;
    transition: color 0.15s;
  }

  .problem-copy:hover {
    color: #e2e8f0;
  }

  .panel-error {
    grid-column: 1 / -1;
    margin: 0 0 11px;
    color: #f87171;
    font-size: calc(11px * var(--font-scale, 1));
    line-height: 1.4;
    overflow-wrap: anywhere;
  }
</style>
