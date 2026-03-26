package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

// Prometheus implements Provider for the Prometheus HTTP API /api/v1/alerts endpoint.
type Prometheus struct {
	cfg    config.SourceConfig
	client *http.Client
	mu     sync.RWMutex
	health model.ProviderHealth
}

func NewPrometheus(cfg config.SourceConfig) *Prometheus {
	return &Prometheus{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *Prometheus) Name() string { return p.cfg.Name }
func (p *Prometheus) Type() string { return "prometheus" }

func (p *Prometheus) Health(_ context.Context) model.ProviderHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.health
}

func (p *Prometheus) Fetch(ctx context.Context) ([]model.Alert, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.URL+"/api/v1/alerts", nil)
	if err != nil {
		return nil, err
	}
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		p.recordError(err)
		return nil, fmt.Errorf("fetching alerts from %s: %w", p.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.recordError(fmt.Errorf("HTTP %d", resp.StatusCode))
		return nil, fmt.Errorf("prometheus %s returned HTTP %d: %s", p.cfg.Name, resp.StatusCode, string(body))
	}

	var envelope promAlertsResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding alerts from %s: %w", p.cfg.Name, err)
	}
	if envelope.Status != "success" {
		return nil, fmt.Errorf("prometheus %s returned status %q: %s", p.cfg.Name, envelope.Status, envelope.Error)
	}

	alerts := make([]model.Alert, 0, len(envelope.Data.Alerts))
	for _, a := range envelope.Data.Alerts {
		alerts = append(alerts, a.toAlert(p.cfg.Name))
	}

	p.mu.Lock()
	p.health = model.ProviderHealth{
		Connected:   true,
		LastSuccess: time.Now(),
		AlertCount:  len(alerts),
	}
	p.mu.Unlock()

	return alerts, nil
}

// Silence is not supported by Prometheus — use Alertmanager for silencing.
func (p *Prometheus) Silence(_ context.Context, _ model.SilenceRequest) (string, error) {
	return "", fmt.Errorf("silence not supported by Prometheus provider %q; use an Alertmanager source", p.cfg.Name)
}

func (p *Prometheus) Unsilence(_ context.Context, _ string) error {
	return fmt.Errorf("unsilence not supported by Prometheus provider %q", p.cfg.Name)
}

func (p *Prometheus) applyAuth(req *http.Request) {
	applyAuth(req, p.cfg.Auth)
}

func (p *Prometheus) recordError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health.Connected = false
	p.health.LastError = err.Error()
	p.health.ErrorCount++
}

// --- Prometheus API response types ---

type promAlertsResponse struct {
	Status string         `json:"status"`
	Data   promAlertsData `json:"data"`
	Error  string         `json:"error"`
}

type promAlertsData struct {
	Alerts []promAlert `json:"alerts"`
}

type promAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    string            `json:"activeAt"`
	Value       string            `json:"value"`
}

func (a promAlert) toAlert(source string) model.Alert {
	activeAt, _ := time.Parse(time.RFC3339Nano, a.ActiveAt)

	// Generate a stable fingerprint from labels
	fingerprint := labelFingerprint(a.Labels)

	return model.Alert{
		ID:          fingerprint,
		Source:      source,
		SourceType:  "prometheus",
		Name:        a.Labels["alertname"],
		Severity:    a.Labels["severity"],
		State:       a.State,
		Labels:      a.Labels,
		Annotations: a.Annotations,
		StartsAt:    activeAt,
		UpdatedAt:   time.Now(),
	}
}

func labelFingerprint(labels map[string]string) string {
	// Build a stable key from sorted label pairs
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	// Simple deterministic join (good enough for dedup, not cryptographic)
	h := 0
	for _, k := range keys {
		for _, c := range k + "=" + labels[k] + "," {
			h = h*31 + int(c)
		}
	}
	return fmt.Sprintf("prom-%x", h)
}
