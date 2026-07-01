import { describe, it, expect } from 'vitest';
import { parseQuery, parseFieldTerm, matchesQuery, anchoredRegex, queryToMatchers } from './query';
import type { Alert } from './alerts';

describe('parseQuery', () => {
  it('parses bare words as text terms', () => {
    const q = parseQuery('critical disk');
    expect(q.terms).toEqual([
      { kind: 'text', value: 'critical', negated: false },
      { kind: 'text', value: 'disk', negated: false },
    ]);
  });

  it('parses a leading dash as a negated text term', () => {
    const q = parseQuery('-noise');
    expect(q.terms).toEqual([{ kind: 'text', value: 'noise', negated: true }]);
  });

  it('parses quoted phrases as one text term', () => {
    const q = parseQuery('"under maintenance"');
    expect(q.terms).toEqual([{ kind: 'text', value: 'under maintenance', negated: false }]);
  });

  it('parses a negated quoted phrase', () => {
    const q = parseQuery('-"under maintenance"');
    expect(q.terms).toEqual([{ kind: 'text', value: 'under maintenance', negated: true }]);
  });

  it('parses field terms with each operator', () => {
    const q = parseQuery('a=1 b!=2 c=~3 d!~4');
    expect(q.terms).toEqual([
      { kind: 'field', scope: 'label', key: 'a', op: '=', value: '1' },
      { kind: 'field', scope: 'label', key: 'b', op: '!=', value: '2' },
      { kind: 'field', scope: 'label', key: 'c', op: '=~', value: '3' },
      { kind: 'field', scope: 'label', key: 'd', op: '!~', value: '4' },
    ]);
  });

  it('parses the annotation: scope prefix', () => {
    const q = parseQuery('annotation:summary=~disk');
    expect(q.terms).toEqual([
      { kind: 'field', scope: 'annotation', key: 'summary', op: '=~', value: 'disk' },
    ]);
  });

  it('strips quotes from a field-term value', () => {
    const q = parseQuery('summary=~"disk full"');
    expect(q.terms).toEqual([
      { kind: 'field', scope: 'label', key: 'summary', op: '=~', value: 'disk full' },
    ]);
  });

  it('falls back to a text term when a token has no valid operator', () => {
    const q = parseQuery('foo:bar');
    expect(q.terms).toEqual([{ kind: 'text', value: 'foo:bar', negated: false }]);
  });

  it('ignores a field term with an empty value', () => {
    const q = parseQuery('a=');
    expect(q.terms).toEqual([{ kind: 'text', value: 'a=', negated: false }]);
  });

  it('preserves the raw string', () => {
    expect(parseQuery('  x=1  ').raw).toBe('  x=1  ');
  });
});

describe('parseFieldTerm', () => {
  it('returns null for a non-field token', () => {
    expect(parseFieldTerm('critical')).toBeNull();
  });
  it('returns a matcher-ready field term', () => {
    expect(parseFieldTerm('team!=infra')).toEqual({
      kind: 'field', scope: 'label', key: 'team', op: '!=', value: 'infra',
    });
  });
});

function mkAlert(over: Partial<Alert> = {}): Alert {
  return {
    id: 'a', source: 'prod-am', sourceType: 'alertmanager', name: 'DiskFull',
    severity: 'critical', state: 'firing',
    labels: { severity: 'critical', namespace: 'prod', team: 'db' },
    annotations: { summary: 'disk is full on node-1' },
    startsAt: '', updatedAt: '', generatorURL: '',
    silencedBy: [], inhibitedBy: [], receivers: [],
    ...over,
  };
}

describe('anchoredRegex', () => {
  it('anchors the full string', () => {
    const re = anchoredRegex('api-.*')!;
    expect(re.test('api-1')).toBe(true);
    expect(re.test('x-api-1')).toBe(false);
  });
  it('returns null for an invalid pattern', () => {
    expect(anchoredRegex('(')).toBeNull();
  });
});

