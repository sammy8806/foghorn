import { derived, writable } from 'svelte/store';
import { GetConfigDiagnostics } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { isWails } from './alerts';

export interface ConfigDiagnostic {
  field: string;
  message: string;
  dropped: boolean;
}

export interface ConfigDiagnosticsPayload {
  path: string;
  fingerprint: string;
  items: ConfigDiagnostic[];
}

const emptyPayload: ConfigDiagnosticsPayload = {
  path: '',
  fingerprint: '',
  items: [],
};

let dismissedFingerprint = '';

export const configDiagnostics = writable<ConfigDiagnosticsPayload>(emptyPayload);
export const visibleConfigDiagnostics = derived(configDiagnostics, payload =>
  payload.items.length > 0 && payload.fingerprint !== dismissedFingerprint ? payload : null,
);

export function applyConfigDiagnostics(input: ConfigDiagnosticsPayload | null | undefined) {
  const items = Array.isArray(input?.items) ? input!.items : [];
  configDiagnostics.set({
    path: typeof input?.path === 'string' ? input.path : '',
    fingerprint: typeof input?.fingerprint === 'string' ? input.fingerprint : '',
    items,
  });
}

export function dismissConfigDiagnostics() {
  configDiagnostics.update(payload => {
    dismissedFingerprint = payload.fingerprint;
    return payload;
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
  dismissedFingerprint = '';
  configDiagnostics.set(emptyPayload);
}
