// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { hint, HINT_DELAY_MS } from './hint';

afterEach(() => {
  document.body.innerHTML = '';
  vi.useRealTimers();
});

function setup(text: string | null) {
  vi.useFakeTimers();
  const el = document.createElement('span');
  document.body.append(el);
  const action = hint(el, text);
  return { el, action };
}

const tip = () => document.querySelector<HTMLElement>('.foghorn-hint');

describe('hint', () => {
  it('shows quickly on hover and hides on leave', () => {
    const { el } = setup('needs alert.silences:create');
    el.dispatchEvent(new MouseEvent('mouseenter'));
    expect(tip()).toBeNull();
    vi.advanceTimersByTime(HINT_DELAY_MS);
    expect(tip()?.textContent).toBe('needs alert.silences:create');
    expect(tip()?.getAttribute('role')).toBe('tooltip');
    el.dispatchEvent(new MouseEvent('mouseleave'));
    expect(tip()).toBeNull();
  });

  it('leaving before the delay shows nothing', () => {
    const { el } = setup('x');
    el.dispatchEvent(new MouseEvent('mouseenter'));
    el.dispatchEvent(new MouseEvent('mouseleave'));
    vi.advanceTimersByTime(1000);
    expect(tip()).toBeNull();
  });

  it('no text means no hint, and updates apply', () => {
    const { el, action } = setup(null);
    el.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(1000);
    expect(tip()).toBeNull();
    el.dispatchEvent(new MouseEvent('mouseleave'));
    action.update('now set');
    el.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(HINT_DELAY_MS);
    expect(tip()?.textContent).toBe('now set');
    action.destroy();
    expect(tip()).toBeNull();
  });
});
