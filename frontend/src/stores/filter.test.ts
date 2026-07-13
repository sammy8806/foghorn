import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { filter, filteredAlerts, parsedQuery, bypassableHiddenCount } from './filter';
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

describe('bypassableHiddenCount', () => {
  it('ignores alerts hidden only by text search', () => {
    alerts.set([
      mkAlert({ name: 'Visible', severity: 'critical' }),
      mkAlert({ name: 'SeverityHidden', severity: 'warning' }),
      mkAlert({ name: 'TextHidden', severity: 'critical', labels: { job: 'api' } }),
    ]);
    filter.set({ text: 'visible', severity: 'critical', source: 'all', showSilenced: true, showAll: false });
    expect(get(bypassableHiddenCount)).toBe(0);
  });

  it('counts alerts that show all would reveal', () => {
    alerts.set([
      mkAlert({ name: 'VisibleCritical', severity: 'critical' }),
      mkAlert({ name: 'VisibleWarning', severity: 'warning' }),
      mkAlert({ name: 'Silenced', severity: 'critical', silencedBy: ['s1'] }),
    ]);
    filter.set({ text: 'visible', severity: 'critical', source: 'all', showSilenced: false, showAll: false });
    expect(get(bypassableHiddenCount)).toBe(1);
  });

  it('returns zero when show all is already active', () => {
    alerts.set([mkAlert({ name: 'Hidden', severity: 'warning' })]);
    filter.set({ text: '', severity: 'critical', source: 'all', showSilenced: true, showAll: true });
    expect(get(bypassableHiddenCount)).toBe(0);
  });
});
