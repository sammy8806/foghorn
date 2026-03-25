package tray

import (
	"fmt"
	"sync"

	"foghorn/internal/model"
)

type OnClickFunc func()
type OnQuitFunc func()

type platformTray interface {
	update(icon []byte, tooltip string) error
	dispose()
}

type Manager struct {
	mu       sync.RWMutex
	onClick  OnClickFunc
	onQuit   OnQuitFunc
	counts   model.SeverityCounts
	ready    bool
	platform platformTray
}

func NewManager(onClick OnClickFunc, onQuit OnQuitFunc) *Manager {
	return &Manager{onClick: onClick, onQuit: onQuit}
}

func (m *Manager) Run(onReady func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.platform == nil {
		m.platform = newPlatformTray(m)
	}
	if m.platform != nil {
		_ = m.platform.update(m.iconForCounts(), m.tooltipLocked())
	}
	if onReady != nil {
		onReady()
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.platform != nil {
		m.platform.dispose()
		m.platform = nil
	}
}

func (m *Manager) UpdateState(counts model.SeverityCounts) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counts = counts
	m.ready = true
	if m.platform != nil {
		_ = m.platform.update(m.iconForCounts(), m.tooltipLocked())
	}
}

func (m *Manager) Tooltip() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tooltipLocked()
}

func (m *Manager) handleClick() {
	if m.onClick != nil {
		go m.onClick()
	}
}

func (m *Manager) handleQuit() {
	if m.onQuit != nil {
		go m.onQuit()
	}
}

func (m *Manager) tooltipLocked() string {
	if !m.ready {
		return "Foghorn - Starting up"
	}

	c := m.counts
	total := c.Critical + c.Warning + c.Info
	switch {
	case c.Critical > 0:
		return fmt.Sprintf("Foghorn - %d critical, %d warning", c.Critical, c.Warning)
	case c.Warning > 0:
		return fmt.Sprintf("Foghorn - %d warning", c.Warning)
	case total == 0:
		return "Foghorn - All clear"
	default:
		return fmt.Sprintf("Foghorn - %d info", c.Info)
	}
}

func (m *Manager) iconForCounts() []byte {
	if !m.ready {
		return IconGrey
	}

	c := m.counts
	total := c.Critical + c.Warning + c.Info
	switch {
	case c.Critical > 0:
		return IconRed
	case c.Warning > 0:
		return IconYellow
	case total == 0:
		return IconGreen
	default:
		return IconGreen
	}
}
