import { writable, derived } from 'svelte/store';
import { alerts, resolveAlertField } from './alerts';
import type { Alert } from './alerts';

export interface FilterState {
  text: string;
  severity: string; // 'all' | 'critical' | 'warning' | 'info'
  source: string;   // 'all' | source name
  showSilenced: boolean;
  showAll: boolean;
}

export const filter = writable<FilterState>({
  text: '',
  severity: 'all',
  source: 'all',
  showSilenced: true,
  showAll: false,
});

export const filteredAlerts = derived([alerts, filter], ([$alerts, $filter]) => {
  return $alerts.filter(alert => matchesFilter(alert, $filter));
});

function matchesFilter(alert: Alert, f: FilterState): boolean {
  if (f.showAll) return true;
  if (f.severity !== 'all' && alert.severity !== f.severity) return false;
  if (f.source !== 'all' && alert.source !== f.source) return false;
  if (!f.showSilenced && alert.silencedBy?.length > 0) return false;

  if (f.text) {
    const q = f.text.toLowerCase();
    const haystack = [
      alert.name,
      alert.source,
      ...Object.keys(alert.labels || {}).map(key => resolveAlertField(alert, `label:${key}`) ?? ''),
      ...Object.keys(alert.annotations || {}).map(key => resolveAlertField(alert, `annotation:${key}`) ?? ''),
    ].join(' ').toLowerCase();
    if (!haystack.includes(q)) return false;
  }

  return true;
}

export const availableSources = derived(alerts, ($alerts) => {
  return [...new Set($alerts.map(a => a.source))].sort();
});
