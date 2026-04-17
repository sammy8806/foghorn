package silence

import (
	"context"
	"testing"
	"time"

	"foghorn/internal/model"
	"foghorn/internal/provider"
)

type mockProvider struct {
	name       string
	silenceID  string
	lastReq    model.SilenceRequest
	called     bool
	unsilenced []string
}

func (m *mockProvider) Name() string                                   { return m.name }
func (m *mockProvider) Type() string                                   { return "mock" }
func (m *mockProvider) SupportsSilence() bool                          { return true }
func (m *mockProvider) Fetch(_ context.Context) ([]model.Alert, error) { return nil, nil }
func (m *mockProvider) Silence(_ context.Context, req model.SilenceRequest) (string, error) {
	m.called = true
	m.lastReq = req
	return m.silenceID, nil
}
func (m *mockProvider) Unsilence(_ context.Context, id string) error {
	m.unsilenced = append(m.unsilenced, id)
	return nil
}
func (m *mockProvider) Health(_ context.Context) model.ProviderHealth {
	return model.ProviderHealth{Connected: true}
}

var _ provider.Provider = (*mockProvider)(nil)

func sampleMatchers() []model.Matcher {
	return []model.Matcher{
		{Name: "alertname", Value: "HighCPU", IsRegex: false, IsEqual: true},
		{Name: "severity", Value: "warning", IsRegex: false, IsEqual: true},
	}
}

func TestCreateSilence(t *testing.T) {
	mp := &mockProvider{name: "test-am", silenceID: "silence-abc"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	id, err := mgr.CreateSilence(context.Background(), "test-am", sampleMatchers(), "2h", "alice", "test silence", "fallback")
	if err != nil {
		t.Fatalf("CreateSilence() error: %v", err)
	}
	if id != "silence-abc" {
		t.Errorf("expected silence ID 'silence-abc', got %q", id)
	}
	if mp.lastReq.ID != "" {
		t.Errorf("expected empty ID on create, got %q", mp.lastReq.ID)
	}
	if mp.lastReq.Comment != "test silence" {
		t.Errorf("expected comment 'test silence', got %q", mp.lastReq.Comment)
	}
	if mp.lastReq.CreatedBy != "alice" {
		t.Errorf("expected createdBy 'alice', got %q", mp.lastReq.CreatedBy)
	}
	if mp.lastReq.EndsAt.Sub(mp.lastReq.StartsAt) < 2*time.Hour-time.Second {
		t.Errorf("expected 2h duration, got %v", mp.lastReq.EndsAt.Sub(mp.lastReq.StartsAt))
	}
	if len(mp.lastReq.Matchers) != 2 {
		t.Errorf("expected 2 matchers, got %d", len(mp.lastReq.Matchers))
	}
}

func TestCreateSilenceUnknownSource(t *testing.T) {
	mgr := New(map[string]provider.Provider{})
	_, err := mgr.CreateSilence(context.Background(), "nonexistent", sampleMatchers(), "1h", "", "", "")
	if err == nil {
		t.Error("expected error for unknown source, got nil")
	}
}

func TestCreateSilenceInvalidDuration(t *testing.T) {
	mp := &mockProvider{name: "test-am"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	_, err := mgr.CreateSilence(context.Background(), "test-am", sampleMatchers(), "not-a-duration", "", "", "")
	if err == nil {
		t.Error("expected error for invalid duration, got nil")
	}
	if mp.called {
		t.Error("expected provider.Silence to not be called when duration is invalid")
	}
}

func TestCreateSilenceFallsBackToDefaultCreatedBy(t *testing.T) {
	mp := &mockProvider{name: "test-am", silenceID: "silence-abc"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	_, err := mgr.CreateSilence(context.Background(), "test-am", sampleMatchers(), "2h", "", "test silence", "configured-user")
	if err != nil {
		t.Fatalf("CreateSilence() error: %v", err)
	}
	if mp.lastReq.CreatedBy != "configured-user" {
		t.Errorf("expected createdBy 'configured-user', got %q", mp.lastReq.CreatedBy)
	}
}

func TestUpdateSilenceForwardsID(t *testing.T) {
	mp := &mockProvider{name: "test-am", silenceID: "silence-abc"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	err := mgr.UpdateSilence(context.Background(), "test-am", "sil-123", sampleMatchers(), "1h", "alice", "updated", "fallback")
	if err != nil {
		t.Fatalf("UpdateSilence() error: %v", err)
	}
	if mp.lastReq.ID != "sil-123" {
		t.Errorf("expected ID 'sil-123' forwarded, got %q", mp.lastReq.ID)
	}
	if mp.lastReq.Comment != "updated" {
		t.Errorf("expected comment 'updated', got %q", mp.lastReq.Comment)
	}
	if mp.lastReq.EndsAt.Sub(mp.lastReq.StartsAt) < time.Hour-time.Second {
		t.Errorf("expected 1h duration, got %v", mp.lastReq.EndsAt.Sub(mp.lastReq.StartsAt))
	}
}

func TestUpdateSilenceRequiresID(t *testing.T) {
	mp := &mockProvider{name: "test-am"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	err := mgr.UpdateSilence(context.Background(), "test-am", "", sampleMatchers(), "1h", "", "", "")
	if err == nil {
		t.Error("expected error for empty silence ID, got nil")
	}
	if mp.called {
		t.Error("expected provider.Silence to not be called when ID is empty")
	}
}

func TestUpdateSilenceUnknownSource(t *testing.T) {
	mgr := New(map[string]provider.Provider{})
	err := mgr.UpdateSilence(context.Background(), "nope", "sil-1", sampleMatchers(), "1h", "", "", "")
	if err == nil {
		t.Error("expected error for unknown source, got nil")
	}
}

func TestUpdateSilenceInvalidDuration(t *testing.T) {
	mp := &mockProvider{name: "test-am"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	err := mgr.UpdateSilence(context.Background(), "test-am", "sil-1", sampleMatchers(), "bogus", "", "", "")
	if err == nil {
		t.Error("expected error for invalid duration, got nil")
	}
	if mp.called {
		t.Error("expected provider.Silence to not be called when duration is invalid")
	}
}

func TestUnsilence(t *testing.T) {
	mp := &mockProvider{name: "test-am"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	if err := mgr.Unsilence(context.Background(), "test-am", "silence-xyz"); err != nil {
		t.Fatalf("Unsilence() error: %v", err)
	}
	if len(mp.unsilenced) != 1 || mp.unsilenced[0] != "silence-xyz" {
		t.Errorf("expected unsilenced [silence-xyz], got %v", mp.unsilenced)
	}
}

func TestUnsilenceUnknownSource(t *testing.T) {
	mgr := New(map[string]provider.Provider{})
	err := mgr.Unsilence(context.Background(), "nope", "sil-1")
	if err == nil {
		t.Error("expected error for unknown source, got nil")
	}
}
