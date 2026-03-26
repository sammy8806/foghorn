package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
sources:
  - name: test-am
    type: alertmanager
    url: http://localhost:9093
    severity_label: priority
    auth:
      type: basic
      username: admin
      password: secret
    poll_interval: 30s

severities:
  default: info
  levels:
    - name: critical
      aliases: [critical, p1]
    - name: info
      color: "#00ff00"
      aliases: [info, notice]

display:
  visible_labels: [alertname, severity]
  visible_annotations: [summary]
  group_by: [cluster]
  group_by_override_key_mode: raw
  group_by_overrides:
    prod:
      - label:namespace
  priority:
    sources: [betterstack-production]
    source_types: [betterstack]
  badges:
    - label: Ack
      field: label:status
      equals: [Acknowledged]
      source_types: [betterstack]
  sort_by: severity

sounds:
  enabled: false

notifications:
  enabled: true
  on_new: true
  on_resolved: false
  batch_threshold: 5

actions: []

resolvers:
  - name: cluster-name
    field: label:cluster
    command: ./resolve-cluster
    args: ["{{.Value}}"]
    timeout: 500ms
    cache_ttl: 24h

ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
	}
	if cfg.Sources[0].Name != "test-am" {
		t.Errorf("expected source name 'test-am', got %q", cfg.Sources[0].Name)
	}
	if cfg.Sources[0].PollInterval.Seconds() != 30 {
		t.Errorf("expected poll_interval 30s, got %v", cfg.Sources[0].PollInterval)
	}
	if cfg.Sources[0].SeverityLabel != "priority" {
		t.Errorf("expected severity_label priority, got %q", cfg.Sources[0].SeverityLabel)
	}
	if cfg.UI.PopupWidth != 800 {
		t.Errorf("expected popup_width 800, got %d", cfg.UI.PopupWidth)
	}
	if cfg.Severities.Default != "info" {
		t.Fatalf("expected severity default info, got %q", cfg.Severities.Default)
	}
	if got := cfg.Severities.Levels[0].Aliases; len(got) != 2 || got[1] != "p1" {
		t.Fatalf("expected severity aliases [critical p1], got %#v", got)
	}
	if cfg.Severities.Levels[1].Color != "#00ff00" {
		t.Fatalf("expected custom info color, got %q", cfg.Severities.Levels[1].Color)
	}
	if len(cfg.Resolvers) != 1 {
		t.Fatalf("expected 1 resolver, got %d", len(cfg.Resolvers))
	}
	if cfg.Resolvers[0].Field != "label:cluster" {
		t.Fatalf("expected resolver field label:cluster, got %q", cfg.Resolvers[0].Field)
	}
	if cfg.Resolvers[0].CacheTTL != 24*time.Hour {
		t.Fatalf("expected resolver cache_ttl 24h, got %v", cfg.Resolvers[0].CacheTTL)
	}
	if got := cfg.Display.GroupByOverrides["prod"]; len(got) != 1 || got[0] != "label:namespace" {
		t.Fatalf("expected group_by_overrides for prod, got %#v", got)
	}
	if got := cfg.Display.Priority.Sources; len(got) != 1 || got[0] != "betterstack-production" {
		t.Fatalf("expected display priority source override, got %#v", got)
	}
	if got := cfg.Display.Priority.SourceTypes; len(got) != 1 || got[0] != "betterstack" {
		t.Fatalf("expected display priority source_types override, got %#v", got)
	}
	if got := cfg.Display.OverrideKeyMode(); got != "raw" {
		t.Fatalf("expected group_by_override_key_mode raw, got %q", got)
	}
	if len(cfg.Display.Badges) != 1 {
		t.Fatalf("expected 1 display badge, got %d", len(cfg.Display.Badges))
	}
	if cfg.Display.Badges[0].Label != "Ack" {
		t.Fatalf("expected display badge label Ack, got %q", cfg.Display.Badges[0].Label)
	}
}

func TestLoadConfigDefaultsSeverityLabel(t *testing.T) {
	yaml := `
sources:
  - name: test
    type: grafana
    url: http://localhost:3000
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Sources[0].SeverityLabel != "severity" {
		t.Fatalf("expected default severity_label severity, got %q", cfg.Sources[0].SeverityLabel)
	}
}

func TestLoadConfigBetterStackDefaultsURL(t *testing.T) {
	yaml := `
sources:
  - name: better
    type: betterstack
    auth:
      type: bearer
      token: secret
    betterstack:
      on_call_schedule: default
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Sources[0].URL != "https://uptime.betterstack.com" {
		t.Fatalf("expected Better Stack default URL, got %q", cfg.Sources[0].URL)
	}
	if cfg.Sources[0].BetterStack.OnCallSchedule != "default" {
		t.Fatalf("expected on_call_schedule default, got %q", cfg.Sources[0].BetterStack.OnCallSchedule)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	os.Setenv("FOGHORN_TEST_USER", "testuser")
	os.Setenv("FOGHORN_TEST_PASS", "testpass")
	defer os.Unsetenv("FOGHORN_TEST_USER")
	defer os.Unsetenv("FOGHORN_TEST_PASS")

	yaml := `
sources:
  - name: test
    type: alertmanager
    url: http://localhost:9093
    auth:
      type: basic
      username: ${FOGHORN_TEST_USER}
      password: ${FOGHORN_TEST_PASS}
    poll_interval: 30s
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Sources[0].Auth.Username != "testuser" {
		t.Errorf("expected expanded username 'testuser', got %q", cfg.Sources[0].Auth.Username)
	}
	if cfg.Sources[0].Auth.Password != "testpass" {
		t.Errorf("expected expanded password 'testpass', got %q", cfg.Sources[0].Auth.Password)
	}
}

func TestDefaultEnablesNewAlertNotifications(t *testing.T) {
	cfg := Default()

	if !cfg.Notifications.Enabled {
		t.Fatal("expected notifications to be enabled by default")
	}
	if !cfg.Notifications.OnNew {
		t.Fatal("expected new alert notifications to be enabled by default")
	}
	if cfg.Notifications.OnResolved {
		t.Fatal("expected resolved alert notifications to remain disabled by default")
	}
	if cfg.Notifications.BatchThreshold != 5 {
		t.Fatalf("expected default batch threshold 5, got %d", cfg.Notifications.BatchThreshold)
	}
}

func TestLoadConfigRejectsDuplicateSeverityAliases(t *testing.T) {
	yaml := `
sources:
  - name: test
    type: alertmanager
    url: http://localhost:9093
severities:
  levels:
    - name: critical
      aliases: [critical, sev1]
    - name: warning
      aliases: [warning, sev1]
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate severity alias config to fail")
	}
}
