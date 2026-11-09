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

func TestGrafanaFetchUsesConfiguredSeverityLabel(t *testing.T) {
	alerts := []map[string]interface{}{
		{
			"fingerprint":  "graf123",
			"startsAt":     "2026-03-25T10:00:00Z",
			"updatedAt":    "2026-03-25T10:05:00Z",
			"generatorURL": "https://grafana.example.com/alerting/graf123/view",
			"labels": map[string]string{
				"alertname":      "DatasourceError",
				"priority":       "warning",
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
		Name:          "grafana-main",
		Type:          "grafana",
		URL:           server.URL,
		PollInterval:  30 * time.Second,
		SeverityLabel: "priority",
	})

	result, err := grafana.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(result))
	}
	if result[0].Severity != "warning" {
		t.Fatalf("expected severity warning from priority label, got %q", result[0].Severity)
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

func grafanaTestSource(url string) config.SourceConfig {
	return config.SourceConfig{Name: "grafana-main", Type: "grafana", URL: url}
}

func TestGrafanaSilenceForwardsID(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alertmanager/grafana/api/v2/silences" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"silenceID": "existing-id"})
	}))
	defer server.Close()

	_, err := NewGrafana(grafanaTestSource(server.URL)).Silence(context.Background(), model.SilenceRequest{
		ID:       "existing-id",
		Matchers: []model.Matcher{{Name: "alertname", Value: "X", IsEqual: true}},
		StartsAt: time.Now(),
		EndsAt:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Silence() error: %v", err)
	}
	if got, _ := body["id"].(string); got != "existing-id" {
		t.Fatalf("expected id existing-id in body, got %q", got)
	}
}

func TestGrafanaUnsilence(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.EscapedPath(), r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := NewGrafana(grafanaTestSource(server.URL)).Unsilence(context.Background(), "abc/def"); err != nil {
		t.Fatalf("Unsilence() error: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/alertmanager/grafana/api/v2/silence/abc%2Fdef" {
		t.Fatalf("unexpected request %s %s", gotMethod, gotPath)
	}
}

func TestGrafanaFetchSilences(t *testing.T) {
	silences := []map[string]interface{}{
		{
			"id": "s-active", "createdBy": "a", "comment": "c",
			"startsAt": "2026-03-31T10:00:00Z", "endsAt": "2026-03-31T14:00:00Z",
			"status": map[string]string{"state": "active"},
			"matchers": []map[string]interface{}{
				{"name": "__alert_rule_uid__", "value": "r1", "isRegex": false, "isEqual": true},
			},
		},
		{
			"id": "s-expired", "createdBy": "b", "comment": "",
			"startsAt": "2026-03-31T08:00:00Z", "endsAt": "2026-03-31T09:00:00Z",
			"status": map[string]string{"state": "expired"},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alertmanager/grafana/api/v2/silences" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(silences)
	}))
	defer server.Close()

	got, err := NewGrafana(grafanaTestSource(server.URL)).FetchSilences(context.Background())
	if err != nil {
		t.Fatalf("FetchSilences() error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s-active" {
		t.Fatalf("expected only s-active, got %+v", got)
	}
	want := model.Matcher{Name: "__alert_rule_uid__", Value: "r1", IsEqual: true}
	if len(got[0].Matchers) != 1 || got[0].Matchers[0] != want {
		t.Fatalf("unexpected matchers %+v", got[0].Matchers)
	}
}
