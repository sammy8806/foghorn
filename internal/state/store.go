package state

import (
	"sync"

	"foghorn/internal/model"
)

// Store holds the in-memory alert state and computes diffs.
type Store struct {
	mu sync.RWMutex
	// alerts keyed by source, then by alert Key()
	bySource map[string]map[string]model.Alert
}

func New() *Store {
	return &Store{
		bySource: make(map[string]map[string]model.Alert),
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
