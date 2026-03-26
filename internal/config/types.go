package config

import (
	"strings"
	"time"
)

var fieldRefModes = map[string]struct{}{
	"raw":      {},
	"resolved": {},
	"both":     {},
}

type Config struct {
	Sources       []SourceConfig      `yaml:"sources"`
	Severities    SeverityConfig      `yaml:"severities"`
	Display       DisplayConfig       `yaml:"display"`
	Sounds        SoundsConfig        `yaml:"sounds"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Actions       []ActionConfig      `yaml:"actions"`
	Resolvers     []ResolverConfig    `yaml:"resolvers"`
	UI            UIConfig            `yaml:"ui"`
}

type SourceConfig struct {
	Name          string            `yaml:"name"`
	Type          string            `yaml:"type"`
	URL           string            `yaml:"url"`
	Auth          AuthConfig        `yaml:"auth"`
	PollInterval  time.Duration     `yaml:"poll_interval"`
	Filters       []string          `yaml:"filters"`
	SeverityLabel string            `yaml:"severity_label"`
	BetterStack   BetterStackConfig `yaml:"betterstack"`
}

type BetterStackConfig struct {
	OnCallSchedule string `yaml:"on_call_schedule"`
	TeamName       string `yaml:"team_name"`
}

type AuthConfig struct {
	Type     string `yaml:"type"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Token    string `yaml:"token"`
}

// SortCriterion is a single sort field with optional order direction.
type SortCriterion struct {
	Field string `yaml:"field" json:"field"`
	Order string `yaml:"order" json:"order"` // "asc" or "desc"
}

// NormalizedDisplayConfig is what the frontend receives — sort_by is always a
// resolved []SortCriterion regardless of how it was written in the config file.
type NormalizedDisplayConfig struct {
	VisibleLabels          []string            `json:"visible_labels"`
	VisibleAnnotations     []string            `json:"visible_annotations"`
	SubtitleAnnotations    []string            `json:"subtitle_annotations"`
	GroupBy                []string            `json:"group_by"`
	GroupByOverrideKeyMode string              `json:"group_by_override_key_mode"`
	GroupByOverrides       map[string][]string `json:"group_by_overrides"`
	SortBy                 []SortCriterion     `json:"sort_by"`
}

type DisplayConfig struct {
	VisibleLabels          []string            `yaml:"visible_labels" json:"visible_labels"`
	VisibleAnnotations     []string            `yaml:"visible_annotations" json:"visible_annotations"`
	SubtitleAnnotations    []string            `yaml:"subtitle_annotations" json:"subtitle_annotations"`
	GroupBy                []string            `yaml:"group_by" json:"group_by"`
	GroupByOverrideKeyMode string              `yaml:"group_by_override_key_mode" json:"group_by_override_key_mode"`
	GroupByOverrides       map[string][]string `yaml:"group_by_overrides" json:"group_by_overrides"`
	SortBy                 interface{}         `yaml:"sort_by" json:"-"`
}

// Normalize converts the raw DisplayConfig into a NormalizedDisplayConfig
// suitable for sending to the frontend.
func (d *DisplayConfig) Normalize() NormalizedDisplayConfig {
	return NormalizedDisplayConfig{
		VisibleLabels:          d.VisibleLabels,
		VisibleAnnotations:     d.VisibleAnnotations,
		SubtitleAnnotations:    d.SubtitleAnnotations,
		GroupBy:                d.GroupBy,
		GroupByOverrideKeyMode: d.OverrideKeyMode(),
		GroupByOverrides:       d.GroupByOverrides,
		SortBy:                 d.ParsedSortBy(),
	}
}

func (d *DisplayConfig) OverrideKeyMode() string {
	switch strings.ToLower(strings.TrimSpace(d.GroupByOverrideKeyMode)) {
	case "raw":
		return "raw"
	default:
		return "display"
	}
}

// ParsedSortBy normalizes sort_by from either a bare string or a list of
// criterion maps into a []SortCriterion.
func (d *DisplayConfig) ParsedSortBy() []SortCriterion {
	switch v := d.SortBy.(type) {
	case string:
		return sortCriteriaFromString(v)
	case []SortCriterion:
		return normalizeCriteria(v)
	case []interface{}:
		return sortCriteriaFromList(v)
	default:
		// nil or unknown — default to severity sort
		return sortCriteriaFromString("severity")
	}
}

// ResolveFieldRef parses a field reference like "field:severity", "label:cluster",
// or "annotation:team". Bare strings without a prefix are treated as label names.
func ResolveFieldRef(ref string) (kind, name string) {
	ref = stripFieldRefMode(ref)
	if s, ok := strings.CutPrefix(ref, "field:"); ok {
		return "field", s
	}
	if s, ok := strings.CutPrefix(ref, "label:"); ok {
		return "label", s
	}
	if s, ok := strings.CutPrefix(ref, "annotation:"); ok {
		return "annotation", s
	}
	return "label", ref
}

