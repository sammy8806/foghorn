package poll

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/state"
)

type mockProvider struct {
	name       string
	fetchCount atomic.Int32
	alerts     []model.Alert
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Type() string { return "mock" }
func (m *mockProvider) Fetch(_ context.Context) ([]model.Alert, error) {
	m.fetchCount.Add(1)
	return m.alerts, nil
}
func (m *mockProvider) Silence(_ context.Context, _ model.SilenceRequest) (string, error) {
	return "", nil
}
func (m *mockProvider) Unsilence(_ context.Context, _ string) error { return nil }
func (m *mockProvider) Health(_ context.Context) model.ProviderHealth {
	return model.ProviderHealth{Connected: true}
}

func TestEnginePolls(t *testing.T) {
	store := state.New()
	mp := &mockProvider{
		name: "test",
		alerts: []model.Alert{
			{ID: "a1", Source: "test", Name: "TestAlert", Severity: "warning", State: "active",
				Labels: map[string]string{"alertname": "TestAlert"}},
		},
	}

	sources := []config.SourceConfig{
		{Name: "test", PollInterval: 100 * time.Millisecond},
	}

	e := New(store, sources, func(source string, p Provider) Provider { return mp })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	diffCh := e.Start(ctx)

	// Wait for at least 2 polls
	count := 0
	timeout := time.After(2 * time.Second)
	for count < 2 {
		select {
		case <-diffCh:
			count++
		case <-timeout:
			t.Fatalf("timed out waiting for polls, got %d", count)
		}
	}

	if mp.fetchCount.Load() < 2 {
		t.Errorf("expected at least 2 fetches, got %d", mp.fetchCount.Load())
	}

	all := store.All()
	if len(all) != 1 {
		t.Errorf("expected 1 alert in store, got %d", len(all))
	}
}
