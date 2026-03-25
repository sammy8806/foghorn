import { writable, derived, get } from 'svelte/store';
import { GetAlerts, GetSeverityCounts, GetDisplayConfig, GetSourcesHealth } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { severityOrder } from '../utils/severity';

export interface Alert {
  id: string;
  source: string;
  sourceType: string;
  name: string;
  severity: string;
  state: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  updatedAt: string;
  generatorURL: string;
  silencedBy: string[];
  inhibitedBy: string[];
  receivers: string[];
}

export interface SeverityCounts {
  critical: number;
  warning: number;
  info: number;
}

export interface SortCriterion {
  field: string;
  order: 'asc' | 'desc';
}

export interface SortPresetOption {
  mode: string;
  label: string;
  criteria?: SortCriterion[];
}

export interface GroupPresetOption {
  mode: string;
  label: string;
  fields?: string[];
}

export interface DisplayConfig {
  visible_labels: string[];
  visible_annotations: string[];
  subtitle_annotations: string[];
  group_by: string[];
  sort_by: SortCriterion[];
}

export const alerts = writable<Alert[]>([]);
export const severityCounts = writable<SeverityCounts>({ critical: 0, warning: 0, info: 0 });
export const displayConfig = writable<DisplayConfig>({
  visible_labels: ['alertname', 'severity', 'cluster', 'namespace'],
  visible_annotations: ['summary'],
  subtitle_annotations: ['summary', 'description'],
  group_by: ['cluster'],
  sort_by: [{ field: 'field:severity', order: 'asc' }, { field: 'field:startsAt', order: 'desc' }],
});

// Sort presets for the UI toggle. Each entry resolves to a []SortCriterion.
export const SORT_PRESETS: Record<string, SortCriterion[]> = {
  severity: [
    { field: 'field:severity', order: 'asc' },
    { field: 'field:startsAt', order: 'desc' },
  ],
  first_seen: [{ field: 'field:startsAt', order: 'asc' }],
  last_seen: [{ field: 'field:updatedAt', order: 'desc' }],
  active_first: [
    { field: 'field:state', order: 'asc' },
    { field: 'field:severity', order: 'asc' },
  ],
  source: [
    { field: 'field:source', order: 'asc' },
    { field: 'field:severity', order: 'asc' },
  ],
  cluster: [
    { field: 'label:cluster', order: 'asc' },
    { field: 'field:severity', order: 'asc' },
  ],
  alert_name: [
    { field: 'field:name', order: 'asc' },
    { field: 'field:severity', order: 'asc' },
  ],
};

export const SORT_PRESET_OPTIONS: SortPresetOption[] = [
  { mode: 'default', label: 'Default' },
  { mode: 'severity', label: 'Severity', criteria: SORT_PRESETS.severity },
  { mode: 'first_seen', label: 'First seen', criteria: SORT_PRESETS.first_seen },
  { mode: 'last_seen', label: 'Last seen', criteria: SORT_PRESETS.last_seen },
  { mode: 'active_first', label: 'Active first', criteria: SORT_PRESETS.active_first },
  { mode: 'source', label: 'Source', criteria: SORT_PRESETS.source },
  { mode: 'cluster', label: 'Cluster', criteria: SORT_PRESETS.cluster },
  { mode: 'alert_name', label: 'Alert name', criteria: SORT_PRESETS.alert_name },
];

export const GROUP_PRESETS: Record<string, string[]> = {
  none: [],
  source: ['field:source'],
  severity: ['field:severity'],
  state: ['field:state'],
  namespace: ['label:namespace'],
  cluster: ['label:cluster'],
};

export const GROUP_PRESET_OPTIONS: GroupPresetOption[] = [
  { mode: 'default', label: 'Default' },
  { mode: 'none', label: 'None', fields: GROUP_PRESETS.none },
  { mode: 'source', label: 'Source', fields: GROUP_PRESETS.source },
  { mode: 'severity', label: 'Severity', fields: GROUP_PRESETS.severity },
  { mode: 'state', label: 'State', fields: GROUP_PRESETS.state },
  { mode: 'namespace', label: 'Namespace', fields: GROUP_PRESETS.namespace },
  { mode: 'cluster', label: 'Cluster', fields: GROUP_PRESETS.cluster },
];

