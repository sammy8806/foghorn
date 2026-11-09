import { describe, expect, it } from 'vitest';
import type { Matcher, SilenceAccess } from './alerts';
import { decideAcrossSources, decideSilence, RULE_UID_LABEL, silenceReadHint } from './silenceAccess';

const eq = (name: string, value: string): Matcher => ({ name, value, isRegex: false, isEqual: true });
const r1 = [eq('alertname', 'Down'), eq(RULE_UID_LABEL, 'r1')];
const general = [eq('alertname', 'Down')];

function access(over: Partial<SilenceAccess> = {}): SilenceAccess {
  return {
    granularity: 'folder',
    check: 'ok',
    checkedAt: '',
    canCreateAll: false,
    canWriteAll: false,
    canRead: true,
    createFolders: [],
    writeFolders: [],
    ruleFolders: { r1: 'child' },
    folderParents: { child: 'parent', parent: '' },
    folderTitles: { child: 'Child', parent: 'Parent', other: 'Other' },
    ...over,
    denied: {
      createSource: false, writeSource: false, createGeneral: false, writeGeneral: false,
      createFolders: [], writeFolders: [], status: 0, message: '', at: '',
      ...(over.denied ?? {}),
    },
  };
}

const SRC_NO_CREATE = "g: token can't create silences (needs alert.silences:create)";

