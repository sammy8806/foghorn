import { writable, derived, get } from 'svelte/store';
import { GetAlerts, GetDisplayConfig, GetOnCallStatus, GetSeverityConfig, GetSeverityCounts, GetSourcesHealth } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { emptySeverityCounts, setSeverityConfig, severityConfig, severityOrder } from './severity';

export interface Alert {
  id: string;
  source: string;
  sourceType: string;
  name: string;
  severity: string;
  state: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  resolvedLabels?: Record<string, string>;
  resolvedAnnotations?: Record<string, string>;
  resolvedFields?: Record<string, string>;
  startsAt: string;
  updatedAt: string;
  generatorURL: string;
  silencedBy: string[];
  inhibitedBy: string[];
  receivers: string[];
}

export type SeverityCounts = Record<string, number>;

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

export type AlertRefMode = 'raw' | 'resolved' | 'both';

export interface AlertFieldDisplay {
  text: string;
  mode: AlertRefMode;
  raw?: string;
  resolved?: string;
}

export interface AlertGroup {
  key: string;
  parts: AlertFieldDisplay[];
  alerts: Alert[];
}

export interface DisplayConfig {
  visible_labels: string[];
  visible_annotations: string[];
  subtitle_annotations: string[];
  group_by: string[];
  group_by_override_key_mode: 'raw' | 'display';
  group_by_overrides: Record<string, string[]>;
  sort_by: SortCriterion[];
}