// Ephemeral sort mode — resets to 'default' on app restart.
export const activeSortMode = writable<string>('default');
export const activeGroupMode = writable<string>('default');

// Resolved criteria: 'default' uses the config value, named presets use SORT_PRESETS.
export const activeSortCriteria = derived(
  [activeSortMode, displayConfig],
  ([$mode, $cfg]) => {
    if ($mode === 'default') return $cfg.sort_by;
    return SORT_PRESETS[$mode] ?? $cfg.sort_by;
  }
);

export const activeGroupBy = derived(
  [activeGroupMode, displayConfig],
  ([$mode, $cfg]) => {
    if ($mode === 'default') return $cfg.group_by;
    return GROUP_PRESETS[$mode] ?? $cfg.group_by;
  }
);

export interface SourceHealth {
  source: string;
  ok: boolean;
  lastPoll: string;
  lastError?: string;
  consecFails: number;
}

export const verbose = writable(false);
export const loading = writable(true);
export const error = writable<string | null>(null);
export const sourcesHealth = writable<SourceHealth[]>([]);
// Set of "source:id" keys for alerts that appeared in the latest refresh
export const newAlertKeys = writable<Set<string>>(new Set());
let hasLoadedOnce = false;

export async function refreshAlerts(): Promise<void> {
  try {
    if (!isWails()) {
      // No backend — show empty state instead of hanging/crashing.
      alerts.set([]);
      severityCounts.set({ critical: 0, warning: 0, info: 0 });
      error.set('Dev mode: no Wails backend connected');
      return;
    }
    const [alertList, counts, health] = await Promise.all([
      GetAlerts(),
      GetSeverityCounts(),
      GetSourcesHealth(),
    ]);
    const incoming = alertList || [];

    // Detect newly appeared alerts (skip first load so initial alerts aren't all highlighted)
    const prev = get(alerts);
    if (prev.length > 0 || hasLoadedOnce) {
      const prevKeys = new Set(prev.map(a => a.source + ':' + a.id));
      const freshKeys = new Set<string>();
      for (const a of incoming) {
        const key = a.source + ':' + a.id;
        if (!prevKeys.has(key)) freshKeys.add(key);
      }
      newAlertKeys.set(freshKeys);
    }
    hasLoadedOnce = true;

    alerts.set(incoming);
    severityCounts.set(counts || { critical: 0, warning: 0, info: 0 });
    sourcesHealth.set(health || []);
    error.set(null);
  } catch (e) {
    error.set(String(e));
  } finally {
    loading.set(false);
  }
}

export async function loadDisplayConfig(): Promise<void> {
  if (!isWails()) return; // use defaults in dev mode
  try {
    const cfg = await GetDisplayConfig();
    if (cfg) {
      displayConfig.set({
        visible_labels: cfg.visible_labels ?? [],
        visible_annotations: cfg.visible_annotations ?? [],
        subtitle_annotations: cfg.subtitle_annotations ?? [],
        group_by: cfg.group_by ?? [],
        sort_by: (cfg.sort_by ?? []).map(criterion => ({
          field: criterion.field,
          order: criterion.order === 'desc' ? 'desc' : 'asc',
        })),
      });
    }
  } catch (_) {
    // use defaults
  }
}

// Detect whether we're running inside the Wails webview or a plain browser.
export const isWails = (): boolean => !!(window as any).runtime || !!(window as any)['go'];

// Both window.runtime (events) and window['go'] (method calls) are injected by
// the Wails webview init script. With StartHidden: true there is a race on first
// paint where neither object exists yet. Poll until both are ready, but give up
// after a short timeout so the app still works in a plain browser (dev mode).
export function waitForBridge(): Promise<void> {
  return new Promise((resolve) => {
    const deadline = Date.now() + 1500;
    const check = () => {
      if ((window as any).runtime) {
        resolve();
      } else if (Date.now() > deadline) {
        // Running outside Wails (e.g. `npm run dev` in a browser) — proceed without bridge.
        resolve();
      } else {
        setTimeout(check, 20);
      }
    };
    check();
  });
}