describe('decideSilence', () => {
  it('access undefined is allowed', () => {
    expect(decideSilence('g', undefined, general, 'create')).toEqual({ allowed: true, reason: '' });
  });

  for (const check of ['unchecked', 'error', 'unsupported'] as const) {
    it(`check ${check} is allowed`, () => {
      expect(decideSilence('g', access({ check }), general, 'create').allowed).toBe(true);
    });
  }

  it('granularity off is allowed', () => {
    expect(decideSilence('g', access({ granularity: 'off' }), general, 'create').allowed).toBe(true);
  });

  it('source granularity without create scopes is refused', () => {
    expect(decideSilence('g', access({ granularity: 'source' }), r1, 'create')).toEqual({ allowed: false, reason: SRC_NO_CREATE });
  });

  it('source granularity with one folder scope is allowed', () => {
    expect(decideSilence('g', access({ granularity: 'source', createFolders: ['x'] }), general, 'create').allowed).toBe(true);
  });

  it('folder: rule in permitted folder', () => {
    expect(decideSilence('g', access({ createFolders: ['child'] }), r1, 'create').allowed).toBe(true);
  });

  it('folder: permission inherited from ancestor', () => {
    expect(decideSilence('g', access({ createFolders: ['parent'] }), r1, 'create').allowed).toBe(true);
  });

  it('folder: other folder refused with title', () => {
    expect(decideSilence('g', access({ createFolders: ['other'] }), r1, 'create')).toEqual({
      allowed: false,
      reason: `g: token can't silence alerts in folder "Child" (needs alert.silences:create on that folder)`,
    });
  });

  it('folder: general silence needs all folders', () => {
    expect(decideSilence('g', access({ createFolders: ['child'] }), general, 'create')).toEqual({
      allowed: false,
      reason: "g: token can only silence single alert rules; keep the __alert_rule_uid__ matcher (needs alert.silences:create on all folders)",
    });
    expect(decideSilence('g', access({ canCreateAll: true }), general, 'create').allowed).toBe(true);
  });

  it('folder: unknown rule falls back to source granularity', () => {
    const m = [eq(RULE_UID_LABEL, 'nope')];
    expect(decideSilence('g', access({ createFolders: ['other'] }), m, 'create').allowed).toBe(true);
    expect(decideSilence('g', access(), m, 'create')).toEqual({ allowed: false, reason: SRC_NO_CREATE });
  });

  it('regex rule-uid matcher counts as general', () => {
    const m = [{ name: RULE_UID_LABEL, value: 'r1', isRegex: true, isEqual: true }];
    expect(decideSilence('g', access({ createFolders: ['child'] }), m, 'create').allowed).toBe(false);
  });

  it('write uses write permissions', () => {
    expect(decideSilence('g', access({ createFolders: ['child'], writeFolders: ['other'] }), r1, 'write')).toEqual({
      allowed: false,
      reason: `g: token can't edit or expire silences in folder "Child" (needs alert.silences:write on that folder)`,
    });
    expect(decideSilence('g', access({ writeFolders: ['parent'] }), r1, 'write').allowed).toBe(true);
    expect(decideSilence('g', access({ writeFolders: ['child'] }), general, 'write').reason).toBe(
      "g: token can only edit silences of single alert rules; keep the __alert_rule_uid__ matcher (needs alert.silences:write on all folders)",
    );
    expect(decideSilence('g', access({ granularity: 'source' }), r1, 'write').reason).toBe(
      "g: token can't edit or expire silences (needs alert.silences:write)",
    );
  });

  it('denied source wins even when unchecked', () => {
    const a = access({ check: 'unchecked', denied: { createSource: true, status: 403 } as any });
    expect(decideSilence('g', a, r1, 'create')).toEqual({
      allowed: false,
      reason: 'g: a silence was refused (HTTP 403); fix the credentials, then refresh',
    });
    expect(decideSilence('g', a, r1, 'write').allowed).toBe(true);
  });

  it('denied folder blocks only that folder', () => {
    const a = access({ canCreateAll: true, denied: { createFolders: ['child'], status: 403 } as any });
    expect(decideSilence('g', a, r1, 'create').allowed).toBe(false);
    expect(decideSilence('g', a, general, 'create').allowed).toBe(true);
  });

  it('denied general blocks only general silences', () => {
    const a = access({ createFolders: ['child'], canCreateAll: true, denied: { createGeneral: true, status: 403 } as any });
    expect(decideSilence('g', a, general, 'create').allowed).toBe(false);
    expect(decideSilence('g', a, r1, 'create').allowed).toBe(true);
  });

  it('alertmanager access is allowed unless denied', () => {
    const a = access({ granularity: '', check: 'unchecked' });
    expect(decideSilence('am', a, general, 'create').allowed).toBe(true);
  });

  it('folder: a token with no create scope at all gets the source text', () => {
    expect(decideSilence('g', access(), r1, 'create')).toEqual({ allowed: false, reason: SRC_NO_CREATE });
    expect(decideSilence('g', access(), general, 'create')).toEqual({ allowed: false, reason: SRC_NO_CREATE });
  });

  it('ancestor walk terminates on a cycle', () => {
    const a = access({ ruleFolders: { r1: 'a' }, folderParents: { a: 'b', b: 'a' }, createFolders: ['x'] });
    expect(decideSilence('g', a, r1, 'create').allowed).toBe(false);
  });

  it('null maps from the backend are tolerated', () => {
    const a = access({ ruleFolders: null, folderParents: null, folderTitles: null, createFolders: null });
    expect(decideSilence('g', a, r1, 'create')).toEqual({ allowed: false, reason: SRC_NO_CREATE });
  });
});

describe('silenceReadHint', () => {
  it('explains hidden details when the token cannot read', () => {
    expect(silenceReadHint('g', access({ canRead: false }))).toBe(
      "silence details hidden: token can't read silences (needs alert.silences:read)",
    );
  });
  it('is null while unchecked', () => {
    expect(silenceReadHint('g', access({ check: 'unchecked', canRead: false }))).toBeNull();
    expect(silenceReadHint('g', undefined)).toBeNull();
  });
});

describe('decideAcrossSources', () => {
  const caps = {
    b: { supportsSilence: true, silenceAccess: access({ granularity: 'source' }) },
    a: { supportsSilence: true, silenceAccess: access({ granularity: 'source' }) },
    prom: { supportsSilence: false },
  };
  it('refuses with the first capable source by name', () => {
    expect(decideAcrossSources(caps, ['prom', 'b', 'a'], general, 'create').reason).toBe(
      "a: token can't create silences (needs alert.silences:create)",
    );
  });
  it('allows when any capable source allows', () => {
    const more = { ...caps, c: { supportsSilence: true, silenceAccess: access({ check: 'error' }) } };
    expect(decideAcrossSources(more, ['a', 'b', 'c'], general, 'create').allowed).toBe(true);
  });
});
