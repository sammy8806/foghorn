import { describe, it, expect } from 'vitest';
import { parseQuery, parseFieldTerm } from './query';

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
