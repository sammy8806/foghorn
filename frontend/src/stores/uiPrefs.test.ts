import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { activeGroupMode, activeSortMode, verbose } from './alerts';
import { filter } from './filter';
import { hasPersistedUIPrefs, initUIPrefsPersistence, restoreUIPrefs } from './uiPrefs';

function createStorage(): Storage {
  const data = new Map<string, string>();
  return {
    get length() { return data.size; },
    clear: () => data.clear(),
    getItem: (key: string) => data.get(key) ?? null,
    key: (index: number) => [...data.keys()][index] ?? null,
    removeItem: (key: string) => { data.delete(key); },
    setItem: (key: string, value: string) => { data.set(key, value); },
  };
}

describe('uiPrefs', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', createStorage());
    activeSortMode.set('default');
    activeGroupMode.set('default');
    verbose.set(false);
    filter.set({ text: '', severity: 'all', source: 'all', showSilenced: true, showAll: false });
  });

  it('restores persisted sort/group/filter/verbose settings', () => {
    localStorage.setItem('foghorn.ui-prefs.v1', JSON.stringify({
      activeSortMode: 'severity',
      activeGroupMode: 'cluster',
      verbose: true,
      filter: {
        text: 'namespace=prod',
        severity: 'critical',
        source: 'prod-am',
        showSilenced: false,
        showAll: true,
      },
    }));

    restoreUIPrefs();

    expect(get(activeSortMode)).toBe('severity');
    expect(get(activeGroupMode)).toBe('cluster');
    expect(get(verbose)).toBe(true);
    expect(get(filter)).toMatchObject({
      text: 'namespace=prod',
      severity: 'critical',
      source: 'prod-am',
      showSilenced: false,
      showAll: true,
    });
  });

  it('persists changes after init', async () => {
    const cleanup = initUIPrefsPersistence();
    activeSortMode.set('state');
    await new Promise(resolve => setTimeout(resolve, 200));
    cleanup();

    expect(hasPersistedUIPrefs()).toBe(true);
    const saved = JSON.parse(localStorage.getItem('foghorn.ui-prefs.v1') || '{}');
    expect(saved.activeSortMode).toBe('state');
  });
});
