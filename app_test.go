package main

import (
	"context"
	"testing"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/provider"
	"foghorn/internal/state"
)

type stubProvider struct {
	name            string
	supportsSilence bool
}

func (s stubProvider) Name() string                                                  { return s.name }
func (s stubProvider) Type() string                                                  { return "stub" }
func (s stubProvider) SupportsSilence() bool                                         { return s.supportsSilence }
func (s stubProvider) Fetch(context.Context) ([]model.Alert, error)                  { return nil, nil }
func (s stubProvider) Silence(context.Context, model.SilenceRequest) (string, error) { return "", nil }
func (s stubProvider) Unsilence(context.Context, string) error                       { return nil }
func (s stubProvider) Health(context.Context) model.ProviderHealth                   { return model.ProviderHealth{} }

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

func TestGetSourceCapabilitiesReflectsProviders(t *testing.T) {
	app := NewApp(&config.Config{}, state.New())
	app.SetProviders(map[string]provider.Provider{
		"am": stubProvider{name: "am", supportsSilence: true},
		"bs": stubProvider{name: "bs", supportsSilence: false},
	})

	got := app.GetSourceCapabilities()
	if !got["am"].SupportsSilence {
		t.Fatalf("expected am to support silence, got %#v", got["am"])
	}
	if got["bs"].SupportsSilence {
		t.Fatalf("expected bs not to support silence, got %#v", got["bs"])
	}
}
