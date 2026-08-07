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

func (m *mockProvider) Name() string          { return m.name }
func (m *mockProvider) Type() string          { return "mock" }
func (m *mockProvider) SupportsSilence() bool { return true }
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
			{Name: "on-call primary", Email: "primary-oncall@example.test"},
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

func TestEngineRefreshNowTriggersImmediatePoll(t *testing.T) {
	store := state.New()
	mp := &mockProvider{name: "test"}

	// Long interval so the ticker won't fire during the test; only the initial
	// poll and the RefreshNow-triggered poll should occur.
	sources := []config.SourceConfig{
		{Name: "test", PollInterval: time.Hour},
	}
	e := New(store, sources, func(string, Provider) Provider { return mp })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	diffCh := e.Start(ctx)

	// Drain the initial poll.
	select {
	case <-diffCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial poll")
	}

	e.RefreshNow()

	select {
	case <-diffCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RefreshNow poll")
	}

	if got := mp.fetchCount.Load(); got < 2 {
		t.Fatalf("expected at least 2 fetches after RefreshNow, got %d", got)
	}
}

func TestEnginePollTimeoutDoesNotHang(t *testing.T) {
	store := state.New()
	mp := &blockingProvider{name: "slow"}

	sources := []config.SourceConfig{
		{Name: "slow", PollInterval: time.Hour, Timeout: 50 * time.Millisecond},
	}
	e := New(store, sources, func(string, Provider) Provider { return mp })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	diffCh := e.Start(ctx)

	select {
	case <-diffCh:
	case <-time.After(2 * time.Second):
		t.Fatal("poll hung past the per-source timeout")
	}

	health := store.SourcesHealth()
	if len(health) != 1 || health[0].OK {
		t.Fatalf("expected timed-out source to be unhealthy, got %#v", health)
	}
}

func TestEngineEnrichesSilences(t *testing.T) {
	store := state.New()
	sp := &silenceProvider{
		name: "am",
		alerts: []model.Alert{
			{ID: "a1", Source: "am", Name: "Down", SilencedBy: []string{"sil-1"},
				Labels: map[string]string{"alertname": "Down"}},
		},
		silences: []model.SilenceInfo{
			{ID: "sil-1", CreatedBy: "oncall", Comment: "maintenance"},
		},
	}
	sources := []config.SourceConfig{
		{Name: "am", PollInterval: time.Hour},
	}
	e := New(store, sources, func(string, Provider) Provider { return sp })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	diffCh := e.Start(ctx)
	select {
	case <-diffCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for poll")
	}

	all := store.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(all))
	}
	if len(all[0].Silences) != 1 || all[0].Silences[0].ID != "sil-1" {
		t.Fatalf("expected alert enriched with silence sil-1, got %#v", all[0].Silences)
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

// blockingProvider's Fetch blocks until the context is cancelled, so it only
// returns once the per-source timeout fires.
type blockingProvider struct {
	name string
}

func (b *blockingProvider) Name() string          { return b.name }
func (b *blockingProvider) Type() string          { return "mock" }
func (b *blockingProvider) SupportsSilence() bool { return false }
func (b *blockingProvider) Fetch(ctx context.Context) ([]model.Alert, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (b *blockingProvider) Silence(context.Context, model.SilenceRequest) (string, error) {
	return "", nil
}
func (b *blockingProvider) Unsilence(context.Context, string) error { return nil }
func (b *blockingProvider) Health(context.Context) model.ProviderHealth {
	return model.ProviderHealth{}
}

// silenceProvider implements provider.SilenceProvider so the engine can enrich
// alerts with silence detail.
type silenceProvider struct {
	name     string
	alerts   []model.Alert
	silences []model.SilenceInfo
}

func (s *silenceProvider) Name() string          { return s.name }
func (s *silenceProvider) Type() string          { return "mock" }
func (s *silenceProvider) SupportsSilence() bool { return true }
func (s *silenceProvider) Fetch(context.Context) ([]model.Alert, error) {
	return s.alerts, nil
}
func (s *silenceProvider) Silence(context.Context, model.SilenceRequest) (string, error) {
	return "", nil
}
func (s *silenceProvider) Unsilence(context.Context, string) error { return nil }
func (s *silenceProvider) Health(context.Context) model.ProviderHealth {
	return model.ProviderHealth{}
}
func (s *silenceProvider) FetchSilences(context.Context) ([]model.SilenceInfo, error) {
	return s.silences, nil
}
