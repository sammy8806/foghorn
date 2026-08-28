import { beforeEach, describe, expect, it } from 'vitest';
import { get } from 'svelte/store';
import {
  configProblems,
  dismissProblems,
  dismissedProblems,
  notificationProblem,
  problemsFingerprint,
  resetDismissedProblemsForTest,
  sourceProblem,
  summarizeProblems,
} from './problems';
import type { SourceHealth } from './alerts';
import type { ConfigDiagnosticsPayload } from './diagnostics';

function health(overrides: Partial<SourceHealth> = {}): SourceHealth {
  return {
    source: 'central1',
    ok: false,
    pending: false,
    lastPoll: '2026-08-27T16:35:27Z',
    lastError: 'Get "https://alertmanager.example/api/v2/alerts": dial tcp: no such host',
    consecFails: 16,
    ...overrides,
  };
}

function diagnostics(overrides: Partial<ConfigDiagnosticsPayload> = {}): ConfigDiagnosticsPayload {
  return {
    path: '~/Library/Application Support/foghorn/config.yaml',
    fingerprint: 'abc',
    items: [],
    excerpt: [],
    ...overrides,
  };
}

describe('sourceProblem', () => {
  it('splits a Go url error into the request and the cause', () => {
    const problem = sourceProblem(health());

    expect(problem.detail).toBe('dial tcp: no such host');
    expect(problem.frame).toEqual([
      { gutter: '', lead: 'GET', text: 'https://alertmanager.example/api/v2/alerts', marked: false },
      { gutter: '', lead: '', text: 'dial tcp: no such host', marked: true },
    ]);
    expect(problem.copy).toBe(health().lastError);
  });

  it('finds the request inside the wrapping a provider adds', () => {
    // What actually reaches the UI: the poll error, wrapped with where it came
    // from. The wrapper only repeats the headline, so it is not carried over.
    const problem = sourceProblem(health({
      lastError: 'fetching alerts from central1: Get "https://alertmanager.example/api/v2/alerts": dial tcp: no such host',
    }));

    expect(problem.detail).toBe('dial tcp: no such host');
    expect(problem.frame).toEqual([
      { gutter: '', lead: 'GET', text: 'https://alertmanager.example/api/v2/alerts', marked: false },
      { gutter: '', lead: '', text: 'dial tcp: no such host', marked: true },
    ]);
  });

  it('still frames an error that will not come apart', () => {
    // The row above shows one ellipsized line of it; without this the panel
    // would open onto nothing but the poll time.
    const problem = sourceProblem(health({ lastError: 'unexpected status 503' }));

    expect(problem.detail).toBe('unexpected status 503');
    expect(problem.frame).toEqual([{ gutter: '', lead: '', text: 'unexpected status 503', marked: true }]);
  });

  it('frames nothing when there is no error to show', () => {
    expect(sourceProblem(health({ lastError: '' })).frame).toEqual([]);
  });

  it('reports a run of failures but not a single one', () => {
    expect(sourceProblem(health({ consecFails: 16 })).meta).toContain('16 consecutive failures');
    expect(sourceProblem(health({ consecFails: 1 })).meta).toHaveLength(1);
  });

  it('says so when a source has never been polled', () => {
    expect(sourceProblem(health({ lastPoll: '' })).meta[0]).toBe('never polled');
  });
});

