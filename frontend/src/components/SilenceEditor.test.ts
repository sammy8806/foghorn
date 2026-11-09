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

describe('SilenceEditor live permission check', () => {
  const eq = (name: string, value: string) => ({ name, value, isRegex: false, isEqual: true });
  const access = (over: Record<string, unknown> = {}) => ({
    granularity: 'folder', check: 'ok', checkedAt: '', canCreateAll: false, canWriteAll: false, canRead: true,
    createFolders: ['child'], writeFolders: [], ruleFolders: { r1: 'child' }, folderParents: {}, folderTitles: {},
    denied: { createSource: false, writeSource: false, createGeneral: false, writeGeneral: false, createFolders: [], writeFolders: [], status: 0, message: '', at: '' },
    ...over,
  });

  beforeEach(() => {
    (window as any).go.main.App.GetUIConfig = vi.fn().mockResolvedValue({
      default_created_by: 'tester',
      silence_editor: { always_visible_matchers: [], collapse_matchers: false },
    });
  });

  async function mountEditor(props: Record<string, unknown>): Promise<void> {
    const component = new SilenceEditor({ target: document.body, props: { open: true, mode: 'create', ...props } });
    mounted.push(component);
    for (let i = 0; i < 5; i++) await tick();
    await new Promise((r) => setTimeout(r, 0));
    await tick();
  }

  const submitButton = () =>
    [...document.querySelectorAll<HTMLButtonElement>('button.btn-primary')].find((b) => b.textContent?.includes('Silence'))!;

  it('removing the rule-uid matcher disables submit', async () => {
    sourceCapabilities.set({ g: { supportsSilence: true, silenceAccess: access() as any } });
    await mountEditor({ seedMatchers: [eq('alertname', 'Down'), eq('__alert_rule_uid__', 'r1')], preferredSource: 'g' });
    expect(submitButton().disabled).toBe(false);

    const ruleInput = [...document.querySelectorAll<HTMLInputElement>('input')].find((i) => i.value === '__alert_rule_uid__');
    expect(ruleInput).toBeDefined();
    let row: HTMLElement | null = ruleInput!.parentElement;
    while (row && !row.querySelector('button[aria-label="Remove matcher"]')) row = row.parentElement;
    row!.querySelector<HTMLButtonElement>('button[aria-label="Remove matcher"]')!.click();
    await tick();
    await tick();

    expect(submitButton().disabled).toBe(true);
    expect(document.querySelector('[role="status"].permission-hint')?.textContent).toContain('can only silence single alert rules');
  });

  it('source picker greys sources that cannot take the silence', async () => {
    sourceCapabilities.set({
      a: { supportsSilence: true, silenceAccess: access({ granularity: 'source', createFolders: [] }) as any },
      b: { supportsSilence: true, silenceAccess: access({ canCreateAll: true }) as any },
    });
    await mountEditor({ seedMatchers: [] });
    const control = document.querySelector<HTMLElement>('[role="combobox"][aria-label="Target source"], [role="combobox"]')!;
    expect(control.textContent).toContain('b');
    control.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    await tick();
    const optA = [...document.querySelectorAll<HTMLElement>('[role="option"]')].find((o) => o.textContent?.includes('a ('));
    expect(optA?.getAttribute('aria-disabled')).toBe('true');
  });

  it('unknown access never blocks', async () => {
    sourceCapabilities.set({ g: { supportsSilence: true, silenceAccess: access({ check: 'unchecked', createFolders: [] }) as any } });
    await mountEditor({ seedMatchers: [eq('alertname', 'Down')], preferredSource: 'g' });
    expect(submitButton().disabled).toBe(false);
  });
});
