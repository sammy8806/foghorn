import { beforeEach, describe, expect, it } from 'vitest';
import { get } from 'svelte/store';
import {
  applyConfigDiagnostics,
  configDiagnostics,
  connectConfigDiagnostics,
  resetConfigDiagnosticsForTest,
  type ConfigDiagnosticsPayload,
} from './diagnostics';

const first: ConfigDiagnosticsPayload = {
  path: '/tmp/config.yaml',
  fingerprint: 'first',
  items: [{ field: 'resolvers[0]', message: 'stdin is required', locator: '', dropped: true }],
  excerpt: [],
};

describe('config diagnostics lifecycle', () => {
  beforeEach(resetConfigDiagnosticsForTest);

  it('clears the payload after a clean reload', () => {
    applyConfigDiagnostics(first);
    expect(get(configDiagnostics).items).toHaveLength(1);

    applyConfigDiagnostics({ path: first.path, fingerprint: '', items: [], excerpt: [] });
    expect(get(configDiagnostics).items).toEqual([]);
  });

  it('fills in what a payload from the bridge leaves out', () => {
    applyConfigDiagnostics(undefined);

    expect(get(configDiagnostics)).toEqual({ path: '', fingerprint: '', items: [], excerpt: [] });
  });

  it('keeps the excerpt that came with the payload', () => {
    applyConfigDiagnostics({
      ...first,
      items: [{ field: 'config', message: 'mapping values are not allowed', locator: 'line 4', dropped: false }],
      excerpt: [{ number: 4, text: '  severity: warn: high', marked: true }],
    });

    expect(get(configDiagnostics).excerpt).toEqual([{ number: 4, text: '  severity: warn: high', marked: true }]);
  });

  it('does not let the startup fetch overwrite a newer event', async () => {
    let resolveFetch!: (payload: ConfigDiagnosticsPayload) => void;
    const fetchPromise = new Promise<ConfigDiagnosticsPayload>(resolve => {
      resolveFetch = resolve;
    });
    let onEvent!: (payload: ConfigDiagnosticsPayload) => void;

    connectConfigDiagnostics(
      () => fetchPromise,
      callback => {
        onEvent = callback;
        return () => {};
      },
    );

    onEvent({ ...first, fingerprint: 'event' });
    resolveFetch({ ...first, fingerprint: 'stale-fetch' });
    await fetchPromise;
    await Promise.resolve();

    expect(get(configDiagnostics).fingerprint).toBe('event');
  });
});