describe('configProblems', () => {
  it('turns an unparseable config into one row carrying the excerpt', () => {
    const problems = configProblems(diagnostics({
      items: [{ field: 'config', locator: 'line 4', message: 'mapping values are not allowed', dropped: false }],
      excerpt: [
        { number: 3, text: '  interval: 30s', marked: false },
        { number: 4, text: '  severity: warn: high', marked: true },
      ],
    }));

    expect(problems).toHaveLength(1);
    expect(problems[0].keyword).toBe('config');
    expect(problems[0].headline).toBe('Not loaded');
    expect(problems[0].detail).toBe('mapping values are not allowed');
    expect(problems[0].frame).toEqual([
      { gutter: '3', lead: '', text: '  interval: 30s', marked: false },
      { gutter: '4', lead: '', text: '  severity: warn: high', marked: true },
    ]);
  });

  it('names the field and the outcome for a problem that did not stop the load', () => {
    const problems = configProblems(diagnostics({
      items: [
        { field: 'ui.scale', locator: '', message: 'out of range', dropped: false },
        { field: 'resolvers[0]', locator: '', message: 'stdin is required', dropped: true },
      ],
    }));

    // The keyword stays the category; the field goes in the headline, where its
    // case survives the uppercase treatment the keyword column gets.
    expect(problems.map(p => p.keyword)).toEqual(['config', 'config']);
    expect(problems.map(p => p.headline)).toEqual([
      'ui.scale using default',
      'resolvers[0] not loaded',
    ]);
    expect(problems.map(p => p.frame)).toEqual([[], []]);
  });

  it('opens the group with the action and closes it with the file', () => {
    const problems = configProblems(diagnostics({
      items: [
        { field: 'ui.scale', locator: '', message: 'out of range', dropped: false },
        { field: 'resolvers[0]', locator: '', message: 'stdin is required', dropped: true },
      ],
    }));

    expect(problems[0].action?.kind).toBe('reveal');
    expect(problems[1].action).toBeNull();
    expect(problems[0].meta).toEqual([]);
    expect(problems[1].meta).toEqual([diagnostics().path, 'rechecks on save']);
  });

  it('puts both on the same row when there is only one', () => {
    const problems = configProblems(diagnostics({
      items: [{ field: 'config', locator: 'line 4', message: 'boom', dropped: false }],
    }));

    expect(problems[0].action?.kind).toBe('reveal');
    expect(problems[0].meta).toEqual([diagnostics().path, 'rechecks on save']);
  });

  it('omits the file footnote before a path is known', () => {
    const problems = configProblems(diagnostics({
      path: '',
      items: [{ field: 'ui.scale', locator: '', message: 'out of range', dropped: false }],
    }));

    expect(problems[0].meta).toEqual([]);
  });
});

describe('notificationProblem', () => {
  it('describes each status the permission can be in', () => {
    expect(notificationProblem('denied')?.headline).toBe('Blocked');
    expect(notificationProblem('not_determined')?.headline).toBe('Not allowed yet');
    expect(notificationProblem('unsupported_legacy')?.headline).toBe('Cannot be checked');
  });

  it('is nothing to report once permission is granted', () => {
    expect(notificationProblem('granted')).toBeNull();
    expect(notificationProblem('')).toBeNull();
  });
});

describe('summarizeProblems', () => {
  it('lets a lone problem speak in its own words', () => {
    const summary = summarizeProblems(configProblems(diagnostics({
      items: [{ field: 'config', locator: 'line 4', message: 'mapping values are not allowed', dropped: false }],
    })));

    expect(summary).toEqual({
      keyword: 'config',
      severity: 'caution',
      headline: 'Not loaded',
      tail: 'mapping values are not allowed',
      // The strip is this problem's only row, so it carries the fix too.
      action: { kind: 'reveal', label: 'Open folder', glyph: '↗' },
    });
  });

  it('hands the fixes back to their own rows once there are several', () => {
    const summary = summarizeProblems([
      sourceProblem(health()),
      ...configProblems(diagnostics({
        items: [{ field: 'config', locator: 'line 4', message: 'boom', dropped: false }],
      })),
    ]);

    expect(summary.action).toBeNull();
  });

  it('counts several and says what they cost you', () => {
    const summary = summarizeProblems([
      sourceProblem(health()),
      ...configProblems(diagnostics({
        items: [{ field: 'config', locator: 'line 4', message: 'boom', dropped: false }],
      })),
    ]);

    expect(summary.keyword).toBe('2 problems');
    expect(summary.headline).toBe('central1 unreachable, config not loaded');
    expect(summary.tail).toBe('alerts may be stale');
  });

  it('takes the worst severity in the set', () => {
    const caution = configProblems(diagnostics({
      items: [{ field: 'ui.scale', locator: '', message: 'out of range', dropped: false }],
    }));

    expect(summarizeProblems(caution).severity).toBe('caution');
    expect(summarizeProblems([...caution, sourceProblem(health())]).severity).toBe('critical');
  });

  it('names the cost of config problems when no source is failing', () => {
    const summary = summarizeProblems(configProblems(diagnostics({
      items: [
        { field: 'ui.scale', locator: '', message: 'out of range', dropped: false },
        { field: 'resolvers[0]', locator: '', message: 'stdin is required', dropped: true },
      ],
    })));

    expect(summary.headline).toBe('ui.scale using default, resolvers[0] not loaded');
    expect(summary.tail).toBe('some settings are not in effect');
  });
});

