# Search query grammar

How the **Filter alerts…** search bar parses a query, what each term type
matches, and how a query converts into an Alertmanager silence.

## Syntax at a glance

| Form | Meaning | Example |
|------|---------|---------|
| `word` | Free-text substring match (case-insensitive) | `database` |
| `"quoted phrase"` | Free-text match, keeps spaces/commas together | `"disk usage"` |
| `-word` | Negated free-text term (NOT) | `-firing` |
| `key=value` | Label equals | `severity=critical` |
| `key!=value` | Label not equals | `team!=platform` |
| `key=~regex` | Label matches regex (anchored) | `pod=~worker-.*` |
| `key!~regex` | Label does not match regex | `pod!~worker-.*` |
| `annotation:key<op>value` | Same four operators, scoped to annotations instead of labels | `annotation:runbook=~.*db.*` |

Values can be quoted (`key="some value"`) if they contain spaces or commas.

## Combining terms

Terms are separated by whitespace and are always **implicitly ANDed** — an
alert must match every term in the query. **There is no OR operator** in the
search grammar. To match one of several values on the same label, write the
alternation into a single regex term yourself, e.g. `severity=~critical|warning`.

## What free text matches

A free-text term (quoted or not) is matched as a case-insensitive substring
against a per-alert haystack built from: alert name, source name, every
label value, every annotation value, and every *resolved* label/annotation/
field value (the display values produced by configured visible-entry
mappings). Free text is never turned into a silence matcher, so widening
this haystack has no effect on silence accuracy — it only makes free-text
search find alerts by their display text as well as their raw values.

## Regex anchoring

`key=~regex` and `key!~regex` anchor the pattern automatically as
`^(?:pattern)$`, so `pod=~worker` only matches the literal value `worker`,
not any pod name that merely contains it — use `pod=~worker.*` for a prefix
match. Field matchers (`=`, `!=`, `=~`, `!~`) only ever compare a label's or
annotation's **raw** value, never the resolved display value. An invalid
regex never matches (equality is treated as false, inequality as false too).

## Creating a silence from a search

The silence button next to the search bar converts the current query into
Alertmanager matchers:

- **Label field terms** (`key=value`, `key!=value`, `key=~regex`,
  `key!~regex`) become silence matchers directly, using the same operator.
- **Free-text terms** and **annotation-scoped field terms** can't become
  Alertmanager matchers — Alertmanager only silences on labels — so they're
  dropped. The silence editor lists each dropped term along with why
  (`text` or `annotation`) so you can add an equivalent label matcher by
  hand if one exists.

## Manual matcher entry

Independent of the search bar, the silence editor's matcher list supports
the same four operators (`=`, `!=`, `=~`, `!~`) per row, plus a paste/bulk
mode: paste or type multiple `key=value` lines (comma- or newline-separated,
quote values containing spaces or commas) and they're parsed into matchers
in one step. Lines that don't parse, or that target an annotation, are
skipped and reported as a count.

## Reference

- Query parsing / matching / matcher conversion: `frontend/src/stores/query.ts`
  (`parseQuery`, `matchesQuery`, `queryToMatchers`)
- Manual/paste matcher entry: `frontend/src/components/MatcherEditor.svelte`,
  `frontend/src/stores/matchers.ts`