// Subscribe to backend push events (call after waitForBridge resolves).
export function initEventListeners(): void {
  if (!isWails()) return; // no event bridge in plain browser
  EventsOn('alerts:updated', () => {
    refreshAlerts();
  });
  EventsOn('config:reloaded', () => {
    loadDisplayConfig();
    refreshAlerts();
  });
}

// Derived: alerts grouped by the display config's group_by field references
export const groupedAlerts = derived(
  [alerts, activeSortCriteria, activeGroupBy],
  ([$alerts, $criteria, $groupBy]) => {
    const groupBy = $groupBy || [];
    if (groupBy.length === 0) {
      return { ungrouped: [...$alerts].sort(sortByCriteria($criteria)) };
    }

    const groups: Record<string, Alert[]> = {};
    for (const alert of $alerts) {
      const key = groupBy.map(ref => resolveAlertField(alert, ref) ?? '').join('/') || 'other';
      if (!groups[key]) groups[key] = [];
      groups[key].push(alert);
    }

    // Sort within each group
    for (const key of Object.keys(groups)) {
      groups[key].sort(sortByCriteria($criteria));
    }

    // Return groups in sorted key order for stable rendering
    const sorted: Record<string, Alert[]> = {};
    for (const key of Object.keys(groups).sort()) {
      sorted[key] = groups[key];
    }
    return sorted;
  }
);

// Enum orderings used for sorting
const STATE_ORDER: Record<string, number> = {
  firing: 0,
  silenced: 1,
  inhibited: 2,
  resolved: 3,
};

export function resolveAlertField(alert: Alert, ref: string): string | undefined {
  if (ref.startsWith('field:')) {
    const name = ref.slice(6);
    switch (name) {
      case 'severity': return alert.severity;
      case 'startsAt': return alert.startsAt;
      case 'updatedAt': return alert.updatedAt;
      case 'source': return alert.source;
      case 'name': return alert.name;
      case 'state': return alert.state;
      default: return undefined;
    }
  }
  if (ref.startsWith('label:')) return alert.labels[ref.slice(6)];
  if (ref.startsWith('annotation:')) return alert.annotations[ref.slice(11)];
  // bare string → label (backwards compat)
  return alert.labels[ref];
}

function compareField(a: Alert, b: Alert, criterion: SortCriterion): number {
  const { field, order } = criterion;
  const name = field.startsWith('field:') ? field.slice(6) : null;
  let result = 0;

  if (name === 'severity') {
    result = severityOrder(a.severity) - severityOrder(b.severity);
  } else if (name === 'state') {
    const ao = STATE_ORDER[a.state] ?? 99;
    const bo = STATE_ORDER[b.state] ?? 99;
    result = ao - bo;
  } else if (name === 'startsAt') {
    result = new Date(a.startsAt).getTime() - new Date(b.startsAt).getTime();
  } else if (name === 'updatedAt') {
    result = new Date(a.updatedAt).getTime() - new Date(b.updatedAt).getTime();
  } else {
    const av = resolveAlertField(a, field) ?? '';
    const bv = resolveAlertField(b, field) ?? '';
    result = av.localeCompare(bv);
  }

  return order === 'desc' ? -result : result;
}

export function sortByCriteria(criteria: SortCriterion[]) {
  return (a: Alert, b: Alert): number => {
    for (const criterion of criteria) {
      const diff = compareField(a, b, criterion);
      if (diff !== 0) return diff;
    }
    return 0;
  };
}

export function criteriaEqual(a: SortCriterion[], b: SortCriterion[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((criterion, index) => {
    const other = b[index];
    return criterion.field === other?.field && criterion.order === other?.order;
  });
}

export function stringArrayEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((value, index) => value === b[index]);
}
