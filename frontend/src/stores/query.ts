import type { Alert, Matcher } from './alerts';

export type FieldOp = '=' | '!=' | '=~' | '!~';

export interface FieldTerm {
  kind: 'field';
  scope: 'label' | 'annotation';
  key: string;
  op: FieldOp;
  value: string;
}

export interface TextTerm {
  kind: 'text';
  value: string;
  negated: boolean;
}

export type QueryTerm = FieldTerm | TextTerm;

export interface ParsedQuery {
  terms: QueryTerm[];
  raw: string;
}

// key = optional "annotation:" scope + label-name grammar; op checks the
// two-char operators before the one-char "=". Value is the rest of the token.
const FIELD_RE = /^(annotation:)?([A-Za-z_][A-Za-z0-9_]*)(!=|=~|!~|=)(.*)$/;

function stripQuotes(s: string): string {
  if (s.length >= 2 && s.startsWith('"') && s.endsWith('"')) {
    return s.slice(1, -1);
  }
  return s;
}

// tokenize splits on whitespace but keeps double-quoted runs together.
// Quotes are retained in the emitted token and stripped per-half later.
function tokenize(text: string): string[] {
  const tokens: string[] = [];
  let cur = '';
  let inQuotes = false;
  for (const ch of text) {
    if (ch === '"') { inQuotes = !inQuotes; cur += ch; continue; }
    if (!inQuotes && /\s/.test(ch)) {
      if (cur) { tokens.push(cur); cur = ''; }
      continue;
    }
    cur += ch;
  }
  if (cur) tokens.push(cur);
  return tokens;
}

export function parseFieldTerm(token: string): FieldTerm | null {
  const m = FIELD_RE.exec(token);
  if (!m) return null;
  const [, annPrefix, key, op, rawVal] = m;
  const value = stripQuotes(rawVal);
  if (!value) return null; // empty value → not a valid field term
  return {
    kind: 'field',
    scope: annPrefix ? 'annotation' : 'label',
    key,
    op: op as FieldOp,
    value,
  };
}

function parseToken(token: string): QueryTerm | null {
  if (!token) return null;
  const field = parseFieldTerm(token);
  if (field) return field;
  let negated = false;
  let body = token;
  if (body.startsWith('-') && body.length > 1) {
    negated = true;
    body = body.slice(1);
  }
  const value = stripQuotes(body);
  if (!value) return null;
  return { kind: 'text', value, negated };
}

export function parseQuery(text: string): ParsedQuery {
  const terms: QueryTerm[] = [];
  for (const token of tokenize(text)) {
    const term = parseToken(token);
    if (term) terms.push(term);
  }
  return { terms, raw: text };
}
