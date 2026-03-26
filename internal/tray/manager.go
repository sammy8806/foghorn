package tray

import (
	"fmt"
	"sync"

	"foghorn/internal/config"
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
	scheme   config.SeverityScheme
}

func NewManager(onClick OnClickFunc, onQuit OnQuitFunc) *Manager {
	normalized, _ := config.NormalizeSeverityConfig(config.DefaultSeverityConfig())
	return &Manager{
		onClick: onClick,
		onQuit:  onQuit,
		counts:  model.SeverityCounts(normalized.Scheme().EmptyCounts()),
		scheme:  normalized.Scheme(),
	}
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

func (m *Manager) SetSeverityConfig(cfg config.NormalizedSeverityConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.scheme = cfg.Scheme()
	if m.counts == nil {
		m.counts = model.SeverityCounts(m.scheme.EmptyCounts())
	}
	for name := range m.counts {
		if _, ok := m.scheme.EmptyCounts()[name]; !ok {
			delete(m.counts, name)
		}
	}
	for name := range m.scheme.EmptyCounts() {
		if _, ok := m.counts[name]; !ok {
			m.counts[name] = 0
		}
	}
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
	total := 0
	for _, count := range c {
		total += count
	}
	if total == 0 {
		return "Foghorn - All clear"
	}

	parts := make([]string, 0, len(m.scheme.Levels))
	for _, level := range m.scheme.Levels {
		if count := c[level.Name]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, level.Name))
		}
	}
	if len(parts) == 0 {
		return "Foghorn - Alerts active"
	}
	if len(parts) == 1 {
		return "Foghorn - " + parts[0]
	}
	return "Foghorn - " + parts[0] + ", " + parts[1]
}

func (m *Manager) iconForCounts() []byte {
	if !m.ready {
		return IconGrey
	}

	c := m.counts
	total := 0
	for _, count := range c {
		total += count
	}
	if total == 0 {
		return IconGreen
	}
	for _, level := range m.scheme.Levels {
		if c[level.Name] <= 0 {
			continue
		}
		switch level.Rank {
		case 0:
			return IconRed
		case 1:
			return IconYellow
		default:
			return IconGreen
		}
	}
	return IconGreen
}
