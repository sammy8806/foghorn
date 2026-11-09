// Package silenceaccess tracks what a source's credentials may do with
// silences: Grafana permission lookups, refused writes and their precedence.
package silenceaccess

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

// Op is the kind of silence write.
type Op int

const (
	OpCreate Op = iota
	OpWrite
)

// RuleUIDLabel is the label Grafana puts on every alert of a managed rule. An
// equality matcher on it makes a silence rule-specific.
const RuleUIDLabel = "__alert_rule_uid__"

// ErrUnsupported marks a Grafana without the permissions endpoint.
var ErrUnsupported = errors.New("permissions endpoint not available")

const (
	actionCreate = "alert.silences:create"
	actionWrite  = "alert.silences:write"
	actionRead   = "alert.silences:read"
)

// Lookup is the raw result of one Grafana permission lookup.
type Lookup struct {
	Permissions   map[string][]string // raw /api/access-control/user/permissions
	RuleFolders   map[string]string   // nil when the rules API was not readable
	FolderParents map[string]string
	FolderTitles  map[string]string
}

// Tracker holds one source's silence access state. Safe for concurrent use.
type Tracker struct {
	mu        sync.Mutex
	grafana   bool
	cfg       config.SilenceCheckConfig
	state     model.SilenceAccess
	started   time.Time // first CheckDue call; baseline when on_start is off
	attempted time.Time // last lookup attempt (success or error)
	forceDue  bool      // a denial asks for a lookup right away
	fp        string    // fingerprint of the last successful lookup
	hasDenial bool
	denialFP  string // fingerprint the denial is held against; "" = next lookup is the baseline
}

// NewTracker creates a tracker for a source type ("grafana" or
// "alertmanager"). cfg is only read for grafana and must have been through
// config validation (all fields set).
func NewTracker(sourceType string, cfg config.SilenceCheckConfig) *Tracker {
	t := &Tracker{grafana: sourceType == "grafana", cfg: cfg}
	t.state.Check = "unchecked"
	if t.grafana {
		t.state.Granularity = cfg.Granularity
		if t.state.Granularity == "" {
			t.state.Granularity = "folder"
		}
	}
	return t
}

func (t *Tracker) onStart() bool  { return t.cfg.OnStart == nil || *t.cfg.OnStart }
func (t *Tracker) onDenied() bool { return t.cfg.OnDenied == nil || *t.cfg.OnDenied }
func (t *Tracker) interval() time.Duration {
	if t.cfg.Interval == nil {
		return config.DefaultSilenceCheckInterval
	}
	return *t.cfg.Interval
}
func (t *Tracker) lookupsEnabled() bool {
	return t.grafana && t.state.Granularity != "off" && (t.onStart() || t.interval() > 0)
}

// Granularity reports the configured granularity ("" for Alertmanager).
func (t *Tracker) Granularity() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state.Granularity
}

// CheckDue reports whether a permission lookup should run now.
func (t *Tracker) CheckDue(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.lookupsEnabled() || t.state.Check == "unsupported" {
		return false
	}
	if t.started.IsZero() {
		t.started = now
	}
	if t.forceDue {
		return true
	}
	iv := t.interval()
	if t.attempted.IsZero() {
		if t.onStart() {
			return true
		}
		return iv > 0 && now.Sub(t.started) >= iv
	}
	return iv > 0 && now.Sub(t.attempted) >= iv
}

// ApplyLookup records a successful lookup.
func (t *Tracker) ApplyLookup(l Lookup, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.attempted, t.forceDue = now, false
	createAll, createFolders := scopes(l.Permissions[actionCreate])
	writeAll, writeFolders := scopes(l.Permissions[actionWrite])
	_, canRead := l.Permissions[actionRead]
	s := &t.state
	s.Check, s.CheckError, s.CheckedAt = "ok", "", now
	s.CanCreateAll, s.CreateFolders = createAll, createFolders
	s.CanWriteAll, s.WriteFolders = writeAll, writeFolders
	s.CanRead = canRead
	s.RuleFolders = copyMap(l.RuleFolders)
	s.FolderParents = copyMap(l.FolderParents)
	s.FolderTitles = copyMap(l.FolderTitles)

	t.fp = fingerprint(l.Permissions)
	if t.hasDenial {
		switch {
		case t.denialFP == "":
			t.denialFP = t.fp
		case t.denialFP != t.fp:
			t.hasDenial, t.denialFP = false, ""
			s.Denied = model.SilenceDenials{}
		}
	}
}

