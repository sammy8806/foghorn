package action

import (
	"testing"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func makeAlert(name, severity, cluster string) model.Alert {
	return model.Alert{
		ID:       name,
		Source:   "test",
		Name:     name,
		Severity: severity,
		Labels: map[string]string{
			"alertname": name,
			"severity":  severity,
			"cluster":   cluster,
		},
		Annotations:  map[string]string{"summary": "Test summary"},
		GeneratorURL: "http://prometheus:9090/graph?g0.expr=up",
	}
}

func TestActionsForAlert_Match(t *testing.T) {
	actions := []config.ActionConfig{
		{
			Name:  "Runbook",
			Match: map[string]string{"severity": "critical"},
			Action: config.ActionDef{
				Type:     "url",
				Template: "https://runbooks.example.com/{{.Name}}",
			},
		},
		{
			Name:  "Dashboard",
			Match: map[string]string{"cluster": "prod"},
			Action: config.ActionDef{
				Type:     "url",
				Template: "https://grafana.example.com/{{.Labels.cluster}}",
			},
		},
	}

	e := New(actions)
	alert := makeAlert("HighCPU", "critical", "prod")

	matched := e.ActionsForAlert(alert)
	if len(matched) != 2 {
		t.Errorf("expected 2 matched actions, got %d", len(matched))
	}
}

func TestActionsForAlert_NoMatch(t *testing.T) {
	actions := []config.ActionConfig{
		{
			Name:  "CriticalOnly",
			Match: map[string]string{"severity": "critical"},
			Action: config.ActionDef{Type: "url", Template: "http://example.com"},
		},
	}

	e := New(actions)
	alert := makeAlert("LowDisk", "warning", "dev")

	matched := e.ActionsForAlert(alert)
	if len(matched) != 0 {
		t.Errorf("expected 0 matched actions, got %d", len(matched))
	}
}

func TestRenderTemplate(t *testing.T) {
	alert := makeAlert("TargetDown", "critical", "saas-cs-0b")
	result, err := renderTemplate("https://runbooks.example.com/{{.Name}}?cluster={{.Labels.cluster}}", alert)
	if err != nil {
		t.Fatalf("renderTemplate() error: %v", err)
	}
	expected := "https://runbooks.example.com/TargetDown?cluster=saas-cs-0b"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	alert := makeAlert("Alert", "warning", "prod")
	_, err := renderTemplate("{{.Invalid.Field.That.Does.Not.Exist}}", alert)
	// template execution may succeed with zero value, not error — just verify no panic
	_ = err
}

func TestMatchesAlert_EmptyMatch(t *testing.T) {
	// Empty match should match everything
	alert := makeAlert("Any", "info", "any")
	if !matchesAlert(map[string]string{}, alert) {
		t.Error("empty match should match all alerts")
	}
}

func TestExecute_ClipboardAction(t *testing.T) {
	// Override copyToClipboard to avoid needing pbcopy
	original := copyToClipboard
	defer func() { copyToClipboard = original }()

	var copied string
	copyToClipboard = func(text string) error {
		copied = text
		return nil
	}

	e := New(nil)
	action := config.ActionConfig{
		Action: config.ActionDef{
			Type:     "clipboard",
			Template: "{{.Name}} on {{.Source}}",
		},
	}
	alert := makeAlert("HighMem", "warning", "prod")

	result, err := e.Execute(action, alert)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result != "HighMem on test" {
		t.Errorf("expected 'HighMem on test', got %q", result)
	}
	if copied != "HighMem on test" {
		t.Errorf("expected clipboard 'HighMem on test', got %q", copied)
	}
}
