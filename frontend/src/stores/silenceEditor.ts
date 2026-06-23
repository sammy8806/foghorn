import { writable } from 'svelte/store';
import type { Alert, SilenceInfo } from './alerts';

// Silence-editor state lives here (not inside AlertCard) so the modal survives
// alert list refreshes, regrouping, resorting, and filter changes. AlertCard
// instances are destroyed/recreated as the list re-renders; a top-level store
// keeps the open modal alive through all of it.
export interface SilenceEditorState {
  open: boolean;
  mode: 'create' | 'edit';
  alert: Alert | null;
  silence: SilenceInfo | null;
}

const closed: SilenceEditorState = {
  open: false,
  mode: 'create',
  alert: null,
  silence: null,
};

export const silenceEditor = writable<SilenceEditorState>({ ...closed });

export function openSilenceCreate(alert: Alert): void {
  silenceEditor.set({ open: true, mode: 'create', alert, silence: null });
}

export function openSilenceEdit(alert: Alert, silence: SilenceInfo): void {
  silenceEditor.set({ open: true, mode: 'edit', alert, silence });
}

export function closeSilenceEditor(): void {
  silenceEditor.set({ ...closed });
}
