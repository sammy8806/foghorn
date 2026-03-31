package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

type alertmanagerAPI struct {
	cfg    config.SourceConfig
	client *http.Client
	mu     sync.RWMutex
	health model.ProviderHealth
	apiV2  string
	kind   string
}

func NewAlertmanager(cfg config.SourceConfig) *alertmanagerAPI {
	return newAlertmanagerAPI(cfg, "alertmanager", "/api/v2")
}

func NewGrafana(cfg config.SourceConfig) *alertmanagerAPI {
	return newAlertmanagerAPI(cfg, "grafana", "/api/alertmanager/grafana/api/v2")
}

func newAlertmanagerAPI(cfg config.SourceConfig, kind, apiV2 string) *alertmanagerAPI {
	return &alertmanagerAPI{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiV2: apiV2,
		kind:  kind,
	}
}

func (a *alertmanagerAPI) Name() string          { return a.cfg.Name }
func (a *alertmanagerAPI) Type() string          { return a.kind }
func (a *alertmanagerAPI) SupportsSilence() bool { return true }

func (a *alertmanagerAPI) Health(_ context.Context) model.ProviderHealth {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.health
}

func (a *alertmanagerAPI) Fetch(ctx context.Context) ([]model.Alert, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.endpoint("/alerts"), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("active", "true")
	q.Set("silenced", "true")
	q.Set("inhibited", "true")
	for _, f := range a.cfg.Filters {
		q.Add("filter", f)
	}
	req.URL.RawQuery = q.Encode()

	a.applyAuth(req)

	resp, err := a.client.Do(req)
	if err != nil {
		a.recordError(err)
		return nil, fmt.Errorf("fetching alerts from %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.recordError(fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
		return nil, fmt.Errorf("%s %s returned HTTP %d", a.kind, a.cfg.Name, resp.StatusCode)
	}

	var raw []amAlert
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding alerts from %s: %w", a.cfg.Name, err)
	}

	alerts := make([]model.Alert, 0, len(raw))
	for _, r := range raw {
		alerts = append(alerts, r.toAlert(a.cfg.Name, a.kind, a.cfg.SeverityLabel))
	}

	a.mu.Lock()
	a.health = model.ProviderHealth{
		Connected:   true,
		LastSuccess: time.Now(),
		AlertCount:  len(alerts),
	}
	a.mu.Unlock()

	return alerts, nil
}

func (a *alertmanagerAPI) Silence(ctx context.Context, req model.SilenceRequest) (string, error) {
	body := amSilenceRequest{
		Matchers:  make([]amMatcher, len(req.Matchers)),
		StartsAt:  req.StartsAt.Format(time.RFC3339),
		EndsAt:    req.EndsAt.Format(time.RFC3339),
		CreatedBy: req.CreatedBy,
		Comment:   req.Comment,
	}
	for i, m := range req.Matchers {
		body.Matchers[i] = amMatcher{
			Name:    m.Name,
			Value:   m.Value,
			IsRegex: m.IsRegex,
			IsEqual: m.IsEqual,
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.endpoint("/silences"), bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	a.applyAuth(httpReq)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("creating silence on %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("silence on %s %s returned HTTP %d: %s", a.kind, a.cfg.Name, resp.StatusCode, string(respBody))
	}

	var result struct {
		SilenceID string `json:"silenceID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.SilenceID, nil
}

func (a *alertmanagerAPI) Unsilence(ctx context.Context, silenceID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", a.endpoint("/silence/"+silenceID), nil)
	if err != nil {
		return err
	}
	a.applyAuth(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("deleting silence on %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete silence on %s %s returned HTTP %d", a.kind, a.cfg.Name, resp.StatusCode)
	}
	return nil
}

func (a *alertmanagerAPI) FetchSilences(ctx context.Context) ([]model.SilenceInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.endpoint("/silences"), nil)
	if err != nil {
		return nil, err
	}
	a.applyAuth(req)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching silences from %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s %s silences returned HTTP %d: %s", a.kind, a.cfg.Name, resp.StatusCode, string(body))
	}

	var raw []amSilence
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding silences from %s: %w", a.cfg.Name, err)
	}

	var silences []model.SilenceInfo
	for _, s := range raw {
		if s.Status.State != "active" {
			continue
		}
		startsAt, _ := time.Parse(time.RFC3339, s.StartsAt)
		endsAt, _ := time.Parse(time.RFC3339, s.EndsAt)
		silences = append(silences, model.SilenceInfo{
			ID:        s.ID,
			CreatedBy: s.CreatedBy,
			Comment:   s.Comment,
			StartsAt:  startsAt,
			EndsAt:    endsAt,
		})
	}
	return silences, nil
}

func (a *alertmanagerAPI) endpoint(path string) string {
	return strings.TrimRight(a.cfg.URL, "/") + a.apiV2 + path
}

func (a *alertmanagerAPI) applyAuth(req *http.Request) {
	applyAuth(req, a.cfg.Auth)
}

func (a *alertmanagerAPI) recordError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.health.Connected = false
	a.health.LastError = err.Error()
	a.health.ErrorCount++
}

// --- Alertmanager v2 API response types ---

type amAlert struct {
	Fingerprint  string            `json:"fingerprint"`
	StartsAt     string            `json:"startsAt"`
	UpdatedAt    string            `json:"updatedAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Status       amAlertStatus     `json:"status"`
	Receivers    []amReceiver      `json:"receivers"`
}

type amAlertStatus struct {
	State       string   `json:"state"`
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
	MutedBy     []string `json:"mutedBy"`
}

type amReceiver struct {
	Name string `json:"name"`
}

type amSilence struct {
	ID        string          `json:"id"`
	CreatedBy string          `json:"createdBy"`
	Comment   string          `json:"comment"`
	StartsAt  string          `json:"startsAt"`
	EndsAt    string          `json:"endsAt"`
	Status    amSilenceStatus `json:"status"`
}

type amSilenceStatus struct {
	State string `json:"state"`
}

type amSilenceRequest struct {
	Matchers  []amMatcher `json:"matchers"`
	StartsAt  string      `json:"startsAt"`
	EndsAt    string      `json:"endsAt"`
	CreatedBy string      `json:"createdBy"`
	Comment   string      `json:"comment"`
}

type amMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

func (r amAlert) toAlert(source, sourceType, severityLabel string) model.Alert {
	startsAt, _ := time.Parse(time.RFC3339, r.StartsAt)
	updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)

	receivers := make([]string, len(r.Receivers))
	for i, recv := range r.Receivers {
		receivers[i] = recv.Name
	}

	return model.Alert{
		ID:           r.Fingerprint,
		Source:       source,
		SourceType:   sourceType,
		Name:         r.Labels["alertname"],
		Severity:     severityFromLabels(r.Labels, severityLabel),
		State:        r.Status.State,
		Labels:       r.Labels,
		Annotations:  r.Annotations,
		StartsAt:     startsAt,
		UpdatedAt:    updatedAt,
		GeneratorURL: r.GeneratorURL,
		SilencedBy:   r.Status.SilencedBy,
		InhibitedBy:  r.Status.InhibitedBy,
		Receivers:    receivers,
	}
}

func severityFromLabels(labels map[string]string, severityLabel string) string {
	if value := strings.TrimSpace(labels[severityLabel]); value != "" {
		return value
	}
	return strings.TrimSpace(labels["severity"])
}
