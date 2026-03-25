package state

import (
	"sort"
	"sync"
	"time"

	"foghorn/internal/model"
)

// Store holds the in-memory alert state and computes diffs.
type Store struct {
	mu sync.RWMutex
	// alerts keyed by source, then by alert Key()
	bySource map[string]map[string]model.Alert
	// health tracks the last poll result per source
	health map[string]model.SourceHealth
}

func New() *Store {
	return &Store{
		bySource: make(map[string]map[string]model.Alert),
		health:   make(map[string]model.SourceHealth),
	}
}

// Update replaces all alerts for a source and returns the diff.
func (s *Store) Update(source string, alerts []model.Alert) model.Diff {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.bySource[source]
	if prev == nil {
		prev = make(map[string]model.Alert)
	}

	curr := make(map[string]model.Alert, len(alerts))
	for _, a := range alerts {
		curr[a.Key()] = a
	}

	var diff model.Diff

	// New and changed
	for key, alert := range curr {
		if old, exists := prev[key]; !exists {
			diff.New = append(diff.New, alert)
		} else if old.State != alert.State {
			diff.Changed = append(diff.Changed, alert)
		}
	}

	// Resolved
	for key, alert := range prev {
		if _, exists := curr[key]; !exists {
			diff.Resolved = append(diff.Resolved, alert)
		}
	}

	s.bySource[source] = curr
	return diff
}

// All returns all current alerts across all sources.
func (s *Store) All() []model.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []model.Alert
	for _, alerts := range s.bySource {
		for _, a := range alerts {
			all = append(all, a)
		}
	}
	// Sort for deterministic order: by source, then by alert key.
	sort.Slice(all, func(i, j int) bool {
		if all[i].Source != all[j].Source {
			return all[i].Source < all[j].Source
		}
		return all[i].Key() < all[j].Key()
	})
	return all
}

// SeverityCounts returns counts per severity across all sources.
func (s *Store) SeverityCounts() model.SeverityCounts {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var counts model.SeverityCounts
	for _, alerts := range s.bySource {
		for _, a := range alerts {
			switch a.Severity {
			case "critical":
				counts.Critical++
			case "warning":
				counts.Warning++
			default:
				counts.Info++
			}
		}
	}
	return counts
}

// RecordPollSuccess marks a successful poll for a source.
func (s *Store) RecordPollSuccess(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health[source] = model.SourceHealth{
		Source:   source,
		OK:       true,
		LastPoll: time.Now(),
	}
}

// RecordPollError marks a failed poll for a source.
func (s *Store) RecordPollError(source string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.health[source]
	s.health[source] = model.SourceHealth{
		Source:      source,
		OK:          false,
		LastPoll:    time.Now(),
		LastError:   err.Error(),
		ConsecFails: prev.ConsecFails + 1,
	}
}

// SourcesHealth returns the poll health for all sources.
func (s *Store) SourcesHealth() []model.SourceHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.SourceHealth, 0, len(s.health))
	for _, h := range s.health {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}
