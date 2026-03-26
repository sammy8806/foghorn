package main

import (
	"testing"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/state"
)

func TestResolveDiffResolvesFrontendPayloads(t *testing.T) {
	app := NewApp(&config.Config{
		Resolvers: []config.ResolverConfig{
			{
				Name:    "cluster-name",
				Field:   "label:cluster",
				Command: "sh",
				Args:    []string{"-c", "printf '%s' \"$1-resolved\"", "--", "{{.Value}}"},
			},
		},
	}, state.New())

	diff := app.ResolveDiff(model.Diff{
		Resolved: []model.Alert{
			{
				ID:     "a1",
				Source: "src1",
				Labels: map[string]string{"cluster": "customer-1"},
			},
		},
	})

	if len(diff.Resolved) != 1 {
		t.Fatalf("expected 1 resolved alert, got %d", len(diff.Resolved))
	}
	if got := diff.Resolved[0].ResolvedLabels["cluster"]; got != "customer-1-resolved" {
		t.Fatalf("expected resolved cluster label in diff payload, got %q", got)
	}
}
