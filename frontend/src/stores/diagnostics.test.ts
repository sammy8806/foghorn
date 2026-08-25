import { beforeEach, describe, expect, it } from 'vitest';
import { get } from 'svelte/store';
import {
  applyConfigDiagnostics,
  connectConfigDiagnostics,
  dismissConfigDiagnostics,
  resetConfigDiagnosticsForTest,
  visibleConfigDiagnostics,
} from './diagnostics';

const first = {
  path: '/tmp/config.yaml',
  fingerprint: 'first',
  items: [{ field: 'resolvers[0]', message: 'stdin is required', dropped: true }],
};

describe('config diagnostics lifecycle', () => {
  beforeEach(resetConfigDiagnosticsForTest);

  it('dismisses only the current fingerprint', () => {
    applyConfigDiagnostics(first);
    expect(get(visibleConfigDiagnostics)?.fingerprint).toBe('first');

    dismissConfigDiagnostics();
    expect(get(visibleConfigDiagnostics)).toBeNull();

    applyConfigDiagnostics({ ...first, fingerprint: 'second' });
    expect(get(visibleConfigDiagnostics)?.fingerprint).toBe('second');
  });

  it('clears the banner after a clean reload', () => {
    applyConfigDiagnostics(first);
    applyConfigDiagnostics({ path: first.path, fingerprint: '', items: [] });
    expect(get(visibleConfigDiagnostics)).toBeNull();
  });

  it('does not let the startup fetch overwrite a newer event', async () => {
    let resolveFetch!: (payload: typeof first) => void;
    const fetchPromise = new Promise<typeof first>(resolve => {
      resolveFetch = resolve;
    });
    let onEvent!: (payload: typeof first) => void;

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

    expect(get(visibleConfigDiagnostics)?.fingerprint).toBe('event');
  });
});
