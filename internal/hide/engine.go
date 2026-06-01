// Package hide implements rule-based suppression of alerts from the default
// UI view. Unlike silences, hide rules are config-driven and time-bounded
// only via min_age — they never reach the alert provider. Hidden alerts
// remain loaded and are surfaced when "Show all" is toggled in the UI.
package hide

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

// Engine evaluates configured hide rules against alerts.
type Engine struct {
	rules []rule
	now   func() time.Time
}

type rule struct {
	name     string
	matchers []matcher
	sources  map[string]struct{} // empty = applies to all sources
	minAge   time.Duration
}

type matcher struct {
	field string // full ref including any mode suffix, e.g. "label:cluster:raw"
	op    string // "=", "!=", "=~", "!~"
	value string
	regex *regexp.Regexp // compiled when op is "=~" or "!~"
}

// New compiles a set of hide rules. Invalid rules are skipped and reported
// via the returned errors slice; the engine still contains the valid rules.
func New(cfgs []config.HideRule) (*Engine, []error) {
	engine := &Engine{
		rules: make([]rule, 0, len(cfgs)),
		now:   time.Now,
	}
	var errs []error
	for i, cfg := range cfgs {
		r, err := compileRule(cfg)
		if err != nil {
			errs = append(errs, fmt.Errorf("hide[%d]: %w", i, err))
			continue
		}
		engine.rules = append(engine.rules, r)
	}
	return engine, errs
}

func compileRule(cfg config.HideRule) (rule, error) {
	r := rule{
		name:   strings.TrimSpace(cfg.Name),
		minAge: cfg.ParsedMinAge,
	}
	if len(cfg.Sources) > 0 {
		r.sources = make(map[string]struct{}, len(cfg.Sources))
		for _, src := range cfg.Sources {
			if s := strings.TrimSpace(src); s != "" {
				r.sources[s] = struct{}{}
			}
		}
	}
	for j, raw := range cfg.Matchers {
		m, err := parseMatcher(raw)
		if err != nil {
			return rule{}, fmt.Errorf("matcher[%d] %q: %w", j, raw, err)
		}
		r.matchers = append(r.matchers, m)
	}
	if len(r.matchers) == 0 {
		return rule{}, fmt.Errorf("at least one matcher is required")
	}
	return r, nil
}

// parseMatcher parses strings like `name=value`, `name!=value`, `name=~regex`,
// or `name!~regex`. Values may be optionally double-quoted (whitespace inside
// the value is preserved either way).
func parseMatcher(raw string) (matcher, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return matcher{}, fmt.Errorf("empty matcher")
	}
	field, op, value, ok := splitMatcher(raw)
	if !ok {
		return matcher{}, fmt.Errorf("missing operator (=, !=, =~, !~)")
	}
	field = strings.TrimSpace(field)
	if field == "" {
		return matcher{}, fmt.Errorf("missing field name")
	}
	value = unquote(strings.TrimSpace(value))
	m := matcher{field: field, op: op, value: value}
	if op == "=~" || op == "!~" {
		re, err := regexp.Compile(value)
		if err != nil {
			return matcher{}, fmt.Errorf("invalid regex %q: %w", value, err)
		}
		m.regex = re
	}
	return m, nil
}

