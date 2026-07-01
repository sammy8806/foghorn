import { writable } from 'svelte/store';
import type { Alert, Matcher, SilenceInfo } from './alerts';
import type { ParsedQuery } from './query';

// Silence-editor state lives here (not inside AlertCard) so the modal survives
// alert list refreshes, regrouping, resorting, and filter changes. AlertCard
// instances are destroyed/recreated as the list re-renders; a top-level store
// keeps the open modal alive through all of it.
export interface SilenceEditorState {
  open: boolean;
  mode: 'create' | 'edit';
  alert: Alert | null;
  silence: SilenceInfo | null;
  query: ParsedQuery | null; // set only for "silence from search" (alert-less create)
  matchers: Matcher[] | null; // set for alert-less creates with explicit matcher seeds
  source: string | null;
}

const closed: SilenceEditorState = {
  open: false,
  mode: 'create',
  alert: null,
  silence: null,
  query: null,
  matchers: null,
  source: null,
};

export const silenceEditor = writable<SilenceEditorState>({ ...closed });

export function openSilenceCreate(alert: Alert): void {
  silenceEditor.set({ open: true, mode: 'create', alert, silence: null, query: null, matchers: null, source: null });
}

export function openSilenceEdit(alert: Alert, silence: SilenceInfo): void {
  silenceEditor.set({ open: true, mode: 'edit', alert, silence, query: null, matchers: null, source: null });
}

export function openSilenceFromQuery(query: ParsedQuery, source: string | null = null): void {
  silenceEditor.set({ open: true, mode: 'create', alert: null, silence: null, query, matchers: null, source });
}

export function openSilenceFromMatchers(matchers: Matcher[], source: string | null = null): void {
  silenceEditor.set({
    open: true,
    mode: 'create',
    alert: null,
    silence: null,
    query: null,
    matchers: matchers.map((m) => ({ ...m })),
    source,
  });
}

export function closeSilenceEditor(): void {
  silenceEditor.set({ ...closed });
}
