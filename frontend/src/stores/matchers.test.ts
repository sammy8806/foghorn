import { describe, it, expect } from 'vitest';
import {
  matchesMatcher,
  matchesAllMatchers,
  valuesToRegexMatcher,
  parseMatcherBlock,
  formatMatcherBlock,
} from './matchers';
import type { Alert } from './alerts';

function mkAlert(labels: Record<string, string>): Alert {
  return {
    id: 'a', source: 'prod-am', sourceType: 'alertmanager', name: 'X',
    severity: 'critical', state: 'firing', labels, annotations: {},
    startsAt: '', updatedAt: '', generatorURL: '',
    silencedBy: [], inhibitedBy: [], receivers: [],
  };
}

describe('matchesMatcher', () => {
  it('exact = / !=', () => {
    expect(matchesMatcher('prod', { name: 'ns', value: 'prod', isRegex: false, isEqual: true })).toBe(true);
    expect(matchesMatcher('dev', { name: 'ns', value: 'prod', isRegex: false, isEqual: true })).toBe(false);
    expect(matchesMatcher('dev', { name: 'ns', value: 'prod', isRegex: false, isEqual: false })).toBe(true);
  });
  it('anchored regex =~ / !~', () => {
    expect(matchesMatcher('api-1', { name: 'app', value: 'api-.*', isRegex: true, isEqual: true })).toBe(true);
    expect(matchesMatcher('x-api-1', { name: 'app', value: 'api-.*', isRegex: true, isEqual: true })).toBe(false);
    expect(matchesMatcher('web', { name: 'app', value: 'api-.*', isRegex: true, isEqual: false })).toBe(true);
  });
  it('invalid regex matches nothing', () => {
    expect(matchesMatcher('x', { name: 'a', value: '(', isRegex: true, isEqual: true })).toBe(false);
  });
});

describe('matchesAllMatchers', () => {
  const a = mkAlert({ ns: 'prod', team: 'db' });
  it('ANDs matchers over raw labels, missing = empty', () => {
    expect(matchesAllMatchers(a, [
      { name: 'ns', value: 'prod', isRegex: false, isEqual: true },
      { name: 'team', value: 'infra', isRegex: false, isEqual: false },
    ])).toBe(true);
    expect(matchesAllMatchers(a, [
      { name: 'missing', value: 'x', isRegex: false, isEqual: true },
    ])).toBe(false);
  });
});

describe('valuesToRegexMatcher', () => {
  it('builds an escaped alternation regex matcher', () => {
    expect(valuesToRegexMatcher('app', ['api-1', 'a.b'])).toEqual({
      name: 'app', value: 'api\\-1|a\\.b', isRegex: true, isEqual: true,
    });
  });
});

describe('parseMatcherBlock', () => {
  it('parses newline- and comma-separated field terms, skipping bad lines', () => {
    const { matchers, skipped } = parseMatcherBlock('ns=prod\napp=~api.*, team!=infra\ngarbage\nannotation:x=y');
    expect(matchers).toEqual([
      { name: 'ns', value: 'prod', isRegex: false, isEqual: true },
      { name: 'app', value: 'api.*', isRegex: true, isEqual: true },
      { name: 'team', value: 'infra', isRegex: false, isEqual: false },
    ]);
    expect(skipped).toBe(2); // "garbage" + annotation-scoped line
  });
  it('strips alertmanager-style quotes', () => {
    const { matchers } = parseMatcherBlock('severity="critical"');
    expect(matchers).toEqual([{ name: 'severity', value: 'critical', isRegex: false, isEqual: true }]);
  });
  it('keeps commas inside quoted values', () => {
    const { matchers, skipped } = parseMatcherBlock('summary="a,b", severity=critical');
    expect(skipped).toBe(0);
    expect(matchers).toEqual([
      { name: 'summary', value: 'a,b', isRegex: false, isEqual: true },
      { name: 'severity', value: 'critical', isRegex: false, isEqual: true },
    ]);
  });
});

describe('formatMatcherBlock', () => {
  it('formats matchers as paste-compatible lines', () => {
    expect(formatMatcherBlock([
      { name: 'namespace', value: 'prod', isRegex: false, isEqual: true },
      { name: 'team', value: 'infra', isRegex: false, isEqual: false },
      { name: 'app', value: 'api-.*', isRegex: true, isEqual: true },
      { name: 'pod', value: 'job runner,1', isRegex: true, isEqual: false },
    ])).toBe('namespace=prod\nteam!=infra\napp=~api-.*\npod!~"job runner,1"');
  });

  it('round-trips through parseMatcherBlock', () => {
    const original = [
      { name: 'namespace', value: 'prod', isRegex: false, isEqual: true },
      { name: 'summary', value: 'disk full,node-1', isRegex: false, isEqual: true },
      { name: 'message', value: 'node says "full"', isRegex: false, isEqual: true },
    ];
    const { matchers, skipped } = parseMatcherBlock(formatMatcherBlock(original));
    expect(skipped).toBe(0);
    expect(matchers).toEqual(original);
  });
});
