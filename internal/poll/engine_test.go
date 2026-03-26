package poll

import (
	"context"
	"errors"
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
	onCallErr  error
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
func (m *mockProvider) FetchOnCall(_ context.Context) (*model.OnCallStatus, error) {
	if m.onCallErr != nil {
		return nil, m.onCallErr
	}
	return &model.OnCallStatus{
		ScheduleID:   "default",
		ScheduleName: "default",
		Users: []model.OnCallUser{
			{Name: "Alice Example", Email: "alice@example.com"},
		},
	}, nil
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
	if onCalls := store.OnCalls(); len(onCalls) != 1 {
		t.Fatalf("expected 1 on-call status, got %d", len(onCalls))
	}
}

func TestEngineRecordsOnCallFailureInHealth(t *testing.T) {
	store := state.New()
	mp := &mockProvider{
		name: "test",
		alerts: []model.Alert{
			{ID: "a1", Source: "test", Name: "TestAlert", Severity: "warning", State: "active",
				Labels: map[string]string{"alertname": "TestAlert"}},
		},
		onCallErr: errors.New("schedule lookup failed"),
	}

	sources := []config.SourceConfig{
		{Name: "test", PollInterval: 100 * time.Millisecond},
	}

	e := New(store, sources, func(source string, p Provider) Provider { return mp })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	diffCh := e.Start(ctx)
	select {
	case <-diffCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for poll")
	}

	health := store.SourcesHealth()
	if len(health) != 1 {
		t.Fatalf("expected 1 health entry, got %d", len(health))
	}
	if health[0].OK {
		t.Fatal("expected source health to be failing")
	}
	if health[0].LastError == "" {
		t.Fatal("expected source health error message")
	}

	all := store.All()
	if len(all) != 1 {
		t.Fatalf("expected alerts to still be stored, got %d", len(all))
	}
	if onCalls := store.OnCalls(); len(onCalls) != 0 {
		t.Fatalf("expected no on-call data after failure, got %d entries", len(onCalls))
	}
}
