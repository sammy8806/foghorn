package silence

import (
	"context"
	"fmt"
	"time"

	"foghorn/internal/model"
	"foghorn/internal/provider"
)

// Manager handles silence creation and deletion across providers.
type Manager struct {
	providers map[string]provider.Provider
}

func New(providers map[string]provider.Provider) *Manager {
	return &Manager{providers: providers}
}

// SilenceAlert creates a silence for a specific alert on its source provider.
// duration is expressed as a duration string, e.g. "2h", "30m".
func (m *Manager) SilenceAlert(ctx context.Context, alert model.Alert, duration string, comment string) (string, error) {
	p, ok := m.providers[alert.Source]
	if !ok {
		return "", fmt.Errorf("no provider registered for source %q", alert.Source)
	}

	dur, err := time.ParseDuration(duration)
	if err != nil {
		return "", fmt.Errorf("invalid duration %q: %w", duration, err)
	}

	// Build matchers from alert labels, using alertname + key identity labels
	matchers := matchersFromAlert(alert)

	req := model.SilenceRequest{
		Matchers:  matchers,
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(dur),
		CreatedBy: "foghorn",
		Comment:   comment,
	}

	return p.Silence(ctx, req)
}

// SilenceByLabels creates a silence matching specific label key=value pairs.
func (m *Manager) SilenceByLabels(ctx context.Context, source string, labels map[string]string, duration string, comment string) (string, error) {
	p, ok := m.providers[source]
	if !ok {
		return "", fmt.Errorf("no provider registered for source %q", source)
	}

	dur, err := time.ParseDuration(duration)
	if err != nil {
		return "", fmt.Errorf("invalid duration %q: %w", duration, err)
	}

	matchers := make([]model.Matcher, 0, len(labels))
	for k, v := range labels {
		matchers = append(matchers, model.Matcher{
			Name:    k,
			Value:   v,
			IsRegex: false,
			IsEqual: true,
		})
	}

	req := model.SilenceRequest{
		Matchers:  matchers,
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(dur),
		CreatedBy: "foghorn",
		Comment:   comment,
	}

	return p.Silence(ctx, req)
}

// Unsilence expires a silence by ID on the named source.
func (m *Manager) Unsilence(ctx context.Context, source, silenceID string) error {
	p, ok := m.providers[source]
	if !ok {
		return fmt.Errorf("no provider registered for source %q", source)
	}
	return p.Unsilence(ctx, silenceID)
}

func matchersFromAlert(alert model.Alert) []model.Matcher {
	// Use alertname + all labels as exact matchers
	matchers := make([]model.Matcher, 0, len(alert.Labels))
	for k, v := range alert.Labels {
		matchers = append(matchers, model.Matcher{
			Name:    k,
			Value:   v,
			IsRegex: false,
			IsEqual: true,
		})
	}
	return matchers
}
