import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { filter, filteredAlerts, parsedQuery, hiddenCount } from './filter';
import { alerts } from './alerts';
import type { Alert } from './alerts';

function mkAlert(over: Partial<Alert>): Alert {
  return {
    id: Math.random().toString(), source: 'prod-am', sourceType: 'alertmanager',
    name: 'A', severity: 'critical', state: 'firing',
    labels: {}, annotations: {}, startsAt: '', updatedAt: '', generatorURL: '',
    silencedBy: [], inhibitedBy: [], receivers: [],
    ...over,
  };
}

describe('filteredAlerts with query grammar', () => {
  it('bare-word substring still works (regression)', () => {
    alerts.set([
      mkAlert({ name: 'DiskFull', labels: { severity: 'critical' } }),
      mkAlert({ name: 'CpuHigh', labels: { severity: 'warning' } }),
    ]);
    filter.set({ text: 'disk', severity: 'all', source: 'all', showSilenced: true, showAll: true });
    expect(get(filteredAlerts).map((a) => a.name)).toEqual(['DiskFull']);
  });

  it('field terms filter the list', () => {
    alerts.set([
      mkAlert({ name: 'A', labels: { namespace: 'prod' } }),
      mkAlert({ name: 'B', labels: { namespace: 'dev' } }),
    ]);
    filter.set({ text: 'namespace=prod', severity: 'all', source: 'all', showSilenced: true, showAll: true });
    expect(get(filteredAlerts).map((a) => a.name)).toEqual(['A']);
  });

  it('exposes parsedQuery', () => {
    filter.set({ text: 'x=1', severity: 'all', source: 'all', showSilenced: true, showAll: true });
    expect(get(parsedQuery).terms).toEqual([
      { kind: 'field', scope: 'label', key: 'x', op: '=', value: '1' },
    ]);
  });
});

describe('hiddenCount', () => {
  it('counts alerts excluded by any active filter', () => {
    alerts.set([
      mkAlert({ name: 'Visible', severity: 'critical' }),
      mkAlert({ name: 'Hidden', severity: 'warning' }),
      mkAlert({ name: 'NoMatch', severity: 'critical', labels: { job: 'db' } }),
    ]);
    filter.set({ text: 'visible', severity: 'all', source: 'all', showSilenced: true, showAll: false });
    expect(get(hiddenCount)).toBe(2);
  });
});
