package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/provider"
	"foghorn/internal/state"
)

type stubProvider struct {
	name            string
	supportsSilence bool
	fetchFn         func(context.Context) ([]model.Alert, error)
}

func (s stubProvider) Name() string          { return s.name }
func (s stubProvider) Type() string          { return "stub" }
func (s stubProvider) SupportsSilence() bool { return s.supportsSilence }
func (s stubProvider) Fetch(ctx context.Context) ([]model.Alert, error) {
	if s.fetchFn != nil {
		return s.fetchFn(ctx)
	}
	return nil, nil
}
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

	diff := app.ResolveDiff(context.Background(), model.Diff{
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

func TestRefreshAlertsForcesImmediateFetch(t *testing.T) {
	app := NewApp(&config.Config{}, state.New())

	calls := 0
	app.SetProviders(map[string]provider.Provider{
		"am": stubProvider{
			name: "am",
			fetchFn: func(context.Context) ([]model.Alert, error) {
				calls++
				return []model.Alert{{
					ID:     "a1",
					Source: "am",
					Name:   "CPUHigh",
				}}, nil
			},
		},
	})

	if err := app.RefreshAlerts(); err != nil {
		t.Fatalf("RefreshAlerts returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 fetch, got %d", calls)
	}

	alerts := app.GetAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert after refresh, got %d", len(alerts))
	}

	health := app.GetSourcesHealth()
	if len(health) != 1 {
		t.Fatalf("expected 1 health entry after refresh, got %d", len(health))
	}
	if !health[0].OK {
		t.Fatalf("expected source to be healthy after refresh, got %#v", health[0])
	}
	if health[0].LastPoll.IsZero() {
		t.Fatalf("expected refresh to update last poll timestamp, got %#v", health[0])
	}
}

func TestRefreshAlertsRecordsFetchFailures(t *testing.T) {
	app := NewApp(&config.Config{}, state.New())
	app.SetProviders(map[string]provider.Provider{
		"am": stubProvider{
			name: "am",
			fetchFn: func(context.Context) ([]model.Alert, error) {
				return nil, errors.New("boom")
			},
		},
	})

	if err := app.RefreshAlerts(); err != nil {
		t.Fatalf("RefreshAlerts returned error: %v", err)
	}

	health := app.GetSourcesHealth()
	if len(health) != 1 {
		t.Fatalf("expected 1 health entry after refresh, got %d", len(health))
	}
	if health[0].OK {
		t.Fatalf("expected failed refresh to mark source unhealthy, got %#v", health[0])
	}
	if health[0].LastError != "boom" {
		t.Fatalf("expected fetch error to be recorded, got %#v", health[0])
	}
	if health[0].LastPoll.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("expected recent last poll timestamp, got %#v", health[0])
	}
}
