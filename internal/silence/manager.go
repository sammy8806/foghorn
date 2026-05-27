package silence

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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
	dur, err := parseDuration(duration)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid duration %q: %w", duration, err)
	}
	return p, dur, nil
}

// leadingWeekDay matches a leading week or day component (e.g. "1w", "3d"),
// which the standard library's time.ParseDuration does not understand.
var leadingWeekDay = regexp.MustCompile(`^\s*(\d+)([wd])`)

// parseDuration extends time.ParseDuration with week ("w") and day ("d") units
// so the backend accepts the same grammar the UI offers (e.g. "3d", "1w2d3h").
// Leading week/day components are folded into hours; the remainder is handed to
// the standard parser, preserving its support for h/m/s/ms and composite values.
func parseDuration(s string) (time.Duration, error) {
	rest := strings.TrimSpace(s)
	var extra time.Duration
	for {
		m := leadingWeekDay.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		unit := 24 * time.Hour
		if m[2] == "w" {
			unit = 7 * 24 * time.Hour
		}
		extra += time.Duration(n) * unit
		rest = strings.TrimSpace(rest[len(m[0]):])
	}
	if rest == "" {
		if extra == 0 {
			// No week/day components and nothing left: mirror stdlib's error.
			return time.ParseDuration(strings.TrimSpace(s))
		}
		return extra, nil
	}
	d, err := time.ParseDuration(rest)
	if err != nil {
		return 0, err
	}
	return extra + d, nil
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
