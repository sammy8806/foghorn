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

func TestAlertmanagerSilenceMissingIDIsError(t *testing.T) {
	// Server returns 200 but no silenceID — e.g. a redirected POST that got
	// downgraded to a GET and returned the silence list instead of a creation
	// result. This must be treated as a failure, not a silent success.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Valid JSON object that decodes cleanly but carries no silenceID.
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	am := NewAlertmanager(config.SourceConfig{Name: "test-am", Type: "alertmanager", URL: server.URL})
	req := model.SilenceRequest{
		Matchers:  []model.Matcher{{Name: "alertname", Value: "TargetDown", IsEqual: true}},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		CreatedBy: "foghorn",
	}

	id, err := am.Silence(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error when server returns no silenceID, got id=%q nil error", id)
	}
}

func TestAlertmanagerSilenceDoesNotFollowRedirect(t *testing.T) {
	// A reverse proxy that 301-redirects the POST must not be silently followed
	// (Go would downgrade POST->GET and drop the body). It must surface an error.
	var postSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			postSeen = true
			http.Redirect(w, r, r.URL.Path, http.StatusMovedPermanently)
			return
		}
		// A followed GET would land here and return a valid 200 object that
		// decodes cleanly but has no silenceID.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	am := NewAlertmanager(config.SourceConfig{Name: "test-am", Type: "alertmanager", URL: server.URL})
	req := model.SilenceRequest{
		Matchers:  []model.Matcher{{Name: "alertname", Value: "TargetDown", IsEqual: true}},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		CreatedBy: "foghorn",
	}

	id, err := am.Silence(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error on redirect, got id=%q nil error", id)
	}
	if !postSeen {
		t.Fatal("expected POST to reach server")
	}
}

