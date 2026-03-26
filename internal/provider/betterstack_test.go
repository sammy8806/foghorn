package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"foghorn/internal/config"
)

func TestBetterStackFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/incidents" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("resolved"); got != "false" {
			t.Fatalf("expected resolved=false query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "25",
					"type": "incident",
					"attributes": map[string]any{
						"name":                 "uptime homepage",
						"cause":                "Status 404",
						"started_at":           "2020-03-09T17:37:56Z",
						"acknowledged_at":      nil,
						"resolved_at":          nil,
						"status":               "Started",
						"team_name":            "Production",
						"url":                  "https://uptime.betterstack.com/",
						"response_url":         "https://example.com/runbook",
						"origin_url":           "https://example.com/check",
						"critical_alert":       true,
						"escalation_policy_id": 12345,
						"metadata": map[string]any{
							"Response code": []map[string]any{
								{"type": "String", "value": "404"},
							},
						},
					},
					"relationships": map[string]any{
						"monitor": map[string]any{
							"data": map[string]any{"id": "2", "type": "monitor"},
						},
					},
				},
			},
			"pagination": map[string]any{"next": ""},
		})
	}))
	defer server.Close()

	p := NewBetterStack(config.SourceConfig{
		Name: "better",
		Type: "betterstack",
		URL:  server.URL,
		Auth: config.AuthConfig{Type: "bearer", Token: "secret"},
	})

	alerts, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	alert := alerts[0]
	if alert.SourceType != "betterstack" {
		t.Fatalf("expected sourceType betterstack, got %q", alert.SourceType)
	}
	if alert.Severity != "critical" {
		t.Fatalf("expected severity critical, got %q", alert.Severity)
	}
	if alert.State != "firing" {
		t.Fatalf("expected state firing, got %q", alert.State)
	}
	if alert.Labels["monitor_id"] != "2" {
		t.Fatalf("expected monitor_id label, got %#v", alert.Labels)
	}
	if alert.Annotations["summary"] != "Status 404" {
		t.Fatalf("expected summary annotation, got %#v", alert.Annotations)
	}
}

func TestBetterStackFetchOnCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/on-calls" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("team_name"); got != "Production" {
			t.Fatalf("expected team_name=Production, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "12345",
					"type": "on_call_calendar",
					"attributes": map[string]any{
						"name":             nil,
						"default_calendar": true,
						"team_name":        "Production",
					},
					"relationships": map[string]any{
						"on_call_users": map[string]any{
							"data": []map[string]any{
								{
									"id":   "2345",
									"type": "user",
									"meta": map[string]any{"email": "tomas@betterstack.com"},
								},
							},
						},
					},
				},
			},
			"included": []map[string]any{
				{
					"id":   "2345",
					"type": "user",
					"attributes": map[string]any{
						"first_name": "Tomas",
						"last_name":  "Hromada",
						"email":      "tomas@betterstack.com",
					},
				},
			},
		})
	}))
	defer server.Close()

	p := NewBetterStack(config.SourceConfig{
		Name: "better",
		Type: "betterstack",
		URL:  server.URL,
		Auth: config.AuthConfig{Type: "bearer", Token: "secret"},
		BetterStack: config.BetterStackConfig{
			OnCallSchedule: "default",
			TeamName:       "Production",
		},
	})

	status, err := p.FetchOnCall(context.Background())
	if err != nil {
		t.Fatalf("FetchOnCall() error: %v", err)
	}
	if status == nil {
		t.Fatal("expected on-call status, got nil")
	}
	if status.ScheduleID != "12345" {
		t.Fatalf("expected schedule id 12345, got %q", status.ScheduleID)
	}
	if len(status.Users) != 1 || status.Users[0].Name != "Tomas Hromada" {
		t.Fatalf("unexpected on-call users: %#v", status.Users)
	}
}
