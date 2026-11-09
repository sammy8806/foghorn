// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ManualRefresh, RefreshAlerts } from '../../wailsjs/go/main/App';
import { refreshAlerts } from './alerts';

vi.mock('../../wailsjs/go/main/App', async (importOriginal) => {
  const real = await importOriginal<Record<string, unknown>>();
  return Object.fromEntries(Object.keys(real).map(k => [k, vi.fn().mockResolvedValue(null)]));
});

beforeEach(() => {
  (window as any).go = {};
});

afterEach(() => {
  delete (window as any).go;
  vi.clearAllMocks();
});

describe('refreshAlerts', () => {
  it('a manual refresh asks the backend to forget refused silence writes', async () => {
    await refreshAlerts({ manual: true });
    expect(ManualRefresh).toHaveBeenCalledTimes(1);
    expect(RefreshAlerts).not.toHaveBeenCalled();
  });

  it('an automatic refresh keeps them', async () => {
    await refreshAlerts();
    expect(RefreshAlerts).toHaveBeenCalledTimes(1);
    expect(ManualRefresh).not.toHaveBeenCalled();
  });
});
