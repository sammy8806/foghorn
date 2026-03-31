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

func TestAlertmanagerFetchSilences(t *testing.T) {
	silences := []map[string]interface{}{
		{
			"id":        "sil-001",
			"createdBy": "steve",
			"comment":   "noisy during maintenance",
			"startsAt":  "2026-03-31T10:00:00Z",
			"endsAt":    "2026-03-31T14:00:00Z",
			"status":    map[string]string{"state": "active"},
		},
		{
			"id":        "sil-002",
			"createdBy": "alice",
			"comment":   "investigating root cause",
			"startsAt":  "2026-03-31T08:00:00Z",
			"endsAt":    "2026-03-31T10:00:00Z",
			"status":    map[string]string{"state": "expired"},
		},
		{
			"id":        "sil-003",
			"createdBy": "bob",
			"comment":   "",
			"startsAt":  "2026-03-31T12:00:00Z",
			"endsAt":    "2026-03-31T16:00:00Z",
			"status":    map[string]string{"state": "active"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/silences" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(silences)
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
	result, err := am.FetchSilences(context.Background())
	if err != nil {
		t.Fatalf("FetchSilences() error: %v", err)
	}

	// Should only return active silences (sil-001 and sil-003), not expired (sil-002)
	if len(result) != 2 {
		t.Fatalf("expected 2 active silences, got %d", len(result))
	}

	if result[0].ID != "sil-001" {
		t.Errorf("expected first silence ID 'sil-001', got %q", result[0].ID)
	}
	if result[0].CreatedBy != "steve" {
		t.Errorf("expected createdBy 'steve', got %q", result[0].CreatedBy)
	}
	if result[0].Comment != "noisy during maintenance" {
		t.Errorf("expected comment 'noisy during maintenance', got %q", result[0].Comment)
	}

	if result[1].ID != "sil-003" {
		t.Errorf("expected second silence ID 'sil-003', got %q", result[1].ID)
	}
	if result[1].CreatedBy != "bob" {
		t.Errorf("expected createdBy 'bob', got %q", result[1].CreatedBy)
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
