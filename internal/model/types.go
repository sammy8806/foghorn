package model

import "time"

// SilenceInfo holds details about an active silence.
type SilenceInfo struct {
	ID        string    `json:"id"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	Matchers  []Matcher `json:"matchers"`
}

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
	Silences            []SilenceInfo     `json:"silences,omitempty"`
	InhibitedBy         []string          `json:"inhibitedBy"`
	Receivers           []string          `json:"receivers"`
}

// Key returns the deduplication key for this alert.
func (a Alert) Key() string {
	return a.Source + ":" + a.ID
}

// SilenceRequest represents a request to create or update a silence.
// When ID is empty, providers create a new silence; when set, they update in place.
type SilenceRequest struct {
	ID        string    `json:"id,omitempty"`
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

type SourceCapabilities struct {
	SupportsSilence bool `json:"supportsSilence"`
}

type OnCallUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type OnCallStatus struct {
	Source       string       `json:"source"`
	ScheduleID   string       `json:"scheduleID"`
	ScheduleName string       `json:"scheduleName"`
	TeamName     string       `json:"teamName,omitempty"`
	Users        []OnCallUser `json:"users"`
	LastUpdated  time.Time    `json:"lastUpdated"`
}

// Diff represents changes between two poll cycles.
type Diff struct {
	New      []Alert `json:"new"`
	Resolved []Alert `json:"resolved"`
	Changed  []Alert `json:"changed"`
}

// SeverityCounts tracks alert counts per severity level.
type SeverityCounts map[string]int

// SeverityBreakdown splits alert counts per severity into active (non-silenced)
// and silenced buckets. An alert is silenced when len(Alert.SilencedBy) > 0.
type SeverityBreakdown struct {
	Active   SeverityCounts `json:"active"`
	Silenced SeverityCounts `json:"silenced"`
}