func TestAlertmanagerFetchSilences(t *testing.T) {
	silences := []map[string]interface{}{
		{
			"id":        "sil-001",
			"createdBy": "operator-a",
			"comment":   "noisy during maintenance",
			"startsAt":  "2026-03-31T10:00:00Z",
			"endsAt":    "2026-03-31T14:00:00Z",
			"status":    map[string]string{"state": "active"},
		},
		{
			"id":        "sil-002",
			"createdBy": "operator-b",
			"comment":   "investigating root cause",
			"startsAt":  "2026-03-31T08:00:00Z",
			"endsAt":    "2026-03-31T10:00:00Z",
			"status":    map[string]string{"state": "expired"},
		},
		{
			"id":        "sil-003",
			"createdBy": "operator-c",
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
	if result[0].CreatedBy != "operator-a" {
		t.Errorf("expected createdBy 'operator-a', got %q", result[0].CreatedBy)
	}
	if result[0].Comment != "noisy during maintenance" {
		t.Errorf("expected comment 'noisy during maintenance', got %q", result[0].Comment)
	}

	if result[1].ID != "sil-003" {
		t.Errorf("expected second silence ID 'sil-003', got %q", result[1].ID)
	}
	if result[1].CreatedBy != "operator-c" {
		t.Errorf("expected createdBy 'operator-c', got %q", result[1].CreatedBy)
	}
}

func TestAlertmanagerSilenceForwardsID(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/silences" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"silenceID": "silence-xyz"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := config.SourceConfig{Name: "test-am", Type: "alertmanager", URL: server.URL}
	am := NewAlertmanager(cfg)
	req := model.SilenceRequest{
		ID: "existing-id",
		Matchers: []model.Matcher{
			{Name: "alertname", Value: "X", IsRegex: false, IsEqual: true},
		},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
		CreatedBy: "foghorn",
		Comment:   "update test",
	}

	if _, err := am.Silence(context.Background(), req); err != nil {
		t.Fatalf("Silence() error: %v", err)
	}
	if got, _ := receivedBody["id"].(string); got != "existing-id" {
		t.Errorf("expected id 'existing-id' in body, got %q (body=%v)", got, receivedBody)
	}
}

func TestAlertmanagerSilenceOmitsEmptyID(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/silences" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"silenceID": "new-silence"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := config.SourceConfig{Name: "test-am", Type: "alertmanager", URL: server.URL}
	am := NewAlertmanager(cfg)
	req := model.SilenceRequest{
		Matchers: []model.Matcher{
			{Name: "alertname", Value: "X", IsRegex: false, IsEqual: true},
		},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
		CreatedBy: "foghorn",
		Comment:   "create test",
	}

	if _, err := am.Silence(context.Background(), req); err != nil {
		t.Fatalf("Silence() error: %v", err)
	}
	if _, present := receivedBody["id"]; present {
		t.Errorf("expected no id field when creating, got body=%v", receivedBody)
	}
}

func TestAlertmanagerFetchSilencesParsesMatchers(t *testing.T) {
	silences := []map[string]interface{}{
		{
			"id":        "sil-m",
			"createdBy": "operator-a",
			"comment":   "mixed matchers",
			"startsAt":  "2026-03-31T10:00:00Z",
			"endsAt":    "2026-03-31T14:00:00Z",
			"status":    map[string]string{"state": "active"},
			"matchers": []map[string]interface{}{
				{"name": "a", "value": "1", "isRegex": false, "isEqual": true},
				{"name": "b", "value": ".*", "isRegex": true, "isEqual": true},
				{"name": "c", "value": "2", "isRegex": false, "isEqual": false},
				{"name": "d", "value": "x", "isRegex": true, "isEqual": false},
			},
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

	cfg := config.SourceConfig{Name: "test-am", Type: "alertmanager", URL: server.URL}
	am := NewAlertmanager(cfg)
	result, err := am.FetchSilences(context.Background())
	if err != nil {
		t.Fatalf("FetchSilences() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 silence, got %d", len(result))
	}
	got := result[0].Matchers
	if len(got) != 4 {
		t.Fatalf("expected 4 matchers, got %d (%v)", len(got), got)
	}
	checks := []struct {
		idx     int
		name    string
		value   string
		isRegex bool
		isEqual bool
	}{
		{0, "a", "1", false, true},
		{1, "b", ".*", true, true},
		{2, "c", "2", false, false},
		{3, "d", "x", true, false},
	}
	for _, c := range checks {
		m := got[c.idx]
		if m.Name != c.name || m.Value != c.value || m.IsRegex != c.isRegex || m.IsEqual != c.isEqual {
			t.Errorf("matcher %d = %+v, want {%s %s %v %v}", c.idx, m, c.name, c.value, c.isRegex, c.isEqual)
		}
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

// generatorURL is fully controlled by the alert source and is rendered as a
// link in the UI, so non-http(s) values must not reach the model.
func TestAlertmanagerDropsUnsafeGeneratorURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"fingerprint":"a","labels":{"alertname":"Evil"},"generatorURL":"javascript:alert(1)","status":{"state":"active"}},
			{"fingerprint":"b","labels":{"alertname":"Ok"},"generatorURL":"https://prom.example/graph","status":{"state":"active"}}
		]`))
	}))
	defer server.Close()

	p := NewAlertmanager(config.SourceConfig{Name: "am", Type: "alertmanager", URL: server.URL})

	alerts, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(alerts))
	}
	if alerts[0].GeneratorURL != "" {
		t.Fatalf("expected javascript: generatorURL to be dropped, got %q", alerts[0].GeneratorURL)
	}
	if alerts[1].GeneratorURL != "https://prom.example/graph" {
		t.Fatalf("expected https generatorURL to be kept, got %q", alerts[1].GeneratorURL)
	}
}

// Silence IDs are echoed back from the source; unescaped they could traverse to
// other API paths on the same host.
func TestAlertmanagerUnsilenceEscapesID(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewAlertmanager(config.SourceConfig{Name: "am", Type: "alertmanager", URL: server.URL})

	if err := p.Unsilence(context.Background(), "../../silences"); err != nil {
		t.Fatalf("Unsilence() error: %v", err)
	}
	if want := "/api/v2/silence/..%2F..%2Fsilences"; gotPath != want {
		t.Fatalf("silence ID was not escaped: got path %q, want %q", gotPath, want)
	}
}
