package tray

import (
	"fmt"

	"foghorn/internal/model"
	"github.com/getlantern/systray"
)

type OnClickFunc func()
type OnQuitFunc func()

type Manager struct {
	onClick OnClickFunc
	onQuit  OnQuitFunc
}

func NewManager(onClick OnClickFunc, onQuit OnQuitFunc) *Manager {
	return &Manager{onClick: onClick, onQuit: onQuit}
}

// Run starts the systray. This blocks — call from the main goroutine.
func (m *Manager) Run(onReady func()) {
	systray.Run(func() {
		systray.SetIcon(IconGrey)
		systray.SetTooltip("Foghorn — Alert Monitor")

		mShow := systray.AddMenuItem("Show/Hide", "Toggle alert window")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit Foghorn")

		go func() {
			for {
				select {
				case <-mShow.ClickedCh:
					if m.onClick != nil {
						m.onClick()
					}
				case <-mQuit.ClickedCh:
					if m.onQuit != nil {
						m.onQuit()
					}
					systray.Quit()
					return
				}
			}
		}()

		if onReady != nil {
			onReady()
		}
	}, func() {
		// Cleanup on exit
	})
}

// UpdateState updates the tray icon and tooltip based on severity counts.
func (m *Manager) UpdateState(counts model.SeverityCounts) {
	total := counts.Critical + counts.Warning + counts.Info

	switch {
	case counts.Critical > 0:
		systray.SetIcon(IconRed)
		systray.SetTooltip(fmt.Sprintf("Foghorn — %d critical, %d warning", counts.Critical, counts.Warning))
	case counts.Warning > 0:
		systray.SetIcon(IconYellow)
		systray.SetTooltip(fmt.Sprintf("Foghorn — %d warning", counts.Warning))
	case total == 0:
		systray.SetIcon(IconGreen)
		systray.SetTooltip("Foghorn — All clear")
	default:
		systray.SetIcon(IconGreen)
		systray.SetTooltip(fmt.Sprintf("Foghorn — %d info", counts.Info))
	}
}
