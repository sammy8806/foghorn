package silence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"foghorn/internal/model"
	"foghorn/internal/provider"
)

// Manager handles silence creation, update, and deletion across providers.
type Manager struct {
	providers map[string]provider.Provider
}

func New(providers map[string]provider.Provider) *Manager {
	return &Manager{providers: providers}
}

// CreateSilence creates a new silence with the explicit matchers supplied by the caller.
// duration is expressed as a duration string, e.g. "2h", "30m".
func (m *Manager) CreateSilence(
	ctx context.Context,
	source string,
	matchers []model.Matcher,
	duration, createdBy, comment, defaultCreatedBy string,
) (string, error) {
	p, dur, err := m.resolve(source, duration)
	if err != nil {
		return "", err
	}
	now := time.Now()
	req := model.SilenceRequest{
		Matchers:  matchers,
		StartsAt:  now,
		EndsAt:    now.Add(dur),
		CreatedBy: resolveCreatedBy(createdBy, defaultCreatedBy),
		Comment:   comment,
	}
	return p.Silence(ctx, req)
}

// UpdateSilence replaces an existing silence in place. The silence keeps its ID;
// startsAt is reset to now and endsAt is now+duration.
func (m *Manager) UpdateSilence(
	ctx context.Context,
	source, silenceID string,
	matchers []model.Matcher,
	duration, createdBy, comment, defaultCreatedBy string,
) error {
	if strings.TrimSpace(silenceID) == "" {
		return fmt.Errorf("silence id is required for update")
	}
	p, dur, err := m.resolve(source, duration)
	if err != nil {
		return err
	}
	now := time.Now()
	req := model.SilenceRequest{
		ID:        silenceID,
		Matchers:  matchers,
		StartsAt:  now,
		EndsAt:    now.Add(dur),
		CreatedBy: resolveCreatedBy(createdBy, defaultCreatedBy),
		Comment:   comment,
	}
	_, err = p.Silence(ctx, req)
	return err
}

// Unsilence expires a silence by ID on the named source.
func (m *Manager) Unsilence(ctx context.Context, source, silenceID string) error {
	p, ok := m.providers[source]
	if !ok {
		return fmt.Errorf("no provider registered for source %q", source)
	}
	return p.Unsilence(ctx, silenceID)
}

func (m *Manager) resolve(source, duration string) (provider.Provider, time.Duration, error) {
	p, ok := m.providers[source]
	if !ok {
		return nil, 0, fmt.Errorf("no provider registered for source %q", source)
	}
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid duration %q: %w", duration, err)
	}
	return p, dur, nil
}

func resolveCreatedBy(createdBy, fallback string) string {
	if value := strings.TrimSpace(createdBy); value != "" {
		return value
	}
	if value := strings.TrimSpace(fallback); value != "" {
		return value
	}
	return "foghorn"
}
