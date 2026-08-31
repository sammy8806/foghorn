/**
 * Decides whether a popup anchored to `trigger` should open upwards.
 *
 * Measures against the nearest scrollable ancestor rather than the viewport:
 * these popups live inside the silence dialog's scrolling body, so "fits on
 * screen" is the wrong question — a menu can sit well inside the window and
 * still be clipped by the body it is scrolling in.
 */
export function shouldDropUp(trigger: HTMLElement, menuHeight: number, gap = 4): boolean {
  const { top, bottom } = clipBounds(trigger);
  const fitsBelow = trigger.getBoundingClientRect().bottom + gap + menuHeight <= bottom;
  const fitsAbove = trigger.getBoundingClientRect().top - gap - menuHeight >= top;
  return !fitsBelow && fitsAbove;
}

function clipBounds(el: HTMLElement): { top: number; bottom: number } {
  for (let p = el.parentElement; p; p = p.parentElement) {
    const overflowY = getComputedStyle(p).overflowY;
    if (overflowY === 'auto' || overflowY === 'scroll') {
      const r = p.getBoundingClientRect();
      return { top: r.top, bottom: r.bottom };
    }
  }
  return { top: 0, bottom: window.innerHeight };
}
