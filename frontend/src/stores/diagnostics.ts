import { writable } from 'svelte/store';
import { GetConfigDiagnostics } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { isWails } from './alerts';

export interface ConfigDiagnostic {
  field: string;
  message: string;
  /** Position to show instead of the field when the whole config failed to
   *  parse ("line 3"). Empty for field problems, where field is the locator. */
  locator: string;
  dropped: boolean;
}

/** One line of the config file, shown as evidence beside the problem that
 *  names it. marked is the line the parser stopped on. */
export interface ConfigExcerptLine {
  number: number;
  text: string;
  marked: boolean;
}

export interface ConfigDiagnosticsPayload {
  path: string;
  fingerprint: string;
  items: ConfigDiagnostic[];
  /** Lines around the first item that points at one. Empty otherwise — the
   *  items say what is wrong without it. */
  excerpt: ConfigExcerptLine[];
}

const emptyPayload: ConfigDiagnosticsPayload = {
  path: '',
  fingerprint: '',
  items: [],
  excerpt: [],
};

export const configDiagnostics = writable<ConfigDiagnosticsPayload>(emptyPayload);

export function applyConfigDiagnostics(input: ConfigDiagnosticsPayload | null | undefined) {
  configDiagnostics.set({
    path: typeof input?.path === 'string' ? input.path : '',
    fingerprint: typeof input?.fingerprint === 'string' ? input.fingerprint : '',
    items: Array.isArray(input?.items) ? input!.items : [],
    excerpt: Array.isArray(input?.excerpt) ? input!.excerpt : [],
  });
}

type DiagnosticsFetcher = () => Promise<ConfigDiagnosticsPayload>;
type DiagnosticsSubscriber = (callback: (payload: ConfigDiagnosticsPayload) => void) => () => void;

// Subscribe before fetching, and never allow the startup snapshot to overwrite
// a newer reload event that arrived while the request was in flight.
export function connectConfigDiagnostics(
  fetchDiagnostics: DiagnosticsFetcher,
  subscribe: DiagnosticsSubscriber,
) {
  let eventReceived = false;
  const unlisten = subscribe(payload => {
    eventReceived = true;
    applyConfigDiagnostics(payload);
  });

  Promise.resolve()
    .then(fetchDiagnostics)
    .then(payload => {
      if (!eventReceived) applyConfigDiagnostics(payload);
    })
    .catch(err => console.error('failed to load config diagnostics', err));

  return unlisten;
}

export function initConfigDiagnostics() {
  if (!isWails()) return () => {};

  return connectConfigDiagnostics(
    () => GetConfigDiagnostics() as Promise<ConfigDiagnosticsPayload>,
    callback => EventsOn('config:diagnostics', payload => callback(payload as ConfigDiagnosticsPayload)),
  );
}

// Keeps the store's module state deterministic between unit tests.
export function resetConfigDiagnosticsForTest() {
  configDiagnostics.set(emptyPayload);
}
