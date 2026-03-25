package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
sources:
  - name: test-am
    type: alertmanager
    url: http://localhost:9093
    auth:
      type: basic
      username: admin
      password: secret
    poll_interval: 30s

display:
  visible_labels: [alertname, severity]
  visible_annotations: [summary]
  group_by: [cluster]
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
	if cfg.UI.PopupWidth != 800 {
		t.Errorf("expected popup_width 800, got %d", cfg.UI.PopupWidth)
	}
	if len(cfg.Resolvers) != 1 {
		t.Fatalf("expected 1 resolver, got %d", len(cfg.Resolvers))
	}
	if cfg.Resolvers[0].Field != "label:cluster" {
		t.Fatalf("expected resolver field label:cluster, got %q", cfg.Resolvers[0].Field)
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
