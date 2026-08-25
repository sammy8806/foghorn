import { beforeEach, describe, expect, it } from 'vitest';
import { get } from 'svelte/store';
import {
  applyConfigDiagnostics,
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
});
