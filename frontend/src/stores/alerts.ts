import { writable, derived } from 'svelte/store';
import { GetAlerts, GetSeverityCounts, GetDisplayConfig } from '../../wailsjs/go/main/App';
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

export interface DisplayConfig {
  visible_labels: string[];
  visible_annotations: string[];
  group_by: string[];
  sort_by: string;
}

export const alerts = writable<Alert[]>([]);
export const severityCounts = writable<SeverityCounts>({ critical: 0, warning: 0, info: 0 });
export const displayConfig = writable<DisplayConfig>({
  visible_labels: ['alertname', 'severity', 'cluster', 'namespace'],
  visible_annotations: ['summary'],
  group_by: ['cluster'],
  sort_by: 'severity',
});

export const loading = writable(true);
export const error = writable<string | null>(null);

export async function refreshAlerts(): Promise<void> {
  try {
    const [alertList, counts] = await Promise.all([
      GetAlerts(),
      GetSeverityCounts(),
    ]);
    alerts.set(alertList || []);
    severityCounts.set(counts || { critical: 0, warning: 0, info: 0 });
    error.set(null);
  } catch (e) {
    error.set(String(e));
  } finally {
    loading.set(false);
  }
}

export async function loadDisplayConfig(): Promise<void> {
  try {
    const cfg = await GetDisplayConfig();
    if (cfg) displayConfig.set(cfg);
  } catch (_) {
    // use defaults
  }
}

// Subscribe to backend push events
export function initEventListeners(): void {
  EventsOn('alerts:updated', () => {
    refreshAlerts();
  });
}

// Derived: alerts grouped by the display config's group_by labels
export const groupedAlerts = derived(
  [alerts, displayConfig],
  ([$alerts, $cfg]) => {
    const groupBy = $cfg.group_by || [];
    if (groupBy.length === 0) {
      return { ungrouped: [...$alerts].sort(sortByConfig($cfg)) };
    }

    const groups: Record<string, Alert[]> = {};
    for (const alert of $alerts) {
      const key = groupBy.map(label => alert.labels[label] || '').join('/') || 'other';
      if (!groups[key]) groups[key] = [];
      groups[key].push(alert);
    }

    // Sort within each group
    for (const key of Object.keys(groups)) {
      groups[key].sort(sortByConfig($cfg));
    }

    return groups;
  }
);

function sortByConfig(cfg: DisplayConfig) {
  return (a: Alert, b: Alert): number => {
    if (cfg.sort_by === 'severity') {
      const diff = severityOrder(a.severity) - severityOrder(b.severity);
      if (diff !== 0) return diff;
    }
    return new Date(b.startsAt).getTime() - new Date(a.startsAt).getTime();
  };
}