func stripFieldRefMode(ref string) string {
	lastColon := strings.LastIndex(ref, ":")
	if lastColon <= 0 {
		return ref
	}
	mode := ref[lastColon+1:]
	if _, ok := fieldRefModes[mode]; ok {
		return ref[:lastColon]
	}
	return ref
}

// defaultOrder returns the default sort direction for a field reference.
// Time fields default to "desc" (newest first); everything else defaults to "asc".
func defaultOrder(fieldRef string) string {
	_, name := ResolveFieldRef(fieldRef)
	switch name {
	case "startsAt", "updatedAt":
		return "desc"
	default:
		return "asc"
	}
}

func sortCriteriaFromString(s string) []SortCriterion {
	switch s {
	case "severity":
		return []SortCriterion{
			{Field: "field:severity", Order: "asc"},
			{Field: "field:startsAt", Order: "desc"},
		}
	default:
		return []SortCriterion{
			{Field: "label:" + s, Order: "asc"},
		}
	}
}

func normalizeCriteria(in []SortCriterion) []SortCriterion {
	criteria := make([]SortCriterion, 0, len(in))
	for _, item := range in {
		field := strings.TrimSpace(item.Field)
		if field == "" {
			continue
		}
		order := strings.ToLower(strings.TrimSpace(item.Order))
		if order != "asc" && order != "desc" {
			order = defaultOrder(field)
		}
		criteria = append(criteria, SortCriterion{Field: field, Order: order})
	}
	if len(criteria) == 0 {
		return sortCriteriaFromString("severity")
	}
	return criteria
}

func sortCriteriaFromList(list []interface{}) []SortCriterion {
	criteria := make([]SortCriterion, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		field, _ := m["field"].(string)
		if field == "" {
			continue
		}
		order, _ := m["order"].(string)
		order = strings.ToLower(strings.TrimSpace(order))
		if order != "asc" && order != "desc" {
			order = defaultOrder(field)
		}
		criteria = append(criteria, SortCriterion{Field: field, Order: order})
	}
	if len(criteria) == 0 {
		return sortCriteriaFromString("severity")
	}
	return criteria
}

type SoundsConfig struct {
	Enabled  bool                      `yaml:"enabled"`
	Critical *SoundEntry               `yaml:"critical"`
	Warning  *SoundEntry               `yaml:"warning"`
	Info     *SoundEntry               `yaml:"info"`
	Sources  map[string]SoundOverrides `yaml:"sources"`
}

type SoundEntry struct {
	File     string        `yaml:"file"`
	Repeat   int           `yaml:"repeat"`
	Interval time.Duration `yaml:"interval"`
}

type SoundOverrides struct {
	Critical *SoundEntry `yaml:"critical"`
	Warning  *SoundEntry `yaml:"warning"`
	Info     *SoundEntry `yaml:"info"`
}

type NotificationsConfig struct {
	Enabled        bool `yaml:"enabled"`
	OnNew          bool `yaml:"on_new"`
	OnResolved     bool `yaml:"on_resolved"`
	BatchThreshold int  `yaml:"batch_threshold"`
}

type ActionConfig struct {
	Name   string            `yaml:"name"`
	Match  map[string]string `yaml:"match"`
	Action ActionDef         `yaml:"action"`
	Icon   string            `yaml:"icon"`
}

type ActionDef struct {
	Type     string `yaml:"type"`
	Template string `yaml:"template"`
	Command  string `yaml:"command"`
	Terminal bool   `yaml:"terminal"`
}

type ResolverConfig struct {
	Name     string            `yaml:"name"`
	Field    string            `yaml:"field"`
	Command  string            `yaml:"command"`
	Args     []string          `yaml:"args"`
	Env      map[string]string `yaml:"env"`
	Timeout  time.Duration     `yaml:"timeout"`
	CacheTTL time.Duration     `yaml:"cache_ttl"`
}

type UIConfig struct {
	Theme            string `yaml:"theme" json:"theme"`
	PopupWidth       int    `yaml:"popup_width" json:"popup_width"`
	PopupHeight      int    `yaml:"popup_height" json:"popup_height"`
	ShowResolved     bool   `yaml:"show_resolved" json:"show_resolved"`
	ShowSilenced     bool   `yaml:"show_silenced" json:"show_silenced"`
	DefaultCreatedBy string `yaml:"default_created_by" json:"default_created_by"`
}
