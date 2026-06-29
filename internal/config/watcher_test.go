package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const minimalConfig = `
sources:
  - name: test
    type: alertmanager
    url: http://localhost:9093
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

func TestWatcherDetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(minimalConfig), 0644); err != nil {
		t.Fatal(err)
	}

	changed := make(chan *Config, 1)
	stop, err := Watch(path, func(cfg *Config) {
		changed <- cfg
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}
	defer stop()

	// Write an updated config
	time.Sleep(100 * time.Millisecond)
	updated := minimalConfig + "\n# updated\n"
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case cfg := <-changed:
		if cfg == nil {
			t.Error("expected non-nil config from watcher")
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for config change notification")
	}
}

func TestWatcherReloadsUIScale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(minimalConfig), 0644); err != nil {
		t.Fatal(err)
	}

	changed := make(chan *Config, 1)
	stop, err := Watch(path, func(cfg *Config) {
		changed <- cfg
	})
	if err != nil {
		t.Fatalf("Watch() error: %v", err)
	}
	defer stop()

	updated := `sources:
  - name: test
    type: alertmanager
    url: http://localhost:9093
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
  scale:
    factor: 1.5
    mode: interface
    apply_to_popup: false
`
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case cfg := <-changed:
		if cfg.UI.Scale.Factor != 1.5 {
			t.Fatalf("expected scale factor 1.5, got %v", cfg.UI.Scale.Factor)
		}
		if cfg.UI.Scale.Mode != "interface" {
			t.Fatalf("expected scale mode interface, got %q", cfg.UI.Scale.Mode)
		}
		if cfg.UI.Scale.ApplyToPopup {
			t.Fatal("expected apply_to_popup false")
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for config change notification")
	}
}
