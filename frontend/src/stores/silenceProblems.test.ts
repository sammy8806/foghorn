import { describe, expect, it } from 'vitest';
import type { SilenceAccess, SourceHealth } from './alerts';
import { silenceAccessProblems, silenceReadProblem } from './problems';

const health = (over: Partial<SourceHealth> = {}): SourceHealth => ({
  source: 'g', ok: true, pending: false, lastPoll: '', consecFails: 0, ...over,
});

const access = (over: Partial<SilenceAccess> = {}): SilenceAccess => ({
  granularity: 'folder', check: 'ok', checkedAt: '', canCreateAll: true, canWriteAll: true, canRead: true,
  createFolders: [], writeFolders: [], ruleFolders: {}, folderParents: {}, folderTitles: {},
  denied: { createSource: false, writeSource: false, createGeneral: false, writeGeneral: false, createFolders: [], writeFolders: [], status: 0, message: '', at: '' },
  ...over,
});

describe('silence problems', () => {
  it('silence read error', () => {
    const p = silenceReadProblem(health({
      silenceReadError: 'grafana g silences returned HTTP 403: {"message":"forbidden"}',
    }))!;
    expect(p.key).toBe('silence-read:g');
    expect(p.headline).toBe('g silence details unavailable');
    expect(p.severity).toBe('caution');
    expect(p.keyword).toBe('silences');
    expect(p.action).toBeNull();
    expect(p.detail).toContain('(needs alert.silences:read)');
    expect(p.meta).toEqual(['alerts still load; Edit and Expire are hidden']);
  });

  it('silence read error splits Go URL errors into the frame', () => {
    const p = silenceReadProblem(health({
      silenceReadError: 'fetching silences from g: Get "https://g.example/api/alertmanager/grafana/api/v2/silences": dial tcp: no such host',
    }))!;
    expect(p.detail).toBe('dial tcp: no such host');
    expect(p.frame[0].lead).toBe('GET');
  });

  it('permission check failed', () => {
    const [p] = silenceAccessProblems('g', access({ check: 'error', checkError: 'GET /api/access-control/user/permissions: HTTP 500: boom' }));
    expect(p.key).toBe('silence-check:g');
    expect(p.headline).toBe('g permission check failed');
    expect(p.detail).toContain('HTTP 500');
    expect(p.meta).toEqual(['silence buttons stay enabled until the check succeeds']);
    expect(p.severity).toBe('caution');
  });

  it('refused write', () => {
    const problems = silenceAccessProblems('g', access({
      denied: { createSource: false, writeSource: false, createGeneral: true, writeGeneral: false, createFolders: [], writeFolders: [], status: 403, message: 'forbidden', at: '' },
    }));
    expect(problems).toHaveLength(1);
    expect(problems[0].key).toBe('silence-denied:g');
    expect(problems[0].headline).toBe('g refused a silence write');
    expect(problems[0].detail).toBe('HTTP 403: forbidden');
    expect(problems[0].meta).toEqual(['silencing stays disabled until permissions change or the config is reloaded']);
  });

  it('no problem when nothing failed', () => {
    expect(silenceReadProblem(health())).toBeNull();
    expect(silenceAccessProblems('g', access())).toEqual([]);
    expect(silenceAccessProblems('g', undefined)).toEqual([]);
  });

  it('unsupported is silent', () => {
    expect(silenceAccessProblems('g', access({ check: 'unsupported' }))).toEqual([]);
  });
});