describe('matchesQuery', () => {
  const a = mkAlert();

  it('matches a bare word as a substring across labels/annotations/name', () => {
    expect(matchesQuery(a, parseQuery('diskfull'))).toBe(true);   // name, case-insensitive
    expect(matchesQuery(a, parseQuery('node-1'))).toBe(true);     // annotation value
    expect(matchesQuery(a, parseQuery('absent'))).toBe(false);
  });

  it('negates a text term', () => {
    expect(matchesQuery(a, parseQuery('-absent'))).toBe(true);
    expect(matchesQuery(a, parseQuery('-disk'))).toBe(false);
  });

  it('applies label = and !=', () => {
    expect(matchesQuery(a, parseQuery('namespace=prod'))).toBe(true);
    expect(matchesQuery(a, parseQuery('namespace=dev'))).toBe(false);
    expect(matchesQuery(a, parseQuery('team!=infra'))).toBe(true);
    expect(matchesQuery(a, parseQuery('team!=db'))).toBe(false);
  });

  it('treats a missing label as empty string', () => {
    expect(matchesQuery(a, parseQuery('missing=""'))).toBe(false);   // "" != absent? Alertmanager: missing == ""
    expect(matchesQuery(a, parseQuery('missing!=x'))).toBe(true);    // "" != x
  });

  it('applies anchored regex for =~ and !~', () => {
    expect(matchesQuery(a, parseQuery('namespace=~pro.*'))).toBe(true);
    expect(matchesQuery(a, parseQuery('namespace=~rod'))).toBe(false); // not anchored full-string
    expect(matchesQuery(a, parseQuery('namespace!~dev.*'))).toBe(true);
  });

  it('matches annotation-scoped field terms', () => {
    expect(matchesQuery(a, parseQuery('annotation:summary=~.*full.*'))).toBe(true);
    expect(matchesQuery(a, parseQuery('annotation:summary=nope'))).toBe(false);
  });

  it('ANDs all terms', () => {
    expect(matchesQuery(a, parseQuery('critical namespace=prod team!=infra'))).toBe(true);
    expect(matchesQuery(a, parseQuery('critical namespace=dev'))).toBe(false);
  });

  it('an invalid =~ regex matches nothing', () => {
    expect(matchesQuery(a, parseQuery('namespace=~('))).toBe(false);
  });

  it('matches a bare word found only in resolved labels/annotations/fields', () => {
    const resolved = mkAlert({
      resolvedLabels: { cluster: 'shipping-euwest1' },
      resolvedAnnotations: { runbook: 'see confluence page zephyr' },
      resolvedFields: { owner: 'team-orion' },
    });
    expect(matchesQuery(resolved, parseQuery('shipping-euwest1'))).toBe(true);
    expect(matchesQuery(resolved, parseQuery('zephyr'))).toBe(true);
    expect(matchesQuery(resolved, parseQuery('team-orion'))).toBe(true);
    expect(matchesQuery(resolved, parseQuery('nowhere'))).toBe(false);
  });
});

describe('queryToMatchers', () => {
  it('converts label terms and drops text + annotation terms', () => {
    const { matchers, dropped } = queryToMatchers(
      parseQuery('critical namespace=prod team!=infra app=~api-.* annotation:summary=~disk'),
    );
    expect(matchers).toEqual([
      { name: 'namespace', value: 'prod', isRegex: false, isEqual: true },
      { name: 'team', value: 'infra', isRegex: false, isEqual: false },
      { name: 'app', value: 'api-.*', isRegex: true, isEqual: true },
    ]);
    expect(dropped).toEqual([
      { label: 'critical', reason: 'text' },
      { label: 'annotation:summary', reason: 'annotation' },
    ]);
  });

  it('maps !~ to a negated regex matcher', () => {
    const { matchers } = queryToMatchers(parseQuery('env!~dev.*'));
    expect(matchers).toEqual([{ name: 'env', value: 'dev.*', isRegex: true, isEqual: false }]);
  });

  it('prefixes a negated text term with a dash in the dropped label', () => {
    const { dropped } = queryToMatchers(parseQuery('-noise'));
    expect(dropped).toEqual([{ label: '-noise', reason: 'text' }]);
  });
});
