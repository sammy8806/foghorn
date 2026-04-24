package tray

import (
	"bytes"
	"testing"

	"foghorn/internal/model"
)

func newTestManager() *Manager {
	return NewManager(nil, nil)
}

func makeBreakdown(active, silenced map[string]int) model.SeverityBreakdown {
	a := model.SeverityCounts{"critical": 0, "warning": 0, "info": 0, "unknown": 0}
	s := model.SeverityCounts{"critical": 0, "warning": 0, "info": 0, "unknown": 0}
	for k, v := range active {
		a[k] = v
	}
	for k, v := range silenced {
		s[k] = v
	}
	return model.SeverityBreakdown{Active: a, Silenced: s}
}

func TestTrayTooltipStartingUp(t *testing.T) {
	m := newTestManager()
	if got := m.Tooltip(); got != "Foghorn - Starting up" {
		t.Errorf("expected starting-up tooltip, got %q", got)
	}
}

func TestTrayTooltipAllClear(t *testing.T) {
	m := newTestManager()
	m.UpdateState(makeBreakdown(nil, nil))

	if got := m.Tooltip(); got != "Foghorn - All clear" {
		t.Errorf("got %q", got)
	}
}

func TestTrayTooltipActiveOnly(t *testing.T) {
	m := newTestManager()
	m.UpdateState(makeBreakdown(map[string]int{"critical": 3, "warning": 1}, nil))

	want := "Foghorn - 3 critical, 1 warning"
	if got := m.Tooltip(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTrayTooltipSilencedOnly(t *testing.T) {
	m := newTestManager()
	m.UpdateState(makeBreakdown(nil, map[string]int{"critical": 2, "warning": 1}))

	want := "Foghorn - All clear (2 critical, 1 warning silenced)"
	if got := m.Tooltip(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTrayTooltipActiveAndSilenced(t *testing.T) {
	m := newTestManager()
	m.UpdateState(makeBreakdown(
		map[string]int{"critical": 3, "warning": 1},
		map[string]int{"critical": 2, "warning": 1},
	))

	want := "Foghorn - 3 critical, 1 warning (2 critical, 1 warning silenced)"
	if got := m.Tooltip(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTrayIconIgnoresSilencedCritical(t *testing.T) {
	m := newTestManager()
	// Only silenced criticals exist; icon must stay green.
	m.UpdateState(makeBreakdown(nil, map[string]int{"critical": 3}))

	if got := m.icon(); !bytes.Equal(got, IconGreen) {
		t.Errorf("expected IconGreen when only silenced criticals exist")
	}
}

func TestTrayIconUsesActiveHighestSeverity(t *testing.T) {
	m := newTestManager()
	m.UpdateState(makeBreakdown(
		map[string]int{"warning": 1}, // highest active is warning
		map[string]int{"critical": 5}, // silenced criticals must be ignored
	))

	if got := m.icon(); !bytes.Equal(got, IconYellow) {
		t.Errorf("expected IconYellow when only warning is active")
	}
}

func TestTrayIconRedOnActiveCritical(t *testing.T) {
	m := newTestManager()
	m.UpdateState(makeBreakdown(map[string]int{"critical": 1}, nil))

	if got := m.icon(); !bytes.Equal(got, IconRed) {
		t.Errorf("expected IconRed when active critical present")
	}
}

func TestTrayIconGreyBeforeReady(t *testing.T) {
	m := newTestManager()
	if got := m.icon(); !bytes.Equal(got, IconGrey) {
		t.Errorf("expected IconGrey before first UpdateState")
	}
}
