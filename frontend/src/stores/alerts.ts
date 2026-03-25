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
  subtitle_annotations: string[];
  group_by: string[];
  sort_by: string;
}

export const alerts = writable<Alert[]>([]);
export const severityCounts = writable<SeverityCounts>({ critical: 0, warning: 0, info: 0 });
export const displayConfig = writable<DisplayConfig>({
  visible_labels: ['alertname', 'severity', 'cluster', 'namespace'],
  visible_annotations: ['summary'],
  subtitle_annotations: ['summary', 'description'],
  group_by: ['cluster'],
  sort_by: 'severity',
});

export const verbose = writable(false);
export const loading = writable(true);
export const error = writable<string | null>(null);

export async function refreshAlerts(): Promise<void> {
  try {
    if (!isWails()) {
      // No backend — show empty state instead of hanging/crashing.
      alerts.set([]);
      severityCounts.set({ critical: 0, warning: 0, info: 0 });
      error.set('Dev mode: no Wails backend connected');
      return;
    }
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
  if (!isWails()) return; // use defaults in dev mode
  try {
    const cfg = await GetDisplayConfig();
    if (cfg) displayConfig.set(cfg);
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
