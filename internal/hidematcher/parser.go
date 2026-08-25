// Package hidematcher parses and evaluates the matcher syntax shared by
// config validation and the hide engine.
package hidematcher

import (
	"fmt"
	"regexp"
	"strings"
)

// Matcher is one compiled hide-rule matcher.
type Matcher struct {
	Field    string
	Operator string
	Value    string
	regex    *regexp.Regexp
}

// Parse parses strings like `name=value`, `name!=value`, `name=~regex`, or
// `name!~regex`. Values may be optionally double-quoted.
func Parse(raw string) (Matcher, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Matcher{}, fmt.Errorf("empty matcher")
	}
	field, operator, value, ok := split(raw)
	if !ok {
		return Matcher{}, fmt.Errorf("missing operator (=, !=, =~, !~)")
	}
	field = strings.TrimSpace(field)
	if field == "" {
		return Matcher{}, fmt.Errorf("missing field name")
	}
	value = unquote(strings.TrimSpace(value))
	matcher := Matcher{Field: field, Operator: operator, Value: value}
	if operator == "=~" || operator == "!~" {
		compiled, err := regexp.Compile(value)
		if err != nil {
			return Matcher{}, fmt.Errorf("invalid regex %q: %w", value, err)
		}
		matcher.regex = compiled
	}
	return matcher, nil
}

// MatchesValue reports whether value satisfies the matcher's underlying
// equality or regular expression, without applying positive/negative polarity.
func (m Matcher) MatchesValue(value string) bool {
	switch m.Operator {
	case "=", "!=":
		return value == m.Value
	case "=~", "!~":
		return m.regex.MatchString(value)
	default:
		return false
	}
}

// Positive reports whether a matching value makes the whole matcher true.
func (m Matcher) Positive() bool {
	return m.Operator == "=" || m.Operator == "=~"
}

func split(raw string) (field, operator, value string, ok bool) {
	bestStart := -1
	bestOperator := ""
	for _, candidate := range []string{"!=", "!~", "=~", "="} {
		index := strings.Index(raw, candidate)
		if index < 0 {
			continue
		}
		if bestStart == -1 || index < bestStart || index == bestStart && len(candidate) > len(bestOperator) {
			bestStart = index
			bestOperator = candidate
		}
	}
	if bestStart < 0 {
		return "", "", "", false
	}
	return raw[:bestStart], bestOperator, raw[bestStart+len(bestOperator):], true
}

func unquote(value string) string {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
