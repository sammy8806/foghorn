// @vitest-environment jsdom

import { tick } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AlertCard from './AlertCard.svelte';
import { sourceCapabilities } from '../stores/alerts';
import type { Alert, DisplayConfig, SilenceAccess } from '../stores/alerts';

const mounted: AlertCard[] = [];

function access(over: Partial<SilenceAccess> = {}): SilenceAccess {
  return {
    granularity: 'folder', check: 'ok', checkedAt: '', canCreateAll: false, canWriteAll: false, canRead: true,
    createFolders: [], writeFolders: [], ruleFolders: { r1: 'f1' }, folderParents: {}, folderTitles: {},
    denied: { createSource: false, writeSource: false, createGeneral: false, writeGeneral: false, createFolders: [], writeFolders: [], status: 0, message: '', at: '' },
    ...over,
  };
}

function alert(over: Partial<Alert> = {}): Alert {
  return {
    id: 'a1', source: 'g', sourceType: 'grafana', name: 'Down', severity: 'critical', state: 'active',
    labels: { alertname: 'Down', __alert_rule_uid__: 'r1' }, annotations: {},
    startsAt: '2026-11-09T10:00:00Z', updatedAt: '2026-11-09T10:00:00Z', generatorURL: '',
    silencedBy: [], inhibitedBy: [], receivers: [],
    ...over,
  };
}

async function mount(a: Alert): Promise<void> {
  const c = new AlertCard({ target: document.body, props: { alert: a, config: {} as DisplayConfig } });
  mounted.push(c);
  await tick();
  document.querySelector<HTMLElement>('.alert-header')?.click();
  await tick();
}

function button(text: string): HTMLButtonElement {
  const b = [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === text);
  if (!b) throw new Error(`no button ${text}: ${document.body.innerHTML.slice(0, 500)}`);
  return b as HTMLButtonElement;
}

beforeEach(() => {
  document.body.innerHTML = '';
});

afterEach(() => {
  for (const c of mounted.splice(0)) c.$destroy();
  vi.restoreAllMocks();
});

describe('AlertCard silence permissions', () => {
  it('silence button is disabled with the hint when the token cannot create', async () => {
    sourceCapabilities.set({ g: { supportsSilence: true, silenceAccess: access({ granularity: 'source' }) } });
    await mount(alert());
    const b = button('Silence…');
    expect(b.disabled).toBe(true);
    const described = document.getElementById(b.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toBe("g: token can't create silences (needs alert.silences:create)");
    expect(b.querySelector('.permission-warn')).not.toBeNull();
  });

  it('silence button stays enabled when the check failed', async () => {
    sourceCapabilities.set({ g: { supportsSilence: true, silenceAccess: access({ check: 'error' }) } });
    await mount(alert());
    expect(button('Silence…').disabled).toBe(false);
    expect(document.querySelector('.permission-warn')).toBeNull();
  });

  it("edit and expire follow the silence's own matchers", async () => {
    sourceCapabilities.set({ g: { supportsSilence: true, silenceAccess: access({ createFolders: ['f1'], writeFolders: ['f1'] }) } });
    await mount(alert({
      silencedBy: ['s1'],
      silences: [{ id: 's1', createdBy: 'x', comment: '', startsAt: '', endsAt: '2026-11-09T12:00:00Z',
        matchers: [{ name: 'alertname', value: 'Down', isRegex: false, isEqual: true }] } as any],
    }));
    expect(button('Edit').disabled).toBe(true);
    expect(button('Expire now').disabled).toBe(true);
    expect(document.getElementById(button('Edit').getAttribute('aria-describedby') ?? '')?.textContent).toContain('single alert rules');
  });

  it('silenced badge explains hidden details', async () => {
    sourceCapabilities.set({ g: { supportsSilence: true, silenceAccess: access({ canRead: false }) } });
    await mount(alert({ silencedBy: ['s1'] }));
    expect(document.querySelector<HTMLElement>('.badge-silenced')?.title).toContain(
      "silence details hidden: token can't read silences (needs alert.silences:read)",
    );
  });
});
