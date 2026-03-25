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
	unsilenced []string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Type() string { return "mock" }
func (m *mockProvider) Fetch(_ context.Context) ([]model.Alert, error) { return nil, nil }
func (m *mockProvider) Silence(_ context.Context, req model.SilenceRequest) (string, error) {
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

func TestSilenceAlert(t *testing.T) {
	mp := &mockProvider{name: "test-am", silenceID: "silence-abc"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	alert := model.Alert{
		ID:     "fp123",
		Source: "test-am",
		Name:   "HighCPU",
		Labels: map[string]string{"alertname": "HighCPU", "severity": "warning", "cluster": "prod"},
	}

	id, err := mgr.SilenceAlert(context.Background(), alert, "2h", "test silence")
	if err != nil {
		t.Fatalf("SilenceAlert() error: %v", err)
	}
	if id != "silence-abc" {
		t.Errorf("expected silence ID 'silence-abc', got %q", id)
	}
	if mp.lastReq.Comment != "test silence" {
		t.Errorf("expected comment 'test silence', got %q", mp.lastReq.Comment)
	}
	if mp.lastReq.EndsAt.Sub(mp.lastReq.StartsAt) < 2*time.Hour-time.Second {
		t.Errorf("expected 2h duration, got %v", mp.lastReq.EndsAt.Sub(mp.lastReq.StartsAt))
	}
	if len(mp.lastReq.Matchers) == 0 {
		t.Error("expected matchers from alert labels, got none")
	}
}

func TestSilenceUnknownSource(t *testing.T) {
	mgr := New(map[string]provider.Provider{})

	alert := model.Alert{Source: "nonexistent", Name: "Alert"}
	_, err := mgr.SilenceAlert(context.Background(), alert, "1h", "")
	if err == nil {
		t.Error("expected error for unknown source, got nil")
	}
}

func TestUnsilence(t *testing.T) {
	mp := &mockProvider{name: "test-am"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	err := mgr.Unsilence(context.Background(), "test-am", "silence-xyz")
	if err != nil {
		t.Fatalf("Unsilence() error: %v", err)
	}
	if len(mp.unsilenced) != 1 || mp.unsilenced[0] != "silence-xyz" {
		t.Errorf("expected unsilenced [silence-xyz], got %v", mp.unsilenced)
	}
}

func TestInvalidDuration(t *testing.T) {
	mp := &mockProvider{name: "test-am"}
	mgr := New(map[string]provider.Provider{"test-am": mp})

	alert := model.Alert{Source: "test-am", Name: "Alert", Labels: map[string]string{}}
	_, err := mgr.SilenceAlert(context.Background(), alert, "not-a-duration", "")
	if err == nil {
		t.Error("expected error for invalid duration, got nil")
	}
}
