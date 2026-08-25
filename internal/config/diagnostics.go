package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Diagnostic is one problem found while loading a config that did not stop the
// config from loading. Field locates the problem the way the YAML does
// (`resolvers[0] "cluster-name"`, `ui.popup_position`) and Message says what is
// wrong and what to do instead, so the UI can render location and fix
// differently.
type Diagnostic struct {
	Field   string `json:"field"`
	Message string `json:"message"`
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
		lines = append(lines, fmt.Sprintf("%s\x00%s\x00%t", item.Field, item.Message, item.Dropped))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\x01")))
	return hex.EncodeToString(sum[:])
}
