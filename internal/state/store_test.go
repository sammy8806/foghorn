package state

import (
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func makeAlert(id, source, name, severity, state string) model.Alert {
	return model.Alert{
		ID:       id,
		Source:   source,
		Name:     name,
		Severity: severity,
		State:    state,
		Labels:   map[string]string{"alertname": name, "severity": severity},
		StartsAt: time.Now(),
	}
}

func TestNewAlerts(t *testing.T) {
	s := New()
	alerts := []model.Alert{
		makeAlert("a1", "src1", "HighCPU", "warning", "active"),
		makeAlert("a2", "src1", "DiskFull", "critical", "active"),
	}

	diff := s.Update("src1", alerts)

	if len(diff.New) != 2 {
		t.Fatalf("expected 2 new, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Fatalf("expected 0 resolved, got %d", len(diff.Resolved))
	}
}

func TestResolvedAlerts(t *testing.T) {
	s := New()
	s.Update("src1", []model.Alert{
		makeAlert("a1", "src1", "HighCPU", "warning", "active"),
		makeAlert("a2", "src1", "DiskFull", "critical", "active"),
	})

	// Second poll: only a1 remains
	diff := s.Update("src1", []model.Alert{
		makeAlert("a1", "src1", "HighCPU", "warning", "active"),
	})

	if len(diff.New) != 0 {
		t.Fatalf("expected 0 new, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(diff.Resolved))
	}
	if diff.Resolved[0].Name != "DiskFull" {
		t.Errorf("expected resolved alert 'DiskFull', got %q", diff.Resolved[0].Name)
	}
}

func TestChangedAlerts(t *testing.T) {
	s := New()
	s.Update("src1", []model.Alert{
		makeAlert("a1", "src1", "HighCPU", "warning", "active"),
	})

	changed := makeAlert("a1", "src1", "HighCPU", "warning", "suppressed")
	diff := s.Update("src1", []model.Alert{changed})

	if len(diff.Changed) != 1 {
		t.Fatalf("expected 1 changed, got %d", len(diff.Changed))
	}
}

func TestSeverityCounts(t *testing.T) {
	s := New()
	s.Update("src1", []model.Alert{
		makeAlert("a1", "src1", "Alert1", "critical", "active"),
		makeAlert("a2", "src1", "Alert2", "warning", "active"),
		makeAlert("a3", "src1", "Alert3", "warning", "active"),
	})

	counts := s.SeverityCounts()
	if counts["critical"] != 1 {
		t.Errorf("expected 1 critical, got %d", counts["critical"])
	}
	if counts["warning"] != 2 {
		t.Errorf("expected 2 warning, got %d", counts["warning"])
	}
}

func TestSeverityCountsWithAliases(t *testing.T) {
	s := New()
	severities, err := config.NormalizeSeverityConfig(config.SeverityConfig{
		Default: "unknown",
		Levels: []config.SeverityLevel{
			{Name: "critical", Aliases: []string{"critical", "sev1"}},
			{Name: "warning", Aliases: []string{"warning", "sev2"}},
			{Name: "unknown", Aliases: []string{"unknown"}},
		},
	})
	if err != nil {
		t.Fatalf("NormalizeSeverityConfig() error: %v", err)
	}
	s.SetSeverityConfig(severities)
	s.Update("src1", []model.Alert{
		makeAlert("a1", "src1", "Alert1", "sev1", "active"),
		makeAlert("a2", "src1", "Alert2", "sev2", "active"),
		makeAlert("a3", "src1", "Alert3", "mystery", "active"),
	})

	counts := s.SeverityCounts()
	if counts["critical"] != 1 {
		t.Errorf("expected 1 critical, got %d", counts["critical"])
	}
	if counts["warning"] != 1 {
		t.Errorf("expected 1 warning, got %d", counts["warning"])
	}
	if counts["unknown"] != 1 {
		t.Errorf("expected 1 unknown, got %d", counts["unknown"])
	}
}

func TestAllAlerts(t *testing.T) {
	s := New()
	s.Update("src1", []model.Alert{
		makeAlert("a1", "src1", "Alert1", "critical", "active"),
	})
	s.Update("src2", []model.Alert{
		makeAlert("b1", "src2", "Alert2", "warning", "active"),
	})

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 total alerts, got %d", len(all))
	}
}

func TestMultiSourceIsolation(t *testing.T) {
	s := New()
	s.Update("src1", []model.Alert{
		makeAlert("a1", "src1", "Alert1", "critical", "active"),
	})
	s.Update("src2", []model.Alert{
		makeAlert("b1", "src2", "Alert2", "warning", "active"),
	})

	// Clearing src1 should not affect src2
	diff := s.Update("src1", []model.Alert{})

	if len(diff.Resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(diff.Resolved))
	}
	all := s.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 remaining alert from src2, got %d", len(all))
	}
}