// splitMatcher locates the first matcher operator in raw and splits around
// it. Operators are scanned in order: !=, !~, =~, =. The earliest start
// position wins; ties prefer the longer operator.
func splitMatcher(raw string) (field, op, value string, ok bool) {
	bestStart := -1
	bestOp := ""
	for _, candidate := range []string{"!=", "!~", "=~", "="} {
		idx := strings.Index(raw, candidate)
		if idx < 0 {
			continue
		}
		if bestStart == -1 || idx < bestStart || (idx == bestStart && len(candidate) > len(bestOp)) {
			bestStart = idx
			bestOp = candidate
		}
	}
	if bestStart < 0 {
		return "", "", "", false
	}
	return raw[:bestStart], bestOp, raw[bestStart+len(bestOp):], true
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// Apply evaluates the engine against each alert. The returned slice has the
// same length and ordering as the input; each alert's HiddenBy field carries
// the names of any rules that matched it. Other Alert fields are untouched.
func (e *Engine) Apply(alerts []model.Alert) []model.Alert {
	if e == nil || len(e.rules) == 0 || len(alerts) == 0 {
		return alerts
	}
	now := e.now()
	out := make([]model.Alert, len(alerts))
	for i, alert := range alerts {
		out[i] = alert
		var matched []string
		for _, r := range e.rules {
			if !r.applies(alert, now) {
				continue
			}
			label := r.name
			if label == "" {
				label = "(unnamed)"
			}
			matched = append(matched, label)
		}
		if len(matched) > 0 {
			out[i].HiddenBy = matched
		} else {
			out[i].HiddenBy = nil
		}
	}
	return out
}

func (r rule) applies(alert model.Alert, now time.Time) bool {
	if len(r.sources) > 0 {
		if _, ok := r.sources[alert.Source]; !ok {
			return false
		}
	}
	if r.minAge > 0 {
		if alert.StartsAt.IsZero() || now.Sub(alert.StartsAt) < r.minAge {
			return false
		}
	}
	for _, m := range r.matchers {
		if !m.matches(alert) {
			return false
		}
	}
	return true
}

func (m matcher) matches(alert model.Alert) bool {
	kind, name, mode := config.ResolveFieldRefMode(m.field)
	candidates := lookupValues(alert, kind, name, mode)
	for _, value := range candidates {
		if m.matchValue(value) {
			return m.op == "=" || m.op == "=~"
		}
	}
	// No value matched. For positive ops (=, =~) that's a miss; for negative
	// ops (!=, !~) the absence of any matching value means the rule matches.
	return m.op == "!=" || m.op == "!~"
}

func (m matcher) matchValue(value string) bool {
	switch m.op {
	case "=", "!=":
		return value == m.value
	case "=~", "!~":
		return m.regex.MatchString(value)
	}
	return false
}

// lookupValues returns the candidate values for a field reference under the
// requested mode. Multiple values are returned when mode is "both" and the
// raw and resolved values differ.
func lookupValues(alert model.Alert, kind, name, mode string) []string {
	raw := rawValue(alert, kind, name)
	resolved := resolvedValue(alert, kind, name)
	switch mode {
	case "raw":
		return []string{raw}
	case "resolved":
		if resolved != "" {
			return []string{resolved}
		}
		return []string{raw}
	default: // "both"
		if resolved == "" || resolved == raw {
			return []string{raw}
		}
		return []string{raw, resolved}
	}
}

func rawValue(alert model.Alert, kind, name string) string {
	switch kind {
	case "field":
		switch name {
		case "severity":
			return alert.Severity
		case "source":
			return alert.Source
		case "sourceType":
			return alert.SourceType
		case "name", "alertname":
			return alert.Name
		case "state":
			return alert.State
		case "startsAt":
			if alert.StartsAt.IsZero() {
				return ""
			}
			return alert.StartsAt.Format(time.RFC3339)
		case "updatedAt":
			if alert.UpdatedAt.IsZero() {
				return ""
			}
			return alert.UpdatedAt.Format(time.RFC3339)
		}
		return ""
	case "annotation":
		return alert.Annotations[name]
	default:
		// Labels are the primary lookup. When a label is absent but the name
		// matches a normalized alert field (severity, state, source, etc.),
		// fall back to the field value so bare names work across providers
		// regardless of which label they emit.
		if v, ok := alert.Labels[name]; ok {
			return v
		}
		return rawValue(alert, "field", labelToFieldName(name))
	}
}

// labelToFieldName maps common label names to their normalized field
// equivalents. Returns "" for names without a corresponding field, which
// causes rawValue to return "" (the same as a missing label).
func labelToFieldName(name string) string {
	switch name {
	case "alertname":
		return "name"
	case "severity", "state", "source", "sourceType":
		return name
	}
	return ""
}

func resolvedValue(alert model.Alert, kind, name string) string {
	switch kind {
	case "field":
		return alert.ResolvedFields[name]
	case "annotation":
		return alert.ResolvedAnnotations[name]
	default:
		return alert.ResolvedLabels[name]
	}
}
