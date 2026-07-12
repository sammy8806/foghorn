import { writable } from 'svelte/store';
import { GetUIConfig } from '../../wailsjs/go/main/App';
import { EventsOn, WindowSetDarkTheme, WindowSetLightTheme, WindowSetSystemDefaultTheme } from '../../wailsjs/runtime/runtime';
import { isWails } from './alerts';

export type ThemeSetting = 'system' | 'light' | 'dark';

export const themeSetting = writable<ThemeSetting>('system');

function normalizeTheme(value: string | undefined): ThemeSetting {
  if (value === 'light' || value === 'dark') return value;
  return 'system';
}

function applyWindowTheme(setting: ThemeSetting): void {
  if (!isWails()) return;
  switch (setting) {
    case 'light':
      WindowSetLightTheme();
      break;
    case 'dark':
      WindowSetDarkTheme();
      break;
    default:
      WindowSetSystemDefaultTheme();
      break;
  }
}

function applyDocumentTheme(setting: ThemeSetting): void {
  document.documentElement.dataset.theme = setting;
}

export function applyTheme(setting: ThemeSetting): void {
  themeSetting.set(setting);
  applyWindowTheme(setting);
  applyDocumentTheme(setting);
}

export async function loadThemeFromConfig(): Promise<void> {
  if (!isWails()) {
    applyTheme('dark');
    return;
  }
  try {
    const uiConfig = await GetUIConfig();
    applyTheme(normalizeTheme(uiConfig.theme));
  } catch (err) {
    console.error('failed to load UI theme', err);
    applyTheme('system');
  }
}

export function initTheme(): () => void {
  void loadThemeFromConfig();

  if (!isWails()) return () => {};

  return EventsOn('config:reloaded', () => {
    void loadThemeFromConfig();
  });
}
