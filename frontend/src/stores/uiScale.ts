import { writable } from 'svelte/store';
import { GetUIScale } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { isWails } from './alerts';

export type UIScaleMode = 'fonts' | 'interface';

export interface UIScale {
  factor: number;
  mode: UIScaleMode;
  apply_to_popup: boolean;
}

const defaultScale: UIScale = {
  factor: 1,
  mode: 'fonts',
  apply_to_popup: true,
};

function normalizeScale(input: Partial<UIScale> | null | undefined): UIScale {
  const factor = typeof input?.factor === 'number' && input.factor > 0 ? input.factor : defaultScale.factor;
  const mode = input?.mode === 'interface' ? 'interface' : 'fonts';
  const applyToPopup = typeof input?.apply_to_popup === 'boolean' ? input.apply_to_popup : true;
  return { factor, mode, apply_to_popup: applyToPopup };
}

export const uiScale = writable<UIScale>(defaultScale);

export function initUIScale() {
  if (!isWails()) return () => {};

  GetUIScale()
    .then(scale => uiScale.set(normalizeScale(scale)))
    .catch(err => console.error('failed to load UI scale', err));

  return EventsOn('ui:scale', scale => {
    uiScale.set(normalizeScale(scale));
  });
}
