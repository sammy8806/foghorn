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

func TestPrometheusFetch(t *testing.T) {
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"alerts": []map[string]interface{}{
				{
					"labels": map[string]string{
						"alertname": "HighCPU",
						"severity":  "warning",
						"cluster":   "prod",
						"namespace": "app",
					},
					"annotations": map[string]string{
						"summary": "CPU usage is high",
					},
					"state":    "firing",
					"activeAt": time.Now().Format(time.RFC3339Nano),
					"value":    "0.95",
				},
				{
					"labels": map[string]string{
						"alertname": "DiskFull",
						"severity":  "critical",
						"cluster":   "prod",
					},
					"annotations": map[string]string{},
					"state":       "firing",
					"activeAt":    time.Now().Format(time.RFC3339Nano),
					"value":       "1",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/alerts" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name: "test-prom",
		Type: "prometheus",
		URL:  server.URL,
	}

	p := NewPrometheus(cfg)
	alerts, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}

	// Find HighCPU
	var highCPU, diskFull *struct{ name, severity, state string }
	for _, a := range alerts {
		if a.Name == "HighCPU" {
			highCPU = &struct{ name, severity, state string }{a.Name, a.Severity, a.State}
		}
		if a.Name == "DiskFull" {
			diskFull = &struct{ name, severity, state string }{a.Name, a.Severity, a.State}
		}
	}

	if highCPU == nil {
		t.Fatal("expected HighCPU alert, not found")
	}
	if highCPU.severity != "warning" {
		t.Errorf("expected severity 'warning', got %q", highCPU.severity)
	}
	if highCPU.state != "firing" {
		t.Errorf("expected state 'firing', got %q", highCPU.state)
	}
	if diskFull == nil {
		t.Fatal("expected DiskFull alert, not found")
	}

	// Verify source name is set
	if alerts[0].Source != "test-prom" {
		t.Errorf("expected source 'test-prom', got %q", alerts[0].Source)
	}
}

func TestPrometheusErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "something went wrong",
		})
	}))
	defer server.Close()

	cfg := config.SourceConfig{Name: "test-prom", URL: server.URL}
	p := NewPrometheus(cfg)
	_, err := p.Fetch(context.Background())
	if err == nil {
		t.Error("expected error for error status, got nil")
	}
}

func TestPrometheusSilenceUnsupported(t *testing.T) {
	p := NewPrometheus(config.SourceConfig{Name: "test"})
	_, err := p.Silence(context.Background(), model.SilenceRequest{})
	if err == nil {
		t.Error("expected error for unsupported Silence(), got nil")
	}
}
