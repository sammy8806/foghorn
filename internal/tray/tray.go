// Package tray manages the system tray icon and menu.
//
// NOTE: getlantern/systray conflicts with Wails on macOS because both define
// an Objective-C AppDelegate, causing a duplicate symbol linker error.
// The tray manager is currently a no-op stub. Proper macOS NSStatusBar
// integration requires dispatching to the existing Wails-owned NSApp via
// CGO dispatch_async(dispatch_get_main_queue(), ...) — a follow-up task.
package tray

import (
	"fmt"
	"log"

	"foghorn/internal/model"
)

type OnClickFunc func()
type OnQuitFunc func()

type Manager struct {
	onClick OnClickFunc
	onQuit  OnQuitFunc
	counts  model.SeverityCounts
}

func NewManager(onClick OnClickFunc, onQuit OnQuitFunc) *Manager {
	return &Manager{onClick: onClick, onQuit: onQuit}
}

// Run is a no-op stub. Calls onReady immediately.
// TODO: implement macOS NSStatusBar via CGO without conflicting with Wails AppDelegate.
func (m *Manager) Run(onReady func()) {
	log.Println("tray: systray is stubbed (NSApp conflict with Wails not yet resolved)")
	if onReady != nil {
		onReady()
	}
}

// UpdateState logs the new severity state. No actual tray icon update yet.
func (m *Manager) UpdateState(counts model.SeverityCounts) {
	m.counts = counts
	total := counts.Critical + counts.Warning + counts.Info
	switch {
	case counts.Critical > 0:
		log.Printf("tray: %d critical, %d warning", counts.Critical, counts.Warning)
	case counts.Warning > 0:
		log.Printf("tray: %d warning", counts.Warning)
	case total == 0:
		log.Println("tray: all clear")
	default:
		log.Printf("tray: %d info", counts.Info)
	}
}

// Tooltip returns the tooltip string that would be set on the tray icon.
func (m *Manager) Tooltip() string {
	c := m.counts
	total := c.Critical + c.Warning + c.Info
	switch {
	case c.Critical > 0:
		return fmt.Sprintf("Foghorn — %d critical, %d warning", c.Critical, c.Warning)
	case c.Warning > 0:
		return fmt.Sprintf("Foghorn — %d warning", c.Warning)
	case total == 0:
		return "Foghorn — All clear"
	default:
		return fmt.Sprintf("Foghorn — %d info", c.Info)
	}
}
