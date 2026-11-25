//go:build linux && linux_tray

package tray

import (
	"bytes"
	"sync"

	"github.com/getlantern/systray"
)

// The systray calls, replaceable in tests.
var (
	setTrayIcon    = systray.SetIcon
	setTrayTooltip = systray.SetTooltip
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
	// shownIcon is the icon last handed to systray. systray writes every icon to a new temp
	// file and deletes the previous one, so setting an unchanged icon on each poll makes the
	// GNOME AppIndicator extension read a file that is being replaced ("Failed to recognize
	// image format") and show a placeholder instead of the icon.
	shownIcon []byte
	stopCh    chan struct{}
	stopOnce  sync.Once
}

func newPlatformTray(m *Manager) platformTray {
	lt := &linuxTray{
		stopCh: make(chan struct{}),
	}

	installAyatanaWarningFilter()
	systray.Register(func() {
		removeAyatanaWarningFilter()

		lt.mu.Lock()
		if lt.disposed {
			lt.mu.Unlock()
			return
		}
		lt.ready = true
		systray.SetTitle("Foghorn")
		lt.showIcon(lt.icon)
		if lt.tooltip != "" {
			setTrayTooltip(lt.tooltip)
		}
		lt.mu.Unlock()

		mShow := systray.AddMenuItem("Show or Hide Window", "Toggle the Foghorn window")
		systray.AddSeparator()
		mAbout := systray.AddMenuItem("About Foghorn", "About Foghorn")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit Foghorn", "Quit Foghorn")

		go func() {
			for {
				select {
				case <-mShow.ClickedCh:
					m.handleClick()
				case <-mAbout.ClickedCh:
					m.handleAbout()
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
		l.showIcon(icon)
		setTrayTooltip(tooltip)
	}
	return nil
}

// showIcon hands icon to systray unless it is empty or already shown.
func (l *linuxTray) showIcon(icon []byte) {
	if len(icon) == 0 || bytes.Equal(icon, l.shownIcon) {
		return
	}
	setTrayIcon(icon)
	l.shownIcon = icon
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
