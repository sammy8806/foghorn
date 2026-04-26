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
	mu        sync.RWMutex
	onClick   OnClickFunc
	onQuit    OnQuitFunc
	breakdown model.SeverityBreakdown
	ready     bool
	platform  platformTray
	scheme    config.SeverityScheme
}

func NewManager(onClick OnClickFunc, onQuit OnQuitFunc) *Manager {
	normalized, _ := config.NormalizeSeverityConfig(config.DefaultSeverityConfig())
	scheme := normalized.Scheme()
	return &Manager{
		onClick:   onClick,
		onQuit:    onQuit,
		breakdown: emptyBreakdown(scheme),
		scheme:    scheme,
	}
}

func (m *Manager) Run(onReady func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.platform == nil {
		m.platform = newPlatformTray(m)
	}
	if m.platform != nil {
		_ = m.platform.update(m.iconLocked(), m.tooltipLocked())
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

// UpdateState sets the latest severity breakdown (active vs silenced) and
// refreshes the tray icon + tooltip accordingly.
func (m *Manager) UpdateState(breakdown model.SeverityBreakdown) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.breakdown = normalizeBreakdown(breakdown, m.scheme)
	m.ready = true
	if m.platform != nil {
		_ = m.platform.update(m.iconLocked(), m.tooltipLocked())
	}
}

func (m *Manager) SetSeverityConfig(cfg config.NormalizedSeverityConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.scheme = cfg.Scheme()
	m.breakdown = normalizeBreakdown(m.breakdown, m.scheme)
	if m.platform != nil {
		_ = m.platform.update(m.iconLocked(), m.tooltipLocked())
	}
}

func (m *Manager) Tooltip() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tooltipLocked()
}

// icon returns the current tray icon bytes. Test-only accessor in the tray
// package; production callers receive the icon via platformTray.update.
func (m *Manager) icon() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.iconLocked()
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

	activeParts := severityParts(m.breakdown.Active, m.scheme)
	silencedParts := severityParts(m.breakdown.Silenced, m.scheme)

	activeFragment := joinParts(activeParts)
	silencedFragment := joinParts(silencedParts)

	switch {
	case activeFragment == "" && silencedFragment == "":
		return "Foghorn - All clear"
	case activeFragment == "":
		return fmt.Sprintf("Foghorn - All clear (%s silenced)", silencedFragment)
	case silencedFragment == "":
		return "Foghorn - " + activeFragment
	default:
		return fmt.Sprintf("Foghorn - %s (%s silenced)", activeFragment, silencedFragment)
	}
}

// iconLocked picks the tray icon based only on the active (non-silenced)
// severity counts. Silenced alerts never influence the icon.
func (m *Manager) iconLocked() []byte {
	if !m.ready {
		return IconGrey
	}

	total := 0
	for _, count := range m.breakdown.Active {
		total += count
	}
	if total == 0 {
		return IconGreen
	}
	for _, level := range m.scheme.Levels {
		if m.breakdown.Active[level.Name] <= 0 {
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

// severityParts returns a list of "<count> <severity>" strings in scheme
// order, skipping zero-count entries. Matches the tray's prior 2-part cap.
func severityParts(counts model.SeverityCounts, scheme config.SeverityScheme) []string {
	parts := make([]string, 0, len(scheme.Levels))
	for _, level := range scheme.Levels {
		if count := counts[level.Name]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, level.Name))
		}
	}
	return parts
}

// joinParts collapses the severity parts into the comma-separated fragment
// shown in the tooltip, preserving the pre-existing 2-part cap.
func joinParts(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return parts[0] + ", " + parts[1]
	}
}

// emptyBreakdown returns a zero-valued breakdown with all levels in the
// given scheme initialized to 0 for both Active and Silenced.
func emptyBreakdown(scheme config.SeverityScheme) model.SeverityBreakdown {
	return model.SeverityBreakdown{
		Active:   model.SeverityCounts(scheme.EmptyCounts()),
		Silenced: model.SeverityCounts(scheme.EmptyCounts()),
	}
}

// normalizeBreakdown aligns a breakdown with the given scheme: unknown levels
// are pruned and missing ones are zero-filled. Nil inner maps are replaced
// with empty ones.
func normalizeBreakdown(b model.SeverityBreakdown, scheme config.SeverityScheme) model.SeverityBreakdown {
	return model.SeverityBreakdown{
		Active:   normalizeCounts(b.Active, scheme),
		Silenced: normalizeCounts(b.Silenced, scheme),
	}
}

func normalizeCounts(counts model.SeverityCounts, scheme config.SeverityScheme) model.SeverityCounts {
	out := model.SeverityCounts(scheme.EmptyCounts())
	for name, count := range counts {
		if _, ok := out[name]; ok {
			out[name] = count
		}
	}
	return out
}
