package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func TestAlertmanagerFetch(t *testing.T) {
	// Mock Alertmanager v2 API
	alerts := []map[string]interface{}{
		{
			"fingerprint":  "abc123",
			"startsAt":     "2026-03-25T10:00:00Z",
			"updatedAt":    "2026-03-25T10:05:00Z",
			"endsAt":       "0001-01-01T00:00:00Z",
			"generatorURL": "http://prometheus:9090/graph?g0.expr=up",
			"labels": map[string]string{
				"alertname": "TargetDown",
				"severity":  "critical",
				"cluster":   "saas-cs-0b",
				"namespace": "monitoring",
			},
			"annotations": map[string]string{
				"summary":     "Target is down",
				"description": "Target has been down for 5 minutes",
			},
			"status": map[string]interface{}{
				"state":       "active",
				"silencedBy":  []string{},
				"inhibitedBy": []string{},
				"mutedBy":     []string{},
			},
			"receivers": []map[string]string{
				{"name": "default"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/alerts" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(alerts)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name:         "test-am",
		Type:         "alertmanager",
		URL:          server.URL,
		PollInterval: 30 * time.Second,
	}

	am := NewAlertmanager(cfg)

	result, err := am.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(result))
	}

	a := result[0]
	if a.Name != "TargetDown" {
		t.Errorf("expected alertname 'TargetDown', got %q", a.Name)
	}
	if a.Severity != "critical" {
		t.Errorf("expected severity 'critical', got %q", a.Severity)
	}
	if a.Source != "test-am" {
		t.Errorf("expected source 'test-am', got %q", a.Source)
	}
	if a.State != "active" {
		t.Errorf("expected state 'active', got %q", a.State)
	}
	if a.Labels["cluster"] != "saas-cs-0b" {
		t.Errorf("expected cluster 'saas-cs-0b', got %q", a.Labels["cluster"])
	}
}

func TestAlertmanagerSilence(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/silences" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"silenceID": "silence-123"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name: "test-am",
		Type: "alertmanager",
		URL:  server.URL,
	}

	am := NewAlertmanager(cfg)
	req := model.SilenceRequest{
		Matchers: []model.Matcher{
			{Name: "alertname", Value: "TargetDown", IsRegex: false, IsEqual: true},
		},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		CreatedBy: "foghorn",
		Comment:   "Silenced via Foghorn",
	}

	id, err := am.Silence(context.Background(), req)
	if err != nil {
		t.Fatalf("Silence() error: %v", err)
	}
	if id != "silence-123" {
		t.Errorf("expected silence ID 'silence-123', got %q", id)
	}
}

func TestAlertmanagerBasicAuth(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name: "test-am",
		Type: "alertmanager",
		URL:  server.URL,
		Auth: config.AuthConfig{
			Type:     "basic",
			Username: "admin",
			Password: "secret",
		},
	}

	am := NewAlertmanager(cfg)
	am.Fetch(context.Background())

	if receivedAuth == "" {
		t.Error("expected Authorization header, got none")
	}
}
