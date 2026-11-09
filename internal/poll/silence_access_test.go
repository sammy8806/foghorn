package poll

import (
	"context"
	"errors"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/state"
)

type accessProvider struct {
	silenceProvider
	silenceErr  error
	canRead     bool
	fetchSilenc int
	events      []string
}

func (a *accessProvider) Fetch(ctx context.Context) ([]model.Alert, error) {
	a.events = append(a.events, "fetch")
	return a.silenceProvider.Fetch(ctx)
}
func (a *accessProvider) FetchSilences(ctx context.Context) ([]model.SilenceInfo, error) {
	a.fetchSilenc++
	if a.silenceErr != nil {
		return nil, a.silenceErr
	}
	return a.silenceProvider.FetchSilences(ctx)
}
func (a *accessProvider) SilenceAccess() model.SilenceAccess { return model.SilenceAccess{} }
func (a *accessProvider) RefreshSilenceAccess(context.Context, time.Time) {
	a.events = append(a.events, "refresh")
}
func (a *accessProvider) CanReadSilences() bool { return a.canRead }

func pollOnce(store *state.Store, p Provider) {
	e := New(store, nil, nil)
	e.poll(context.Background(), config.SourceConfig{Name: "g"}, p, make(chan DiffEvent, 4))
}

func health(t *testing.T, store *state.Store) model.SourceHealth {
	t.Helper()
	h := store.SourcesHealth()
	if len(h) != 1 {
		t.Fatalf("health %+v", h)
	}
	return h[0]
}

func TestPollRecordsSilenceReadError(t *testing.T) {
	store := state.New()
	p := &accessProvider{silenceProvider: silenceProvider{name: "g"}, canRead: true, silenceErr: errors.New("HTTP 403")}
	pollOnce(store, p)
	if h := health(t, store); !h.OK || h.SilenceReadError == "" {
		t.Fatalf("health %+v", h)
	}
	p.silenceErr = nil
	pollOnce(store, p)
	if h := health(t, store); h.SilenceReadError != "" {
		t.Fatalf("not cleared: %+v", h)
	}
}

func TestPollSkipsSilenceReadWhenNotPermitted(t *testing.T) {
	store := state.New()
	store.RecordSilenceReadError("g", errors.New("old"))
	p := &accessProvider{silenceProvider: silenceProvider{name: "g"}, canRead: false}
	pollOnce(store, p)
	if p.fetchSilenc != 0 {
		t.Fatal("FetchSilences called without read permission")
	}
	if h := health(t, store); h.SilenceReadError != "" {
		t.Fatalf("health %+v", h)
	}
}

func TestPollRefreshesSilenceAccessBeforeFetch(t *testing.T) {
	p := &accessProvider{silenceProvider: silenceProvider{name: "g"}, canRead: true}
	pollOnce(state.New(), p)
	if len(p.events) != 2 || p.events[0] != "refresh" || p.events[1] != "fetch" {
		t.Fatalf("events %v", p.events)
	}
}
