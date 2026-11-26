//go:build linux && linux_tray

package tray

import "testing"

func TestLinuxTraySetsIconOnlyWhenItChanges(t *testing.T) {
	var shown [][]byte
	origIcon, origTooltip := setTrayIcon, setTrayTooltip
	setTrayIcon = func(icon []byte) { shown = append(shown, icon) }
	setTrayTooltip = func(string) {}
	t.Cleanup(func() { setTrayIcon, setTrayTooltip = origIcon, origTooltip })

	lt := &linuxTray{ready: true, stopCh: make(chan struct{})}
	steps := []struct {
		icon      []byte
		wantCalls int
	}{
		{IconGreen, 1},
		{IconGreen, 1},  // unchanged: no new temp file
		{IconYellow, 2}, // changed
		{nil, 2},        // empty: keep the current icon
		{IconYellow, 2},
		{IconGreen, 3},
	}
	for i, step := range steps {
		if err := lt.update(step.icon, "Foghorn"); err != nil {
			t.Fatalf("step %d: update: %v", i, err)
		}
		if len(shown) != step.wantCalls {
			t.Fatalf("step %d: SetIcon called %d times, want %d", i, len(shown), step.wantCalls)
		}
	}
}
