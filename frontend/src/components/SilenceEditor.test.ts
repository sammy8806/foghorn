// @vitest-environment jsdom

import { tick } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SilenceEditor from './SilenceEditor.svelte';
import { alerts, sourceCapabilities } from '../stores/alerts';

const mounted: SilenceEditor[] = [];

beforeEach(() => {
  document.body.innerHTML = '';
  alerts.set([]);
  sourceCapabilities.set({ alertmanager: { supportsSilence: true } });

  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: () => ({ matches: true }),
  });
  vi.spyOn(HTMLElement.prototype, 'offsetParent', 'get').mockImplementation(function () {
    return this.isConnected ? document.body : null;
  });

  Object.defineProperty(window, 'go', {
    configurable: true,
    value: {
      main: {
        App: {
          GetUIConfig: vi.fn().mockResolvedValue({
            default_created_by: '',
            silence_editor: { always_visible_matchers: [], collapse_matchers: true },
          }),
        },
      },
    },
  });
});

afterEach(() => {
  for (const component of mounted.splice(0)) component.$destroy();
  vi.restoreAllMocks();
});

describe('SilenceEditor focus trap', () => {
  it('moves forward from the dialog frame to its first control', async () => {
    const opener = document.createElement('button');
    document.body.append(opener);
    opener.focus();

    const component = new SilenceEditor({
      target: document.body,
      props: { open: true, mode: 'create', seedMatchers: [] },
    });
    mounted.push(component);
    await tick();
    await tick();

    const dialog = document.querySelector<HTMLElement>('[role="dialog"]');
    const close = document.querySelector<HTMLButtonElement>('button[aria-label="Close"]');
    expect(dialog).not.toBeNull();
    expect(close).not.toBeNull();
    expect(document.activeElement).toBe(dialog);

    dialog?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));

    expect(document.activeElement).toBe(close);
  });
});
