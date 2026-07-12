import { get } from 'svelte/store';
import { activeGroupMode, activeSortMode, verbose } from './alerts';
import { filter, type FilterState } from './filter';

const STORAGE_KEY = 'foghorn.ui-prefs.v1';

interface UIPrefs {
  activeSortMode: string;
  activeGroupMode: string;
  verbose: boolean;
  filter: Pick<FilterState, 'text' | 'severity' | 'source' | 'showSilenced' | 'showAll'>;
}

function readPrefs(): UIPrefs | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<UIPrefs>;
    if (!parsed || typeof parsed !== 'object') return null;
    return {
      activeSortMode: typeof parsed.activeSortMode === 'string' ? parsed.activeSortMode : 'default',
      activeGroupMode: typeof parsed.activeGroupMode === 'string' ? parsed.activeGroupMode : 'default',
      verbose: !!parsed.verbose,
      filter: {
        text: typeof parsed.filter?.text === 'string' ? parsed.filter.text : '',
        severity: typeof parsed.filter?.severity === 'string' ? parsed.filter.severity : 'all',
        source: typeof parsed.filter?.source === 'string' ? parsed.filter.source : 'all',
        showSilenced: typeof parsed.filter?.showSilenced === 'boolean' ? parsed.filter.showSilenced : true,
        showAll: !!parsed.filter?.showAll,
      },
    };
  } catch {
    return null;
  }
}

function writePrefs(prefs: UIPrefs): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
  } catch {
    // Ignore quota/private-mode failures.
  }
}

function snapshotPrefs(): UIPrefs {
  const currentFilter = get(filter);
  return {
    activeSortMode: get(activeSortMode),
    activeGroupMode: get(activeGroupMode),
    verbose: get(verbose),
    filter: {
      text: currentFilter.text,
      severity: currentFilter.severity,
      source: currentFilter.source,
      showSilenced: currentFilter.showSilenced,
      showAll: currentFilter.showAll,
    },
  };
}

/** True when localStorage contains saved UI preferences. */
export function hasPersistedUIPrefs(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) !== null;
  } catch {
    return false;
  }
}

/** Restore persisted UI preferences before first paint when possible. */
export function restoreUIPrefs(): void {
  const prefs = readPrefs();
  if (!prefs) return;
  activeSortMode.set(prefs.activeSortMode);
  activeGroupMode.set(prefs.activeGroupMode);
  verbose.set(prefs.verbose);
  filter.update(current => ({ ...current, ...prefs.filter }));
}

/** Subscribe to preference stores and persist changes. Returns cleanup. */
export function initUIPrefsPersistence(): () => void {
  restoreUIPrefs();

  let timer: ReturnType<typeof setTimeout> | null = null;
  const scheduleSave = () => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => writePrefs(snapshotPrefs()), 120);
  };

  const unsubs = [
    activeSortMode.subscribe(scheduleSave),
    activeGroupMode.subscribe(scheduleSave),
    verbose.subscribe(scheduleSave),
    filter.subscribe(scheduleSave),
  ];

  return () => {
    if (timer) clearTimeout(timer);
    for (const unsub of unsubs) unsub();
  };
}
