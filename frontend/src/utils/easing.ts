/**
 * A CSS `cubic-bezier(x1, y1, x2, y2)` as a JS easing function.
 *
 * Lets a Svelte transition run the exact curve a stylesheet declares, rather
 * than an approximation picked from svelte/easing by eye — the silence dialog
 * animates through transitions in both directions (a CSS animation can't run on
 * a node Svelte is in the middle of removing) but still has to match the curve
 * the rest of the app's chrome uses.
 *
 * Solves x(u) = t for u by Newton-Raphson, falling back to bisection when the
 * derivative is too flat for Newton to converge.
 */
export function cubicBezier(x1: number, y1: number, x2: number, y2: number): (t: number) => number {
  const a = (p1: number, p2: number) => 1 - 3 * p2 + 3 * p1;
  const b = (p1: number, p2: number) => 3 * p2 - 6 * p1;
  const c = (p1: number) => 3 * p1;

  const bezier = (u: number, p1: number, p2: number) => ((a(p1, p2) * u + b(p1, p2)) * u + c(p1)) * u;
  const slope = (u: number, p1: number, p2: number) => 3 * a(p1, p2) * u * u + 2 * b(p1, p2) * u + c(p1);

  return (t: number): number => {
    if (t <= 0) return 0;
    if (t >= 1) return 1;

    let u = t;
    for (let i = 0; i < 8; i++) {
      const error = bezier(u, x1, x2) - t;
      if (Math.abs(error) < 1e-6) return bezier(u, y1, y2);
      const d = slope(u, x1, x2);
      if (Math.abs(d) < 1e-6) break;
      u -= error / d;
    }

    let lo = 0;
    let hi = 1;
    u = t;
    for (let i = 0; i < 20 && Math.abs(bezier(u, x1, x2) - t) > 1e-6; i++) {
      if (bezier(u, x1, x2) < t) lo = u;
      else hi = u;
      u = (lo + hi) / 2;
    }
    return bezier(u, y1, y2);
  };
}