describe('problemsFingerprint', () => {
  it('does not depend on the order sources happen to report in', () => {
    const a = sourceProblem(health({ source: 'central1' }));
    const b = sourceProblem(health({ source: 'central2' }));

    expect(problemsFingerprint([a, b])).toBe(problemsFingerprint([b, a]));
  });

  it('changes when the same source starts failing differently', () => {
    const before = sourceProblem(health({ lastError: 'unexpected status 503' }));
    const after = sourceProblem(health({ lastError: 'unexpected status 500' }));

    expect(problemsFingerprint([before])).not.toBe(problemsFingerprint([after]));
  });

  it('ignores a run of failures, so retrying does not resurrect a dismissal', () => {
    const early = sourceProblem(health({ consecFails: 2 }));
    const later = sourceProblem(health({ consecFails: 40 }));

    expect(problemsFingerprint([early])).toBe(problemsFingerprint([later]));
  });

  it('does not resurrect a dismissal when a valid entry shifts a list position', () => {
    const before = configProblems(diagnostics({
      items: [{ field: 'resolvers[0]', locator: '', message: 'stdin is required', dropped: true }],
    }));
    const after = configProblems(diagnostics({
      items: [{ field: 'resolvers[1]', locator: '', message: 'stdin is required', dropped: true }],
    }));

    expect(problemsFingerprint(before)).toBe(problemsFingerprint(after));
  });
});

describe('dismissal', () => {
  beforeEach(resetDismissedProblemsForTest);

  const visible = (problems: Parameters<typeof problemsFingerprint>[0]) =>
    problems.length > 0 && problemsFingerprint(problems) !== get(dismissedProblems);

  it('hides exactly the set that was on screen', () => {
    const shown = [sourceProblem(health())];
    expect(visible(shown)).toBe(true);

    dismissProblems(problemsFingerprint(shown));
    expect(visible(shown)).toBe(false);
  });

  it('comes back when a problem joins the set', () => {
    const shown = [sourceProblem(health())];
    dismissProblems(problemsFingerprint(shown));

    const more = [...shown, ...configProblems(diagnostics({
      items: [{ field: 'config', locator: 'line 4', message: 'boom', dropped: false }],
    }))];
    expect(visible(more)).toBe(true);
  });

  it('stays dismissed while the same problem persists unchanged', () => {
    const shown = [sourceProblem(health({ consecFails: 3 }))];
    dismissProblems(problemsFingerprint(shown));

    expect(visible([sourceProblem(health({ consecFails: 4 }))])).toBe(false);
  });
});

describe('the keyword column', () => {
  it('never case-mangles a name the user chose', () => {
    const problems = configProblems(diagnostics({
      items: [{ field: 'resolvers[0] "Cluster-Name"', locator: '', message: 'stdin is required', dropped: true }],
    }));

    expect(problems[0].keyword).toBe('config');
    expect(problems[0].headline).toContain('"Cluster-Name"');
  });
});

describe('the reveal action', () => {
  const oneProblem = (platform?: string) => configProblems(diagnostics({
    items: [{ field: 'config', locator: 'line 4', message: 'boom', dropped: false }],
  }), platform)[0];

  it('names where it takes you, so it is not the Show beside it', () => {
    // Neither wording is "open the file": macOS selects it in Finder and every
    // other platform opens the folder around it, which is what these say.
    expect(oneProblem('darwin').action?.label).toBe('Reveal in Finder');
    expect(oneProblem('linux').action?.label).toBe('Open folder');
    expect(oneProblem('windows').action?.label).toBe('Open folder');
  });

  it('falls back to the folder wording before the platform is known', () => {
    expect(oneProblem('').action?.label).toBe('Open folder');
    expect(oneProblem().action?.label).toBe('Open folder');
  });
});
