package model

import "time"

// Alert is the unified alert type all providers normalize to.
type Alert struct {
	ID                  string            `json:"id"`
	Source              string            `json:"source"`
	SourceType          string            `json:"sourceType"`
	Name                string            `json:"name"`
	Severity            string            `json:"severity"`
	State               string            `json:"state"`
	Labels              map[string]string `json:"labels"`
	Annotations         map[string]string `json:"annotations"`
	ResolvedLabels      map[string]string `json:"resolvedLabels,omitempty"`
	ResolvedAnnotations map[string]string `json:"resolvedAnnotations,omitempty"`
	ResolvedFields      map[string]string `json:"resolvedFields,omitempty"`
	StartsAt            time.Time         `json:"startsAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
	GeneratorURL        string            `json:"generatorURL"`
	SilencedBy          []string          `json:"silencedBy"`
	InhibitedBy         []string          `json:"inhibitedBy"`
	Receivers           []string          `json:"receivers"`
}

// Key returns the deduplication key for this alert.
func (a Alert) Key() string {
	return a.Source + ":" + a.ID
}

// SilenceRequest represents a request to silence alerts.
type SilenceRequest struct {
	Matchers  []Matcher `json:"matchers"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
}

// Matcher is a label matcher for silences.
type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

// ProviderHealth tracks a provider's connection state.
type ProviderHealth struct {
	Connected   bool      `json:"connected"`
	LastSuccess time.Time `json:"lastSuccess"`
	LastError   string    `json:"lastError"`
	ErrorCount  int       `json:"errorCount"`
	AlertCount  int       `json:"alertCount"`
}

// SourceHealth tracks the poll status for a single source as seen by the frontend.
type SourceHealth struct {
	Source      string    `json:"source"`
	OK          bool      `json:"ok"`
	LastPoll    time.Time `json:"lastPoll"`
	LastError   string    `json:"lastError,omitempty"`
	ConsecFails int       `json:"consecFails"`
}

// Diff represents changes between two poll cycles.
type Diff struct {
	New      []Alert `json:"new"`
	Resolved []Alert `json:"resolved"`
	Changed  []Alert `json:"changed"`
}

// SeverityCounts tracks alert counts per severity level.
type SeverityCounts struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}
