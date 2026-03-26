package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

type BetterStack struct {
	cfg    config.SourceConfig
	client *http.Client
	mu     sync.RWMutex
	health model.ProviderHealth
}

func NewBetterStack(cfg config.SourceConfig) *BetterStack {
	return &BetterStack{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (b *BetterStack) Name() string          { return b.cfg.Name }
func (b *BetterStack) Type() string          { return "betterstack" }
func (b *BetterStack) SupportsSilence() bool { return false }

func (b *BetterStack) Health(_ context.Context) model.ProviderHealth {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.health
}

func (b *BetterStack) Fetch(ctx context.Context) ([]model.Alert, error) {
	nextURL := b.endpoint("/api/v3/incidents")
	params := url.Values{}
	params.Set("resolved", "false")
	nextURL += "?" + params.Encode()

	var incidents []bsIncident
	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, err
		}
		b.applyAuth(req)

		resp, err := b.client.Do(req)
		if err != nil {
			b.recordError(err)
			return nil, fmt.Errorf("fetching incidents from %s: %w", b.cfg.Name, err)
		}

		var envelope bsIncidentListResponse
		if resp.StatusCode == http.StatusOK {
			err = json.NewDecoder(resp.Body).Decode(&envelope)
		} else {
			body, _ := io.ReadAll(resp.Body)
			err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		resp.Body.Close()
		if err != nil {
			b.recordError(err)
			return nil, fmt.Errorf("fetching incidents from %s: %w", b.cfg.Name, err)
		}

		incidents = append(incidents, envelope.Data...)
		nextURL = strings.TrimSpace(envelope.Pagination.Next)
	}

	alerts := make([]model.Alert, 0, len(incidents))
	for _, incident := range incidents {
		alerts = append(alerts, incident.toAlert(b.cfg.Name))
	}

	b.mu.Lock()
	b.health = model.ProviderHealth{
		Connected:   true,
		LastSuccess: time.Now(),
		AlertCount:  len(alerts),
	}
	b.mu.Unlock()

	return alerts, nil
}

func (b *BetterStack) FetchOnCall(ctx context.Context) (*model.OnCallStatus, error) {
	scheduleRef := strings.TrimSpace(b.cfg.BetterStack.OnCallSchedule)
	if scheduleRef == "" {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.endpoint("/api/v2/on-calls"), nil)
	if err != nil {
		return nil, err
	}
	if teamName := strings.TrimSpace(b.cfg.BetterStack.TeamName); teamName != "" {
		query := req.URL.Query()
		query.Set("team_name", teamName)
		req.URL.RawQuery = query.Encode()
	}
	b.applyAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching on-call schedule from %s: %w", b.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("betterstack %s on-call returned HTTP %d: %s", b.cfg.Name, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope bsOnCallListResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding on-call schedule from %s: %w", b.cfg.Name, err)
	}

	status, ok := envelope.selectSchedule(scheduleRef)
	if !ok {
		return nil, fmt.Errorf("betterstack %s on-call schedule %q not found", b.cfg.Name, scheduleRef)
	}
	return &status, nil
}

func (b *BetterStack) Silence(_ context.Context, _ model.SilenceRequest) (string, error) {
	return "", fmt.Errorf("silence not supported by Better Stack provider %q", b.cfg.Name)
}

func (b *BetterStack) Unsilence(_ context.Context, _ string) error {
	return fmt.Errorf("unsilence not supported by Better Stack provider %q", b.cfg.Name)
}

func (b *BetterStack) endpoint(path string) string {
	return strings.TrimRight(b.cfg.URL, "/") + path
}

func (b *BetterStack) applyAuth(req *http.Request) {
	applyAuth(req, b.cfg.Auth)
}

func (b *BetterStack) recordError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.health.Connected = false
	b.health.LastError = err.Error()
	b.health.ErrorCount++
}

type bsIncidentListResponse struct {
	Data       []bsIncident `json:"data"`
	Pagination struct {
		Next string `json:"next"`
	} `json:"pagination"`
}

type bsIncident struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name             string                              `json:"name"`
		URL              string                              `json:"url"`
		Cause            string                              `json:"cause"`
		StartedAt        string                              `json:"started_at"`
		AcknowledgedAt   string                              `json:"acknowledged_at"`
		AcknowledgedBy   string                              `json:"acknowledged_by"`
		ResolvedAt       string                              `json:"resolved_at"`
		ResolvedBy       string                              `json:"resolved_by"`
		Status           string                              `json:"status"`
		TeamName         string                              `json:"team_name"`
		ResponseURL      string                              `json:"response_url"`
		ScreenshotURL    string                              `json:"screenshot_url"`
		OriginURL        string                              `json:"origin_url"`
		CriticalAlert    bool                                `json:"critical_alert"`
		Metadata         map[string][]bsIncidentMetadataItem `json:"metadata"`
		EscalationPolicy any                                 `json:"escalation_policy_id"`
	} `json:"attributes"`
	Relationships map[string]struct {
		Data struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	} `json:"relationships"`
}

