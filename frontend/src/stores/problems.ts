import { writable } from 'svelte/store';
import type { SourceHealth } from './alerts';
import type { ConfigDiagnosticsPayload } from './diagnostics';

// Everything that can be wrong with the workspace, reduced to one shape. A
// config that will not parse, a source that will not answer and a notification
// permission that was never granted are three unrelated failures, but the user
// meets them in one place — row zero of the alert list — so they have to be one
// list before they can be one row.
//
// This module is pure: it maps the three inputs onto Problems and says nothing
// about how they are drawn. What each action DOES stays with the component that
// owns the binding, which is why an action carries a kind rather than a
// callback.

export type ProblemSeverity = 'critical' | 'caution';

export type ProblemActionKind = 'retry' | 'reveal' | 'notification-settings';

export interface ProblemAction {
  kind: ProblemActionKind;
  label: string;
  /** Trailing mark: ⟳ for something re-run in place, ↗ for somewhere else. */
  glyph: string;
}

/** One line of monospace evidence under a problem. */
export interface ProblemFrameLine {
  /** Line number for a file excerpt; empty for free-form evidence. */
  gutter: string;
  /** Dimmed label naming the line ("GET"); empty when the text stands alone. */
  lead: string;
  text: string;
  /** The line the problem is actually about. Always the accent colour, and in
   *  a file excerpt also the highlighted row. */
  marked: boolean;
}

export interface Problem {
  /** Identity, and what the dismissal fingerprint is built from. */
  key: string;
  /** Mono gutter label naming what is broken. The only coloured mark. */
  keyword: string;
  severity: ProblemSeverity;
  /** Bold lead of the row: the outcome. */
  headline: string;
  /** Dimmed tail after the em dash: the cause. */
  detail: string;
  /** How the collapsed strip refers to this problem when it lists several. */
  short: string;
  /** Monospace evidence. Empty when the message is the whole story. */
  frame: ProblemFrameLine[];
  /** Mono footnotes under the frame. */
  meta: string[];
  /** Raw text behind a "copy" link, for the errors this splits up. */
  copy: string;
  action: ProblemAction | null;
}

export interface ProblemSummary {
  keyword: string;
  severity: ProblemSeverity;
  /** The bright part of the one-line summary. */
  headline: string;
  /** The dimmed consequence after it. */
  tail: string;
  /** The lone problem's action, when the strip is speaking for one problem and
   *  can therefore carry its fix. Null once there are several fixes to offer. */
  action: ProblemAction | null;
}

// Go's *url.Error stringifies as `Get "https://host/path": dial tcp: no such
// host`, which is three facts in one line. Split so the request and the failure
// can sit on their own lines instead of wrapping as one paragraph.
const urlErrorPattern = /^([A-Za-z]+) "([^"]+)": ([\s\S]+)$/;

// The backend already normalises list positions out of its own fingerprint, so
// inserting a valid entry ahead of a broken one cannot resurrect a warning the
// user dismissed. The fingerprint here carries the same normalisation, for the
// same reason — a position is where a problem is, not which problem it is.
const listPositionPattern = /\[\d+\]/g;

export function sourceProblem(health: SourceHealth): Problem {
  const raw = health.lastError ?? '';
  const parsed = urlErrorPattern.exec(raw);
  const cause = parsed ? parsed[3] : raw;

  return {
    key: `source:${health.source}`,
    keyword: 'source',
    severity: 'critical',
    headline: `${health.source} unreachable`,
    detail: cause,
    short: `${health.source} unreachable`,
    // Without a URL to pull out there is nothing to line up, and the cause is
    // already the detail — a frame repeating it would be the same text twice.
    frame: parsed
      ? [
          { gutter: '', lead: parsed[1].toUpperCase(), text: parsed[2], marked: false },
          { gutter: '', lead: '', text: parsed[3], marked: true },
        ]
      : [],
    meta: sourceMeta(health),
    copy: raw,
    action: { kind: 'retry', label: 'Retry', glyph: '⟳' },
  };
}

function sourceMeta(health: SourceHealth): string[] {
  const meta = [
    health.lastPoll ? `last poll ${formatPollTime(health.lastPoll)}` : 'never polled',
  ];
  // One failure is what the row already says; a run of them is the new fact.
  if (health.consecFails > 1) meta.push(`${health.consecFails} consecutive failures`);
  return meta;
}

