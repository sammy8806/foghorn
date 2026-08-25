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

export function initConfigDiagnostics() {
  if (!isWails()) return () => {};

  GetConfigDiagnostics()
    .then(payload => applyConfigDiagnostics(payload as ConfigDiagnosticsPayload))
    .catch(err => console.error('failed to load config diagnostics', err));

  return EventsOn('config:diagnostics', payload => {
    applyConfigDiagnostics(payload as ConfigDiagnosticsPayload);
  });
}

// Keeps the store's module state deterministic between unit tests.
export function resetConfigDiagnosticsForTest() {
  dismissedFingerprint = '';
  configDiagnostics.set(emptyPayload);
}