type bsIncidentMetadataItem struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func (i bsIncident) toAlert(source string) model.Alert {
	startsAt := parseRFC3339(i.Attributes.StartedAt)
	updatedAt := startsAt
	if acknowledged := parseRFC3339(i.Attributes.AcknowledgedAt); !acknowledged.IsZero() {
		updatedAt = acknowledged
	}
	if resolved := parseRFC3339(i.Attributes.ResolvedAt); !resolved.IsZero() {
		updatedAt = resolved
	}

	labels := map[string]string{
		"alertname": source + "/" + i.Attributes.Name,
		"team":      i.Attributes.TeamName,
		"status":    i.Attributes.Status,
	}
	for rel, data := range i.Relationships {
		if strings.TrimSpace(data.Data.ID) != "" {
			labels[rel+"_id"] = data.Data.ID
		}
	}
	if severity := metadataSeverity(i.Attributes.Metadata); severity != "" {
		labels["severity"] = severity
	}

	annotations := map[string]string{
		"summary": i.Attributes.Cause,
	}
	if strings.TrimSpace(i.Attributes.AcknowledgedBy) != "" {
		annotations["acknowledged_by"] = i.Attributes.AcknowledgedBy
	}
	if strings.TrimSpace(i.Attributes.ResolvedBy) != "" {
		annotations["resolved_by"] = i.Attributes.ResolvedBy
	}
	if link := firstNonEmpty(i.Attributes.OriginURL, i.Attributes.ResponseURL, i.Attributes.URL, i.Attributes.ScreenshotURL); link != "" {
		annotations["link"] = link
	}
	for key, values := range i.Attributes.Metadata {
		if len(values) == 0 || strings.TrimSpace(values[0].Value) == "" {
			continue
		}
		labels["meta_"+normalizeLabelKey(key)] = values[0].Value
	}

	name := strings.TrimSpace(i.Attributes.Name)
	if name == "" {
		name = strings.TrimSpace(i.Attributes.Cause)
	}

	return model.Alert{
		ID:           i.ID,
		Source:       source,
		SourceType:   "betterstack",
		Name:         name,
		Severity:     betterStackSeverity(i.Attributes.CriticalAlert, i.Attributes.Metadata),
		State:        betterStackState(i.Attributes.ResolvedAt),
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     startsAt,
		UpdatedAt:    updatedAt,
		GeneratorURL: firstNonEmpty(i.Attributes.OriginURL, i.Attributes.ResponseURL, i.Attributes.URL),
	}
}

func metadataSeverity(metadata map[string][]bsIncidentMetadataItem) string {
	for key, values := range metadata {
		if !strings.EqualFold(strings.TrimSpace(key), "severity") || len(values) == 0 {
			continue
		}
		if value := strings.TrimSpace(values[0].Value); value != "" {
			return value
		}
	}
	return ""
}

func betterStackSeverity(critical bool, metadata map[string][]bsIncidentMetadataItem) string {
	if severity := metadataSeverity(metadata); severity != "" {
		return severity
	}
	if critical {
		return "critical"
	}
	return "warning"
}

func betterStackState(resolvedAt string) string {
	if parseRFC3339(resolvedAt).IsZero() {
		return "firing"
	}
	return "resolved"
}

func normalizeLabelKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseRFC3339(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	ts, _ := time.Parse(time.RFC3339, value)
	return ts
}

type bsOnCallListResponse struct {
	Data     []bsOnCallSchedule `json:"data"`
	Included []bsOnCallUser     `json:"included"`
}

type bsOnCallSchedule struct {
	ID         string `json:"id"`
	Attributes struct {
		Name            string `json:"name"`
		DefaultCalendar bool   `json:"default_calendar"`
		TeamName        string `json:"team_name"`
	} `json:"attributes"`
	Relationships struct {
		OnCallUsers struct {
			Data []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
				Meta struct {
					Email string `json:"email"`
				} `json:"meta"`
			} `json:"data"`
		} `json:"on_call_users"`
	} `json:"relationships"`
}

type bsOnCallUser struct {
	ID         string `json:"id"`
	Attributes struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	} `json:"attributes"`
}

func (r bsOnCallListResponse) selectSchedule(ref string) (model.OnCallStatus, bool) {
	usersByID := make(map[string]bsOnCallUser, len(r.Included))
	for _, user := range r.Included {
		usersByID[user.ID] = user
	}

	ref = strings.TrimSpace(ref)
	for _, schedule := range r.Data {
		if matchesOnCallSchedule(schedule, ref) {
			return schedule.toStatus(usersByID), true
		}
	}
	return model.OnCallStatus{}, false
}

func matchesOnCallSchedule(schedule bsOnCallSchedule, ref string) bool {
	if ref == "" {
		return false
	}
	if strings.EqualFold(ref, "default") {
		return schedule.Attributes.DefaultCalendar
	}
	return schedule.ID == ref || strings.EqualFold(schedule.Attributes.Name, ref)
}

func (s bsOnCallSchedule) toStatus(usersByID map[string]bsOnCallUser) model.OnCallStatus {
	status := model.OnCallStatus{
		ScheduleID:   s.ID,
		ScheduleName: strings.TrimSpace(s.Attributes.Name),
		TeamName:     strings.TrimSpace(s.Attributes.TeamName),
		Users:        make([]model.OnCallUser, 0, len(s.Relationships.OnCallUsers.Data)),
	}
	if status.ScheduleName == "" && s.Attributes.DefaultCalendar {
		status.ScheduleName = "default"
	}

	for _, ref := range s.Relationships.OnCallUsers.Data {
		user := model.OnCallUser{
			Email: strings.TrimSpace(ref.Meta.Email),
		}
		if included, ok := usersByID[ref.ID]; ok {
			user.Name = strings.TrimSpace(strings.TrimSpace(included.Attributes.FirstName + " " + included.Attributes.LastName))
			if user.Email == "" {
				user.Email = strings.TrimSpace(included.Attributes.Email)
			}
		}
		if user.Name == "" {
			user.Name = user.Email
		}
		status.Users = append(status.Users, user)
	}

	return status
}
