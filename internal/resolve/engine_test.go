package resolve

import (
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func TestResolveAlertLabelCommand(t *testing.T) {
	engine := New([]config.ResolverConfig{
		{
			Name:    "cluster",
			Field:   "label:cluster",
			Command: "sh",
			Args:    []string{"-c", "printf '%s' \"$1-resolved\"", "--", "{{.Value}}"},
			Timeout: time.Second,
		},
	})

	alert := model.Alert{
		Name:   "TargetDown",
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	resolved := engine.ResolveAlert(alert)
	if got := resolved.ResolvedLabels["cluster"]; got != "saas-cs-0b-resolved" {
		t.Fatalf("expected resolved cluster label, got %q", got)
	}
	if got := resolved.Labels["cluster"]; got != "saas-cs-0b" {
		t.Fatalf("expected raw cluster label to stay untouched, got %q", got)
	}
}

func TestResolveAlertFieldCommand(t *testing.T) {
	engine := New([]config.ResolverConfig{
		{
			Name:    "source",
			Field:   "field:source",
			Command: "sh",
			Args:    []string{"-c", "printf '%s/%s' \"$1\" \"$2\"", "--", "{{.Value}}", "{{.Labels.cluster}}"},
			Timeout: time.Second,
		},
	})

	alert := model.Alert{
		Source: "prod-am",
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	resolved := engine.ResolveAlert(alert)
	if got := resolved.ResolvedFields["source"]; got != "prod-am/saas-cs-0b" {
		t.Fatalf("expected resolved source field, got %q", got)
	}
}
