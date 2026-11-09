// A fast hover hint for controls that explain why they're disabled. The native
// title tooltip waits about a second and can't be tuned, and WebKit sends no
// mouse events to disabled buttons, so this goes on a wrapper around them.
// Screen readers get the same text through the control's aria-describedby.

export const HINT_DELAY_MS = 150;

export function hint(node: HTMLElement, text: string | null | undefined) {
  let current = text ?? '';
  let timer: ReturnType<typeof setTimeout> | null = null;
  let tip: HTMLDivElement | null = null;

  function place() {
    if (!tip) return;
    const r = node.getBoundingClientRect();
    const t = tip.getBoundingClientRect();
    const margin = 8;
    let left = r.left + r.width / 2 - t.width / 2;
    left = Math.max(margin, Math.min(left, window.innerWidth - t.width - margin));
    let top = r.bottom + 6;
    if (top + t.height > window.innerHeight - margin) top = r.top - t.height - 6;
    tip.style.left = `${Math.round(left)}px`;
    tip.style.top = `${Math.round(Math.max(margin, top))}px`;
  }

  function show() {
    timer = null;
    if (!current || tip) return;
    tip = document.createElement('div');
    tip.className = 'foghorn-hint';
    tip.setAttribute('role', 'tooltip');
    tip.textContent = current;
    document.body.append(tip);
    place();
  }

  function hide() {
    if (timer) clearTimeout(timer);
    timer = null;
    tip?.remove();
    tip = null;
  }

  function enter() {
    if (!current || tip || timer) return;
    timer = setTimeout(show, HINT_DELAY_MS);
  }

  node.addEventListener('mouseenter', enter);
  node.addEventListener('mouseleave', hide);
  node.addEventListener('focusin', enter);
  node.addEventListener('focusout', hide);
  node.addEventListener('mousedown', hide);

  return {
    update(next: string | null | undefined) {
      current = next ?? '';
      if (!current) hide();
      else if (tip) {
        tip.textContent = current;
        place();
      }
    },
    destroy() {
      hide();
      node.removeEventListener('mouseenter', enter);
      node.removeEventListener('mouseleave', hide);
      node.removeEventListener('focusin', enter);
      node.removeEventListener('focusout', hide);
      node.removeEventListener('mousedown', hide);
    },
  };
}
