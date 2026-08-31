package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var diagnosticPositionPattern = regexp.MustCompile(`\[\d+\]`)

// Wrapping that tells you where in the loader we were, which the UI does not
// need: it already says which file failed.
var loadFailureWrapping = []string{"reading config: ", "parsing config: ", "validating config: ", "yaml: "}

var yamlLinePattern = regexp.MustCompile(`^line (\d+): `)

// Diagnostic is one problem found while loading a config that did not stop the
// config from loading. Field locates the problem the way the YAML does
// (`resolvers[0] "cluster-name"`, `ui.popup_position`) and Message says what is
// wrong and what to do instead, so the UI can render location and fix
// differently.
type Diagnostic struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	// Locator is the position to show in place of Field when the problem is not
	// a field at all: a config that would not parse points at "line 3". Empty
	// for field problems, where Field already is the locator.
	Locator string `json:"locator"`
	// Dropped distinguishes an entry that was discarded (true) from a field
	// whose value was replaced by the default (false). The UI renders these as
	// "not loaded" and "using default" respectively.
	Dropped bool `json:"dropped"`
}

// Diagnostics collects every problem from a single load. It is appended to
// rather than returned early, so one pass reports every stale entry instead of
// only the first.
type Diagnostics []Diagnostic

// Drop records a problem whose entry was removed from the config.
func (d *Diagnostics) Drop(field, format string, args ...any) {
	*d = append(*d, Diagnostic{Field: field, Message: fmt.Sprintf(format, args...), Dropped: true})
}

// Substitute records a problem whose value was replaced by the default.
func (d *Diagnostics) Substitute(field, format string, args ...any) {
	*d = append(*d, Diagnostic{Field: field, Message: fmt.Sprintf(format, args...), Dropped: false})
}

// Fingerprint is a stable identity for a set of diagnostics, used by the UI as
// the dismissal key: dismissing hides this exact set, and a reload producing a
// different set shows the banner again. Order-independent so that reordering
// unrelated config entries does not resurrect a dismissed banner.
func (d Diagnostics) Fingerprint() string {
	if len(d) == 0 {
		return ""
	}
	lines := make([]string, 0, len(d))
	for _, item := range d {
		// Positional indices help users find a problem but are not part of its
		// identity. Inserting an unrelated valid entry must not resurrect a
		// dismissed warning for the same named/typed problem.
		stableField := diagnosticPositionPattern.ReplaceAllString(item.Field, "[]")
		// Locator is part of the identity: the line number lives there rather
		// than in Message, so without it "fixed line 3, broke line 9" would
		// stay dismissed.
		lines = append(lines, fmt.Sprintf("%s\x00%s\x00%s\x00%t", stableField, item.Locator, item.Message, item.Dropped))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\x01")))
	return hex.EncodeToString(sum[:])
}

// DescribeLoadFailure splits the error from a config that would not load at all
// into a short locator and the message proper. "parsing config: yaml: line 3:
// mapping values are not allowed in this context" reads well in a log and badly
// in a card, where the wrapping is noise and the position belongs in its own
// column rather than buried mid-sentence.
//
// The locator is empty when the error carries no single position — an
// unreadable file has nothing to point at, and a batch of type errors points at
// several lines at once.
func DescribeLoadFailure(err error) (locator, message string) {
	message = strings.TrimSpace(err.Error())

	// yaml groups type errors under a header with one indented line each.
	if _, rest, found := strings.Cut(message, "unmarshal errors:"); found {
		problems := nonEmptyLines(rest)
		if len(problems) > 1 {
			return "", strings.Join(problems, "; ")
		}
		if len(problems) == 1 {
			message = problems[0]
		}
	}

	message = unwrapLoadFailure(message)

	if match := yamlLinePattern.FindStringSubmatch(message); match != nil {
		return "line " + match[1], strings.TrimSpace(message[len(match[0]):])
	}
	return "", message
}

// The wrapping nests, so peel until nothing more comes off.
func unwrapLoadFailure(message string) string {
	for {
		peeled := message
		for _, wrapper := range loadFailureWrapping {
			peeled = strings.TrimPrefix(peeled, wrapper)
		}
		if peeled == message {
			return message
		}
		message = peeled
	}
}

func nonEmptyLines(input string) []string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
