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
		switch r.URL.Path {
		case "/api/v3/incidents":
			if got := r.URL.Query().Get("resolved"); got != "false" {
				t.Fatalf("expected resolved=false query, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":   "incident-001",
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
							"escalation_policy_id": "policy-001",
							"metadata": map[string]any{
								"Response code": []map[string]any{
									{"type": "String", "value": "404"},
								},
							},
						},
						"relationships": map[string]any{
							"monitor": map[string]any{
								"data": map[string]any{"id": "monitor-001", "type": "monitor"},
							},
						},
					},
				},
				"pagination": map[string]any{"next": ""},
			})
		case "/api/v2/incidents/incident-001/comments":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":   "comment-001",
						"type": "incident_comment",
						"attributes": map[string]any{
							"content":    "Investigating issue",
							"user_email": "responder@example.test",
							"created_at": "2025-06-03T12:10:28.357Z",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := NewBetterStack(config.SourceConfig{
		Name: "better",
		Type: "betterstack",
		URL:  server.URL,
		Auth: config.AuthConfig{Type: "bearer", Token: "secret"},
		BetterStack: config.BetterStackConfig{
			TeamID: "team-001",
		},
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
	if alert.Labels["monitor_id"] != "monitor-001" {
		t.Fatalf("expected monitor_id label, got %#v", alert.Labels)
	}
	if alert.Annotations["summary"] != "Status 404" {
		t.Fatalf("expected summary annotation, got %#v", alert.Annotations)
	}
	if alert.Annotations["comments"] != "responder@example.test - 2025-06-03T12:10:28Z\nInvestigating issue" {
		t.Fatalf("expected comments annotation, got %#v", alert.Annotations)
	}
	if alert.Annotations["link"] != "https://uptime.betterstack.com/" {
		t.Fatalf("expected incident link annotation, got %#v", alert.Annotations)
	}
	if alert.GeneratorURL != server.URL+"/team/team-001/incidents/incident-001" {
		t.Fatalf("expected generator URL to prefer team incident page, got %q", alert.GeneratorURL)
	}
}

func TestFormatBetterStackCommentsFormatsMentions(t *testing.T) {
	comments := []bsIncidentComment{
		{
			ID: "comment-001",
			Attributes: struct {
				Content   string `json:"content"`
				UserEmail string `json:"user_email"`
				UserName  string `json:"user_name"`
				CreatedAt string `json:"created_at"`
			}{
				Content:   `**M_START**{"type":"User","id":"user-002","value":"user-002","better_stack_id":"bs-user-002","email":"mention-target@example.test","tagify_name":"mention-target","avatar_url":"https://example.test/avatar.png","prefix":"@"}**M_END** Done.`,
				UserEmail: "comment-author@example.test",
				UserName:  "comment-author",
				CreatedAt: "2026-05-28T09:29:32Z",
			},
		},
	}

	got := formatBetterStackComments(comments)
	want := "comment-author <comment-author@example.test> - 2026-05-28T09:29:32Z\n[@mention-target](mailto:mention-target@example.test) Done."
	if got != want {
		t.Fatalf("expected formatted mention comment %q, got %q", want, got)
	}
}

func TestFormatBetterStackCommentsUsesEmailWhenAuthorNameMissing(t *testing.T) {
	comments := []bsIncidentComment{
		{
			ID: "comment-001",
			Attributes: struct {
				Content   string `json:"content"`
				UserEmail string `json:"user_email"`
				UserName  string `json:"user_name"`
				CreatedAt string `json:"created_at"`
			}{
				Content:   "Investigating issue",
				UserEmail: "comment-author@example.test",
				CreatedAt: "2026-05-28T09:29:32Z",
			},
		},
	}

	got := formatBetterStackComments(comments)
	want := "comment-author@example.test - 2026-05-28T09:29:32Z\nInvestigating issue"
	if got != want {
		t.Fatalf("expected email author comment %q, got %q", want, got)
	}
}

func TestFormatBetterStackCommentsFormatsEscapedMentions(t *testing.T) {
	got := formatBetterStackMentions(`**M_START**{\"type\":\"User\",\"id\":\"user-002\",\"value\":\"user-002\",\"better_stack_id\":\"bs-user-002\",\"email\":\"mention-target@example.test\",\"tagify_name\":\"mention-target\",\"prefix\":\"@\"}**M_END** Done.`)
	want := "[@mention-target](mailto:mention-target@example.test) Done."
	if got != want {
		t.Fatalf("expected escaped mention %q, got %q", want, got)
	}
}

func TestFormatBetterStackCommentsFormatsNumericMentions(t *testing.T) {
	got := formatBetterStackMentions(`**M_START**{"type":"User","id":1002,"value":1002,"better_stack_id":2002,"email":"mention-target@example.test","tagify_name":"mention-target","avatar_url":"https://example.test/avatar.png","prefix":"@"}**M_END** 

Disabling this monitor for now.`)
	want := "[@mention-target](mailto:mention-target@example.test) \n\nDisabling this monitor for now."
	if got != want {
		t.Fatalf("expected numeric mention %q, got %q", want, got)
	}
}

func TestBetterStackFetchFallsBackToIncidentURLWithoutTeamID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/incidents":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":   "incident-001",
						"type": "incident",
						"attributes": map[string]any{
							"name":       "uptime homepage",
							"cause":      "Status 404",
							"started_at": "2020-03-09T17:37:56Z",
							"status":     "Started",
							"team_name":  "Production",
							"url":        "https://uptime.betterstack.com/",
						},
					},
				},
				"pagination": map[string]any{"next": ""},
			})
		case "/api/v2/incidents/incident-001/comments":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{},
			})
		default:
			http.NotFound(w, r)
			return
		}
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
	if alerts[0].GeneratorURL != "https://uptime.betterstack.com/" {
		t.Fatalf("expected generator URL to fall back to incident URL, got %q", alerts[0].GeneratorURL)
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
					"id":   "schedule-001",
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
									"id":   "user-001",
									"type": "user",
									"meta": map[string]any{"email": "primary-oncall@example.test"},
								},
							},
						},
					},
				},
			},
			"included": []map[string]any{
				{
					"id":   "user-001",
					"type": "user",
					"attributes": map[string]any{
						"first_name": "on-call",
						"last_name":  "primary",
						"email":      "primary-oncall@example.test",
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
	if status.ScheduleID != "schedule-001" {
		t.Fatalf("expected schedule id schedule-001, got %q", status.ScheduleID)
	}
	if len(status.Users) != 1 || status.Users[0].Name != "on-call primary" {
		t.Fatalf("unexpected on-call users: %#v", status.Users)
	}
}
