//go:build linux && linux_tray

package tray

import (
	"bytes"
	"sync"

	"fyne.io/systray"
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
	// shownIcon is the icon last handed to systray, so an unchanged icon is not sent over
	// D-Bus again on every poll.
	shownIcon []byte
	stopCh    chan struct{}
	stopOnce  sync.Once
}

func newPlatformTray(m *Manager) platformTray {
	lt := &linuxTray{
		stopCh: make(chan struct{}),
	}

	// The title also becomes the StatusNotifierItem id, so it is set before the item is exported.
	systray.SetTitle("Foghorn")
	// A left click shows or hides the window, as on Windows. Without a handler the item reports
	// ItemIsMenu and rejects Activate, so a host that sends the left click as Activate (such as
	// Quickshell) does nothing. ItemIsMenu is read when the item is exported, so this is set
	// before start. The menu stays on right click or hover, depending on the host.
	systray.SetOnTapped(m.handleClick)
	// fyne.io/systray exports the StatusNotifierItem over D-Bus itself (no libappindicator, no
	// GTK loop). Its onReady callback runs before the item is exported, when an icon set there
	// can be lost, so the tray is set up only once start has returned.
	start, _ := systray.RunWithExternalLoop(nil, nil)
	go func() {
		start()

		lt.mu.Lock()
		if lt.disposed {
			lt.mu.Unlock()
			return
		}
		lt.ready = true
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

// dispose runs at shutdown. It quits with systray.Quit rather than the end func of
// RunWithExternalLoop, which closes the D-Bus connection without a nil check and would panic
// if the session bus was unreachable; the connection closes with the process anyway.
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
