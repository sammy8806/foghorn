//go:build linux && linux_tray

package tray

import (
	"sync"

	"github.com/getlantern/systray"
)

func Supported() bool {
	return true
}

func StartHiddenByDefault() bool {
	return false
}

type linuxTray struct {
	mu       sync.Mutex
	ready    bool
	disposed bool
	icon     []byte
	tooltip  string
	stopCh   chan struct{}
	stopOnce sync.Once
}

func newPlatformTray(m *Manager) platformTray {
	lt := &linuxTray{
		stopCh: make(chan struct{}),
	}

	systray.Register(func() {
		lt.mu.Lock()
		if lt.disposed {
			lt.mu.Unlock()
			return
		}
		lt.ready = true
		icon := lt.icon
		tooltip := lt.tooltip
		lt.mu.Unlock()

		systray.SetTitle("Foghorn")
		if len(icon) > 0 {
			systray.SetIcon(icon)
		}
		if tooltip != "" {
			systray.SetTooltip(tooltip)
		}

		mShow := systray.AddMenuItem("Show or Hide Window", "Toggle the Foghorn window")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit Foghorn", "Quit Foghorn")

		go func() {
			for {
				select {
				case <-mShow.ClickedCh:
					m.handleClick()
				case <-mQuit.ClickedCh:
					m.handleQuit()
				case <-lt.stopCh:
					return
				}
			}
		}()
	}, nil)

	return lt
}

func (l *linuxTray) update(icon []byte, tooltip string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.disposed {
		return nil
	}

	l.icon = icon
	l.tooltip = tooltip
	if l.ready {
		if len(icon) > 0 {
			systray.SetIcon(icon)
		}
		systray.SetTooltip(tooltip)
	}
	return nil
}

func (l *linuxTray) dispose() {
	l.mu.Lock()
	if l.disposed {
		l.mu.Unlock()
		return
	}
	l.disposed = true
	l.mu.Unlock()

	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
	systray.Quit()
}
