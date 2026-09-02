// @vitest-environment jsdom

import { tick } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SelectMenu from './SelectMenu.svelte';

const options = [
  { value: 'alpha', label: 'Alpha' },
  { value: 'beta', label: 'Beta' },
  { value: 'gamma', label: 'Gamma' },
];
const mounted: SelectMenu[] = [];

beforeEach(() => {
  document.body.innerHTML = '';
  Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
    configurable: true,
    value: vi.fn(),
  });
});

afterEach(() => {
  for (const component of mounted.splice(0)) component.$destroy();
  vi.restoreAllMocks();
});

function mount(): { component: SelectMenu; control: HTMLElement } {
  const component = new SelectMenu({
    target: document.body,
    props: { ariaLabel: 'Target source', options, value: 'beta' },
  });
  mounted.push(component);
  const control = document.querySelector<HTMLElement>('[role="combobox"]');
  if (!control) throw new Error('combobox was not rendered');
  return { component, control };
}

function press(control: HTMLElement, key: string): KeyboardEvent {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
  control.dispatchEvent(event);
  return event;
}

describe('SelectMenu', () => {
  it('exposes its current value and active option as a combobox', async () => {
    const { control } = mount();

    expect(control.getAttribute('aria-label')).toBe('Target source');
    expect(control.textContent).toContain('Beta');
    expect(control.getAttribute('aria-expanded')).toBe('false');

    press(control, 'Enter');
    await tick();

    const listbox = document.querySelector<HTMLElement>('[role="listbox"]');
    const activeId = control.getAttribute('aria-activedescendant');
    expect(control.getAttribute('aria-expanded')).toBe('true');
    expect(control.getAttribute('aria-controls')).toBe(listbox?.id);
    expect(activeId).not.toBeNull();
    expect(document.getElementById(activeId ?? '')?.textContent).toContain('Beta');
    expect(document.getElementById(activeId ?? '')?.getAttribute('aria-selected')).toBe('true');
  });

  it('selects with the keyboard and scrolls the active option into view', async () => {
    const { component, control } = mount();
    const changes: string[] = [];
    component.$on('change', (event) => changes.push(event.detail));

    press(control, 'Enter');
    await tick();
    press(control, 'ArrowDown');
    await tick();

    const activeId = control.getAttribute('aria-activedescendant');
    expect(document.getElementById(activeId ?? '')?.textContent).toContain('Gamma');
    expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalled();

    press(control, 'Enter');
    await tick();

    expect(changes).toEqual(['gamma']);
    expect(control.textContent).toContain('Gamma');
    expect(control.getAttribute('aria-expanded')).toBe('false');
    expect(document.activeElement).toBe(control);
  });

  it('dismisses with Escape without changing the value', async () => {
    const { component, control } = mount();
    const changes: string[] = [];
    component.$on('change', (event) => changes.push(event.detail));

    press(control, 'Enter');
    await tick();
    press(control, 'ArrowDown');
    await tick();
    const escape = press(control, 'Escape');
    await tick();

    expect(escape.defaultPrevented).toBe(true);
    expect(changes).toEqual([]);
    expect(control.textContent).toContain('Beta');
    expect(control.getAttribute('aria-expanded')).toBe('false');
  });

  it('commits the active option when tabbing away', async () => {
    const { component, control } = mount();
    const changes: string[] = [];
    component.$on('change', (event) => changes.push(event.detail));

    press(control, 'Enter');
    await tick();
    press(control, 'ArrowUp');
    await tick();
    const tab = press(control, 'Tab');
    await tick();

    expect(tab.defaultPrevented).toBe(false);
    expect(changes).toEqual(['alpha']);
    expect(control.textContent).toContain('Alpha');
    expect(control.getAttribute('aria-expanded')).toBe('false');
  });

  it('dismisses when the pointer moves outside the control', async () => {
    const { control } = mount();

    control.click();
    await tick();
    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    await tick();

    expect(control.getAttribute('aria-expanded')).toBe('false');
  });
});