function formatPollTime(timestamp: string): string {
  const at = new Date(timestamp);
  if (Number.isNaN(at.getTime()) || at.getTime() === 0) return 'never';
  return at.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function configProblems(payload: ConfigDiagnosticsPayload): Problem[] {
  const problems = payload.items.map((item, index) => {
    // A config that would not parse at all arrives as a single item filed under
    // "config": there is no field to name, so the headline carries the outcome
    // and the excerpt does the pointing a field name would otherwise do.
    //
    // The keyword stays the category either way. A field path is an identifier
    // the user has to be able to find in their own file — some of it is their
    // own naming — and the keyword column is set uppercase, so putting it there
    // would hand back text that no longer matches what they wrote.
    const unusable = item.field === 'config';
    const outcome = item.dropped ? 'not loaded' : 'using default';
    const headline = unusable ? 'Not loaded' : `${item.field} ${outcome}`;

    return {
      key: `config:${item.field}:${index}`,
      keyword: 'config',
      severity: 'caution' as ProblemSeverity,
      headline,
      detail: item.message,
      short: unusable ? 'config not loaded' : headline,
      frame: unusable ? excerptFrame(payload) : [],
      meta: [] as string[],
      copy: '',
      action: null as ProblemAction | null,
    };
  });

  // The config rows are a group with one fix between them, so the action opens
  // the group and the file footnote closes it, rather than every row repeating
  // both. With a single row — the common case — they land on the same one.
  if (problems.length > 0) {
    problems[0].action = { kind: 'reveal', label: 'Open', glyph: '↗' };
    problems[problems.length - 1].meta = configMeta(payload);
  }
  return problems;
}

function excerptFrame(payload: ConfigDiagnosticsPayload): ProblemFrameLine[] {
  return payload.excerpt.map(line => ({
    gutter: String(line.number),
    lead: '',
    text: line.text,
    marked: line.marked,
  }));
}

function configMeta(payload: ConfigDiagnosticsPayload): string[] {
  if (!payload.path) return [];
  // Saying the file is watched is what stops people from hunting for a reload
  // command after they fix it.
  return [payload.path, 'rechecks on save'];
}

/** Maps the macOS notification permission status onto a problem, or null when
 *  the permission is not something to complain about. */
export function notificationProblem(status: string): Problem | null {
  const headline =
    status === 'denied' ? 'Blocked'
    : status === 'not_determined' ? 'Not allowed yet'
    : status === 'unsupported_legacy' ? 'Cannot be checked'
    : '';
  if (!headline) return null;

  const detail =
    status === 'denied'
      ? 'macOS is not letting Foghorn post to Notification Center.'
      : status === 'unsupported_legacy'
        ? 'This macOS version does not report permission status. Check that Foghorn is allowed.'
        : 'macOS has not granted notification permission to Foghorn yet.';

  return {
    key: `notifications:${status}`,
    keyword: 'notifications',
    severity: 'caution',
    headline,
    detail,
    short: `notifications ${headline.toLowerCase()}`,
    frame: [],
    meta: [],
    copy: '',
    action: { kind: 'notification-settings', label: 'Open Settings', glyph: '↗' },
  };
}

export function summarizeProblems(problems: Problem[]): ProblemSummary {
  const severity: ProblemSeverity = problems.some(p => p.severity === 'critical') ? 'critical' : 'caution';

  // One problem gets to speak for itself: its own keyword, its own words, its
  // own fix. The strip IS that problem's row, so nothing below it may repeat
  // the row — and the action belongs up here rather than a disclosure away.
  //
  // A count only earns the gutter once there is more than one thing to count,
  // and once there is, the fixes differ per problem and go back to their rows.
  if (problems.length === 1) {
    return {
      keyword: problems[0].keyword,
      severity,
      headline: problems[0].headline,
      tail: problems[0].detail,
      action: problems[0].action,
    };
  }

  return {
    keyword: `${problems.length} problems`,
    severity,
    headline: problems.map(p => p.short).join(', '),
    tail: consequenceOf(problems),
    action: null,
  };
}

// What the list means for the window you are looking at, which is the one thing
// the individual messages never say.
function consequenceOf(problems: Problem[]): string {
  if (problems.some(p => p.key.startsWith('source:'))) return 'alerts may be stale';
  if (problems.some(p => p.key.startsWith('config:'))) return 'some settings are not in effect';
  return '';
}

/** Identity of a whole set of problems, so dismissing one set cannot hide the
 *  next. Order-independent: sources report in whatever order they finish. */
export function problemsFingerprint(problems: Problem[]): string {
  return problems
    .map(problem =>
      [problem.key, problem.headline, problem.detail].join('\u0000').replace(listPositionPattern, '[]'),
    )
    .sort()
    .join('\u0001');
}

// The × clears the strip for exactly the set on screen. A problem that outlives
// the dismissal stays dismissed — the status bar's health dot is what keeps a
// failing source visible — but any change to the set brings the strip back.
export const dismissedProblems = writable<string>('');

export function dismissProblems(fingerprint: string) {
  dismissedProblems.set(fingerprint);
}

export function resetDismissedProblemsForTest() {
  dismissedProblems.set('');
}
