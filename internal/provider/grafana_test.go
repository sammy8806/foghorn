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

func TestGrafanaFetch(t *testing.T) {
	alerts := []map[string]interface{}{
		{
			"fingerprint":  "graf123",
			"startsAt":     "2026-03-25T10:00:00Z",
			"updatedAt":    "2026-03-25T10:05:00Z",
			"generatorURL": "https://grafana.example.com/alerting/graf123/view",
			"labels": map[string]string{
				"alertname":      "DatasourceError",
				"severity":       "critical",
				"grafana_folder": "Infra",
			},
			"annotations": map[string]string{
				"summary": "Datasource health check failed",
			},
			"status": map[string]interface{}{
				"state":       "active",
				"silencedBy":  []string{},
				"inhibitedBy": []string{},
				"mutedBy":     []string{},
			},
			"receivers": []map[string]string{
				{"name": "grafana-default-email"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alertmanager/grafana/api/v2/alerts" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	}))
	defer server.Close()

	grafana := NewGrafana(config.SourceConfig{
		Name:         "grafana-main",
		Type:         "grafana",
		URL:          server.URL,
		PollInterval: 30 * time.Second,
	})

	result, err := grafana.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(result))
	}
	if result[0].SourceType != "grafana" {
		t.Fatalf("expected source type grafana, got %q", result[0].SourceType)
	}
	if result[0].Receivers[0] != "grafana-default-email" {
		t.Fatalf("expected receiver grafana-default-email, got %#v", result[0].Receivers)
	}
}

func TestGrafanaSilenceAccepts202(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alertmanager/grafana/api/v2/silences" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"silenceID": "grafana-silence-123"})
	}))
	defer server.Close()

	grafana := NewGrafana(config.SourceConfig{
		Name: "grafana-main",
		Type: "grafana",
		URL:  server.URL,
	})

	id, err := grafana.Silence(context.Background(), model.SilenceRequest{
		Matchers: []model.Matcher{{Name: "alertname", Value: "DatasourceError", IsEqual: true}},
		StartsAt: time.Now(),
		EndsAt:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Silence() error: %v", err)
	}
	if id != "grafana-silence-123" {
		t.Fatalf("expected grafana-silence-123, got %q", id)
	}
}

func TestGrafanaBearerAuth(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer server.Close()

	grafana := NewGrafana(config.SourceConfig{
		Name: "grafana-main",
		Type: "grafana",
		URL:  server.URL,
		Auth: config.AuthConfig{
			Type:  "bearer",
			Token: "grafana-token",
		},
	})

	if _, err := grafana.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if receivedAuth != "Bearer grafana-token" {
		t.Fatalf("expected bearer auth header, got %q", receivedAuth)
	}
}