export const alerts = writable<Alert[]>([]);
export const severityCounts = writable<SeverityCounts>(emptySeverityCounts());
export const displayConfig = writable<DisplayConfig>({
  visible_labels: ['alertname', 'severity', 'cluster', 'namespace'],
  visible_annotations: ['summary'],
  subtitle_annotations: ['summary', 'description'],
  group_by: ['cluster'],
  group_by_override_key_mode: 'display',
  group_by_overrides: {},
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

export interface OnCallUser {
  name: string;
  email: string;
}

export interface OnCallStatus {
  source: string;
  scheduleID: string;
  scheduleName: string;
  teamName?: string;
  users: OnCallUser[];
  lastUpdated: string;
}

export const verbose = writable(false);
export const loading = writable(true);
export const error = writable<string | null>(null);
export const sourcesHealth = writable<SourceHealth[]>([]);
export const onCallStatus = writable<OnCallStatus[]>([]);
// Set of "source:id" keys that have appeared since the user last acknowledged them.
export const newAlertKeys = writable<Set<string>>(new Set());
// Set of "source:id" keys that are briefly kept visible after resolution.
export const resolvedAlertKeys = writable<Set<string>>(new Set());
let hasLoadedOnce = false;
const RESOLVED_FLASH_MS = 30000;
const transientResolvedAlerts = new Map<string, Alert>();
const resolvedAlertTimers = new Map<string, ReturnType<typeof setTimeout>>();

interface AlertsUpdatedDiff {
  new?: Alert[];
  resolved?: Alert[];
  changed?: Alert[];
}

function healthBySource(entries: SourceHealth[]): Map<string, SourceHealth> {
  return new Map(entries.map(entry => [entry.source, entry]));
}

function logHealthFailures(previousEntries: SourceHealth[], nextEntries: SourceHealth[]): void {
  const previous = healthBySource(previousEntries);
  for (const entry of nextEntries) {
    if (entry.ok) continue;
    const prior = previous.get(entry.source);
    const changed = !prior || prior.ok || prior.lastError !== entry.lastError || prior.consecFails !== entry.consecFails;
    if (!changed) continue;
    console.error(`[provider:${entry.source}] ${entry.lastError || 'poll failed'} (consecutive failures: ${entry.consecFails})`);
  }
}

export async function refreshAlerts(): Promise<void> {
  try {
    if (!isWails()) {
      // No backend — show empty state instead of hanging/crashing.
      alerts.set([]);
      severityCounts.set(emptySeverityCounts());
      onCallStatus.set([]);
      error.set('Dev mode: no Wails backend connected');
      return;
    }
    const [alertList, counts, health, onCall] = await Promise.all([
      GetAlerts(),
      GetSeverityCounts(),
      GetSourcesHealth(),
      GetOnCallStatus(),
    ]);
    const incoming = alertList || [];
    const currentHealth = health || [];

    // Track newly appeared alerts until the user acknowledges them.
    const prev = get(alerts);
    const previousHealthEntries = get(sourcesHealth);
    const prevHealth = healthBySource(previousHealthEntries);
    const nextHealth = healthBySource(currentHealth);
    const incomingKeys = new Set(incoming.map(a => a.source + ':' + a.id));
    const prevKeys = new Set(prev.map(a => a.source + ':' + a.id));
    const unseenKeys = new Set<string>();
    const baselineSources = new Set<string>();

    for (const [source, next] of nextHealth) {
      const previous = prevHealth.get(source);
      if (next.ok && (!previous || !previous.ok)) {
        baselineSources.add(source);
      }
    }

    for (const key of get(newAlertKeys)) {
      if (incomingKeys.has(key)) unseenKeys.add(key);
    }

    if (prev.length > 0 || hasLoadedOnce) {
      for (const alert of incoming) {
        const key = alert.source + ':' + alert.id;
        if (!prevKeys.has(key)) unseenKeys.add(key);
        if (baselineSources.has(alert.source)) unseenKeys.delete(key);
      }
    }

    for (const key of incomingKeys) {
      clearResolvedFlash(key);
    }

    newAlertKeys.set(unseenKeys);
    hasLoadedOnce = true;

    alerts.set(mergeVisibleAlerts(incoming));
    severityCounts.set(counts || emptySeverityCounts());
    sourcesHealth.set(currentHealth);
    logHealthFailures(previousHealthEntries, currentHealth);
    onCallStatus.set(onCall || []);
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
        group_by_override_key_mode: cfg.group_by_override_key_mode === 'raw' ? 'raw' : 'display',
        group_by_overrides: cfg.group_by_overrides ?? {},
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

export async function loadSeverityConfig(): Promise<void> {
  if (!isWails()) return;
  try {
    const cfg = await GetSeverityConfig();
    setSeverityConfig(cfg ?? undefined);
    severityCounts.update(current => ({ ...emptySeverityCounts(), ...(current ?? {}) }));
  } catch (_) {
    setSeverityConfig();
    severityCounts.set(emptySeverityCounts());
  }
}

export function acknowledgeAlert(alertKey: string): void {
  newAlertKeys.update(keys => {
    if (!keys.has(alertKey)) return keys;
    const next = new Set(keys);
    next.delete(alertKey);
    return next;
  });
}

export function acknowledgeAllAlerts(): void {
  newAlertKeys.set(new Set());
}

export function acknowledgeResolvedAlert(alertKey: string): void {
  clearResolvedFlash(alertKey);
  alerts.update(current => current.filter(item => item.source + ':' + item.id !== alertKey));
}

export function acknowledgeAllResolvedAlerts(): void {
  const keys = [...transientResolvedAlerts.keys()];
  for (const key of keys) {
    clearResolvedFlash(key);
  }
  alerts.update(current => current.filter(item => !keys.includes(item.source + ':' + item.id)));
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
  EventsOn('alerts:updated', (diff?: AlertsUpdatedDiff) => {
    handleResolvedAlerts(diff);
    refreshAlerts();
  });
  EventsOn('config:reloaded', () => {
    loadSeverityConfig();
    loadDisplayConfig();
    refreshAlerts();
  });
}

// Derived: alerts grouped by the display config's group_by field references
export const groupedAlerts = derived(
  [alerts, activeSortCriteria, activeGroupBy, displayConfig, severityConfig],
  ([$alerts, $criteria, $groupBy, $config, _severityConfig]) => {
    const baseGroupBy = $groupBy || [];
    const overrideKeyMode = $config.group_by_override_key_mode ?? 'display';
    const groupByOverrides = $config.group_by_overrides ?? {};
    if (baseGroupBy.length === 0) {
      return [{ key: 'ungrouped', parts: [], alerts: [...$alerts].sort(sortByCriteria($criteria)) }] as AlertGroup[];
    }

    const groups = new Map<string, AlertGroup>();
    for (const alert of $alerts) {
      const baseKey = groupKeyForOverride(alert, baseGroupBy, overrideKeyMode);
      const effectiveGroupBy = [...baseGroupBy, ...(groupByOverrides[baseKey] ?? [])];
      const parts = resolveGroupParts(alert, effectiveGroupBy);
      const key = parts.map(part => part.text).join('/') || 'other';
      const group = groups.get(key);
      if (group) {
        group.alerts.push(alert);
        continue;
      }
      groups.set(key, { key, parts, alerts: [alert] });
    }

    const sorted = [...groups.values()];
    for (const group of sorted) {
      group.alerts.sort(sortByCriteria($criteria));
    }
    sorted.sort((a, b) => {
      const severityDiff = highestSeverityInGroup(a) - highestSeverityInGroup(b);
      if (severityDiff !== 0) return severityDiff;
      return a.key.localeCompare(b.key);
    });
    return sorted;
  }
);

function resolveGroupParts(alert: Alert, groupBy: string[]): AlertFieldDisplay[] {
  return groupBy
    .map(ref => resolveAlertFieldDisplay(alert, ref))
    .filter((item): item is AlertFieldDisplay => !!item);
}

function groupKeyFromParts(parts: AlertFieldDisplay[]): string {
  return parts.map(part => part.text).join('/') || 'other';
}

function groupKeyForOverride(alert: Alert, groupBy: string[], mode: 'raw' | 'display'): string {
  if (mode === 'display') {
    return groupKeyFromParts(resolveGroupParts(alert, groupBy));
  }

  return groupBy
    .map(ref => resolveAlertFieldValueForOverride(alert, ref))
    .filter((value): value is string => !!value)
    .join('/') || 'other';
}

function resolveAlertFieldValueForOverride(alert: Alert, ref: string): string | undefined {
  const parsed = parseAlertRef(ref);
  const { raw, resolved } = getAlertFieldValues(alert, parsed.ref);
  return raw ?? resolved;
}

function highestSeverityInGroup(group: AlertGroup): number {
  return group.alerts.reduce((highest, alert) => Math.min(highest, severityOrder(alert.severity)), severityOrder('unknown'));
}

function mergeVisibleAlerts(incoming: Alert[]): Alert[] {
  const visible = [...incoming];
  const incomingKeys = new Set(incoming.map(alert => alert.source + ':' + alert.id));

  for (const [key, alert] of transientResolvedAlerts) {
    if (!incomingKeys.has(key)) {
      visible.push(alert);
    }
  }

  return visible;
}

function handleResolvedAlerts(diff?: AlertsUpdatedDiff): void {
  if (!diff?.resolved?.length) return;

  for (const alert of diff.resolved) {
    const key = alert.source + ':' + alert.id;
    const resolvedAlert: Alert = {
      ...alert,
      state: 'resolved',
      updatedAt: new Date().toISOString(),
    };

    transientResolvedAlerts.set(key, resolvedAlert);

    newAlertKeys.update(keys => {
      if (!keys.has(key)) return keys;
      const next = new Set(keys);
      next.delete(key);
      return next;
    });

    resolvedAlertKeys.update(keys => {
      const next = new Set(keys);
      next.add(key);
      return next;
    });

    const existingTimer = resolvedAlertTimers.get(key);
    if (existingTimer) clearTimeout(existingTimer);
    resolvedAlertTimers.set(key, setTimeout(() => {
      clearResolvedFlash(key);
      alerts.update(current => current.filter(item => item.source + ':' + item.id !== key));
    }, RESOLVED_FLASH_MS));
  }
}

function clearResolvedFlash(key: string): void {
  transientResolvedAlerts.delete(key);

  const timer = resolvedAlertTimers.get(key);
  if (timer) {
    clearTimeout(timer);
    resolvedAlertTimers.delete(key);
  }

  resolvedAlertKeys.update(keys => {
    if (!keys.has(key)) return keys;
    const next = new Set(keys);
    next.delete(key);
    return next;
  });
}

// Enum orderings used for sorting
const STATE_ORDER: Record<string, number> = {
  firing: 0,
  silenced: 1,
  inhibited: 2,
  resolved: 3,
};

export function resolveAlertField(alert: Alert, ref: string): string | undefined {
  return resolveAlertFieldDisplay(alert, ref)?.text;
}

export function resolveAlertLabel(alert: Alert, name: string): string | undefined {
  return resolveAlertFieldDisplay(alert, `label:${name}`)?.text;
}

export function resolveAlertAnnotation(alert: Alert, name: string): string | undefined {
  return resolveAlertFieldDisplay(alert, `annotation:${name}`)?.text;
}

export function resolveAlertFieldDisplay(alert: Alert, ref: string): AlertFieldDisplay | undefined {
  const parsed = parseAlertRef(ref);
  const { raw, resolved } = getAlertFieldValues(alert, parsed.ref);

  switch (parsed.mode) {
    case 'raw':
      if (raw) return { text: raw, mode: 'raw', raw, resolved };
      if (resolved) return { text: resolved, mode: 'raw', raw, resolved };
      return undefined;
    case 'resolved':
      if (resolved) return { text: resolved, mode: 'resolved', raw, resolved };
      if (raw) return { text: raw, mode: 'resolved', raw, resolved };
      return undefined;
    case 'both':
      if (raw && resolved && raw !== resolved) {
        return { text: `${raw} (${resolved})`, mode: 'both', raw, resolved };
      }
      if (raw) return { text: raw, mode: 'both', raw, resolved };
      if (resolved) return { text: resolved, mode: 'both', raw, resolved };
      return undefined;
  }
}

export function fieldNameFromRef(ref: string): string {
  return parseAlertRef(ref).name;
}

function parseAlertRef(ref: string): { ref: string; mode: AlertRefMode; kind: 'field' | 'label' | 'annotation'; name: string } {
  let baseRef = ref;
  let mode: AlertRefMode = 'both';
  const lastColon = ref.lastIndexOf(':');
  if (lastColon > 0) {
    const suffix = ref.slice(lastColon + 1);
    if (suffix === 'raw' || suffix === 'resolved' || suffix === 'both') {
      baseRef = ref.slice(0, lastColon);
      mode = suffix;
    }
  }

  if (baseRef.startsWith('field:')) {
    return { ref: baseRef, mode, kind: 'field', name: baseRef.slice(6) };
  }
  if (baseRef.startsWith('label:')) {
    return { ref: baseRef, mode, kind: 'label', name: baseRef.slice(6) };
  }
  if (baseRef.startsWith('annotation:')) {
    return { ref: baseRef, mode, kind: 'annotation', name: baseRef.slice(11) };
  }
  return { ref: `label:${baseRef}`, mode, kind: 'label', name: baseRef };
}

function getAlertFieldValues(alert: Alert, ref: string): { raw?: string; resolved?: string } {
  if (ref.startsWith('field:')) {
    const name = ref.slice(6);
    return {
      raw: getRawFieldValue(alert, name),
      resolved: alert.resolvedFields?.[name],
    };
  }
  if (ref.startsWith('label:')) {
    const name = ref.slice(6);
    return {
      raw: alert.labels?.[name],
      resolved: alert.resolvedLabels?.[name],
    };
  }
  const name = ref.slice(11);
  return {
    raw: alert.annotations?.[name],
    resolved: alert.resolvedAnnotations?.[name],
  };
}

function getRawFieldValue(alert: Alert, name: string): string | undefined {
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
