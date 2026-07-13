import { writable, derived } from 'svelte/store';
import { alerts } from './alerts';
import type { Alert } from './alerts';
import { parseQuery, matchesQuery, type ParsedQuery } from './query';

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

// parsedQuery is the single parse of the search box, reused by the list filter
// and by "silence from search" (AlertList).
export const parsedQuery = derived(filter, ($filter) => parseQuery($filter.text));

export const filteredAlerts = derived(
  [alerts, filter, parsedQuery],
  ([$alerts, $filter, $parsed]) => $alerts.filter((alert) => matchesFilter(alert, $filter, $parsed)),
);

export const hiddenCount = derived(
  [alerts, filteredAlerts],
  ([$alerts, $filtered]) => Math.max(0, $alerts.length - $filtered.length),
);

function matchesFilter(alert: Alert, f: FilterState, parsed: ParsedQuery): boolean {
  // Text/structured search always applies, even when showAll is active, so users
  // can search within silenced/otherwise-hidden alerts.
  if (parsed.terms.length > 0 && !matchesQuery(alert, parsed)) return false;

  if (f.showAll) return true;
  if (alert.hiddenBy && alert.hiddenBy.length > 0) return false;
  if (f.severity !== 'all' && alert.severity !== f.severity) return false;
  if (f.source !== 'all' && alert.source !== f.source) return false;
  if (!f.showSilenced && alert.silencedBy?.length > 0) return false;

  return true;
}

export const availableSources = derived(alerts, ($alerts) => {
  return [...new Set($alerts.map((a) => a.source))].sort();
});