// ApplyLookupError records a failed lookup. Decisions fall back to unknown
// (enabled), except where a denial says otherwise.
func (t *Tracker) ApplyLookupError(err error, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.attempted, t.forceDue = now, false
	s := &t.state
	if errors.Is(err, ErrUnsupported) {
		s.Check, s.CheckError = "unsupported", ""
	} else {
		s.Check, s.CheckError = "error", err.Error()
	}
	s.CanCreateAll, s.CanWriteAll, s.CanRead = false, false, false
	s.CreateFolders, s.WriteFolders = nil, nil
	s.RuleFolders, s.FolderParents, s.FolderTitles = map[string]string{}, map[string]string{}, map[string]string{}
	t.fp = ""
}

// RecordDenied marks a refused silence write as narrowly as its matchers allow.
func (t *Tracker) RecordDenied(op Op, matchers []model.Matcher, status int, message string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.grafana && !t.onDenied() {
		return
	}
	d := &t.state.Denied
	scope, folder := "source", ""
	if t.grafana {
		if uid, ok := RuleUID(matchers); !ok {
			scope = "general"
		} else if f, known := t.state.RuleFolders[uid]; known {
			scope, folder = "folder", f
		}
	}
	switch {
	case scope == "source" && op == OpCreate:
		d.CreateSource = true
	case scope == "source":
		d.WriteSource = true
	case scope == "general" && op == OpCreate:
		d.CreateGeneral = true
	case scope == "general":
		d.WriteGeneral = true
	case op == OpCreate:
		d.CreateFolders = addSorted(d.CreateFolders, folder)
	default:
		d.WriteFolders = addSorted(d.WriteFolders, folder)
	}
	d.Status, d.Message, d.At = status, message, now
	if !t.hasDenial {
		t.hasDenial = true
		if t.state.Check == "ok" {
			t.denialFP = t.fp
		} else {
			t.denialFP = ""
		}
	}
	t.forceDue = true
}

// CanReadSilences is false only when a successful lookup showed no
// alert.silences:read.
func (t *Tracker) CanReadSilences() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !(t.state.Check == "ok" && !t.state.CanRead)
}

// Snapshot returns a deep copy of the state.
func (t *Tracker) Snapshot() model.SilenceAccess {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.state
	s.CreateFolders = slices.Clone(s.CreateFolders)
	s.WriteFolders = slices.Clone(s.WriteFolders)
	s.RuleFolders = copyMap(s.RuleFolders)
	s.FolderParents = copyMap(s.FolderParents)
	s.FolderTitles = copyMap(s.FolderTitles)
	s.Denied.CreateFolders = slices.Clone(s.Denied.CreateFolders)
	s.Denied.WriteFolders = slices.Clone(s.Denied.WriteFolders)
	return s
}

// RuleUID returns the value of an equality (non-regex) __alert_rule_uid__
// matcher.
func RuleUID(matchers []model.Matcher) (string, bool) {
	for _, m := range matchers {
		if m.Name == RuleUIDLabel && m.IsEqual && !m.IsRegex {
			return m.Value, true
		}
	}
	return "", false
}

func scopes(raw []string) (all bool, folders []string) {
	seen := map[string]bool{}
	for _, s := range raw {
		switch {
		case s == "*" || s == "folders:*" || s == "folders:uid:*":
			all = true
		case strings.HasPrefix(s, "folders:uid:"):
			uid := strings.TrimPrefix(s, "folders:uid:")
			if uid != "" && !seen[uid] {
				seen[uid] = true
				folders = append(folders, uid)
			}
		}
	}
	sort.Strings(folders)
	return all, folders
}

func fingerprint(perms map[string][]string) string {
	var entries []string
	for action, scopes := range perms {
		if !strings.HasPrefix(action, "alert.silences:") {
			continue
		}
		for _, s := range scopes {
			entries = append(entries, action+"\x00"+s)
		}
		if len(scopes) == 0 {
			entries = append(entries, action)
		}
	}
	sort.Strings(entries)
	entries = slices.Compact(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(sum[:])
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return maps.Clone(m)
}

func addSorted(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	list = append(list, v)
	sort.Strings(list)
	return list
}
