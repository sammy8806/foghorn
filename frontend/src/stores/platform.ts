import { writable, get } from 'svelte/store';
import { Environment } from '../../wailsjs/runtime/runtime';

export type Platform = 'darwin' | 'windows' | 'linux' | '';

const store = writable<Platform>('');

/**
 * The platform drives window-chrome CSS (traffic-light inset, drag regions,
 * vibrancy) and a couple of behavioural branches. Chrome layout has to be right
 * on the very first paint, so it is seeded synchronously from the user agent
 * and then corrected from the Wails runtime, which is authoritative but only
 * answers once the bridge is up.
 */
export const platform = { subscribe: store.subscribe };

function apply(value: Platform) {
  if (get(store) === value) return;
  document.documentElement.dataset.platform = value;
  store.set(value);
}

/** Best-effort synchronous guess, used only until the runtime answers. */
function guessFromUserAgent(): Platform {
  const ua = navigator.userAgent;
  if (/Mac OS X|Macintosh/.test(ua)) return 'darwin';
  if (/Windows/.test(ua)) return 'windows';
  if (/Linux|X11/.test(ua)) return 'linux';
  return '';
}

/** Call once, before the app mounts, so the first paint already has chrome. */
export function seedPlatform(): void {
  apply(guessFromUserAgent());
}

/** Replace the guess with the runtime's answer. Safe to call more than once. */
export async function syncPlatform(): Promise<void> {
  const environment = await Environment();
  apply((environment.platform || '') as Platform);
}

export function isMacOS(): boolean {
  return get(store) === 'darwin';
}
