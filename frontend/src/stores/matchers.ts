import type { Alert, Matcher } from './alerts';
import { anchoredRegex, parseFieldTerm } from './query';

export function matchesMatcher(value: string, m: Matcher): boolean {
  if (m.isRegex) {
    const re = anchoredRegex(m.value);
    if (!re) return false;
    return m.isEqual ? re.test(value) : !re.test(value);
  }
  return m.isEqual ? value === m.value : value !== m.value;
}

export function matchesAllMatchers(alert: Alert, matchers: Matcher[]): boolean {
  return matchers.every((m) => matchesMatcher((alert.labels && alert.labels[m.name]) ?? '', m));
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\-]/g, '\\$&');
}

export function valuesToRegexMatcher(name: string, values: string[]): Matcher {
  const value = values.map(escapeRegex).join('|');
  return { name, value, isRegex: true, isEqual: true };
}

function splitMatcherBlock(text: string): string[] {
  const chunks: string[] = [];
  let current = '';
  let inQuotes = false;
  let escaped = false;
  for (const ch of text) {
    if (ch === '"' && !escaped) {
      inQuotes = !inQuotes;
      current += ch;
      escaped = false;
      continue;
    }
    escaped = inQuotes && ch === '\\' && !escaped;
    if (!inQuotes && (ch === '\n' || ch === ',')) {
      const trimmed = current.trim();
      if (trimmed) chunks.push(trimmed);
      current = '';
      continue;
    }
    current += ch;
  }
  const trimmed = current.trim();
  if (trimmed) chunks.push(trimmed);
  return chunks;
}

export function parseMatcherBlock(text: string): { matchers: Matcher[]; skipped: number } {
  const chunks = splitMatcherBlock(text);
  const matchers: Matcher[] = [];
  let skipped = 0;
  for (const chunk of chunks) {
    const term = parseFieldTerm(chunk);
    if (!term || term.scope === 'annotation') {
      skipped++;
      continue;
    }
    matchers.push({
      name: term.key,
      value: term.value.replace(/\\"/g, '"'),
      isRegex: term.op === '=~' || term.op === '!~',
      isEqual: term.op === '=' || term.op === '=~',
    });
  }
  return { matchers, skipped };
}

function matcherOp(m: Matcher): string {
  if (m.isRegex && m.isEqual) return '=~';
  if (m.isRegex && !m.isEqual) return '!~';
  if (!m.isRegex && m.isEqual) return '=';
  return '!=';
}

function formatMatcherValue(value: string): string {
  if (!value || /[\s",]/.test(value)) {
    return `"${value.replace(/"/g, '\\"')}"`;
  }
  return value;
}

export function formatMatcherBlock(matchers: Matcher[]): string {
  return matchers
    .filter((m) => m.name.trim() && m.value)
    .map((m) => `${m.name.trim()}${matcherOp(m)}${formatMatcherValue(m.value)}`)
    .join('\n');
}
