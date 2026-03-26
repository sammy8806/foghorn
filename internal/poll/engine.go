package poll

import (
	"context"
	"fmt"
	"log"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/provider"
	"foghorn/internal/state"
)

// Provider re-exports the provider interface to avoid import cycles in tests.
type Provider = provider.Provider

// ProviderFactory creates a provider from a source config.
type ProviderFactory func(source string, p Provider) Provider

// Engine manages per-source polling goroutines.
type Engine struct {
	store   *state.Store
	sources []config.SourceConfig
	factory ProviderFactory
}

// DiffEvent pairs a source name with its diff result.
type DiffEvent struct {
	Source string
	Diff   model.Diff
}

func New(store *state.Store, sources []config.SourceConfig, factory ProviderFactory) *Engine {
	return &Engine{
		store:   store,
		sources: sources,
		factory: factory,
	}
}

// Start launches a polling goroutine per source. Returns a channel of DiffEvents.
func (e *Engine) Start(ctx context.Context) <-chan DiffEvent {
	ch := make(chan DiffEvent, 64)

	for _, src := range e.sources {
		go e.pollLoop(ctx, src, ch)
	}

	return ch
}

func (e *Engine) pollLoop(ctx context.Context, src config.SourceConfig, ch chan<- DiffEvent) {
	var p Provider

	// Create provider based on type
	switch src.Type {
	case "alertmanager":
		p = provider.NewAlertmanager(src)
	case "grafana":
		p = provider.NewGrafana(src)
	case "betterstack":
		p = provider.NewBetterStack(src)
	case "prometheus":
		p = provider.NewPrometheus(src)
	default:
		if e.factory == nil {
			log.Printf("unknown provider type %q for source %s", src.Type, src.Name)
			return
		}
	}

	// Allow factory override (for testing)
	if e.factory != nil {
		p = e.factory(src.Name, p)
	}

	if p == nil {
		return
	}

	ticker := time.NewTicker(src.PollInterval)
	defer ticker.Stop()

	// Initial poll immediately
	e.poll(ctx, src.Name, p, ch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.poll(ctx, src.Name, p, ch)
		}
	}
}

func (e *Engine) poll(ctx context.Context, source string, p Provider, ch chan<- DiffEvent) {
	alerts, err := p.Fetch(ctx)
	if err != nil {
		log.Printf("error polling %s: %v", source, err)
		e.store.RecordPollError(source, err)
		// Emit empty diff so frontend refreshes health status
		select {
		case ch <- DiffEvent{Source: source}:
		default:
		}
		return
	}

	diff := e.store.Update(source, alerts)
	if onCallProvider, ok := p.(provider.OnCallProvider); ok {
		onCall, err := onCallProvider.FetchOnCall(ctx)
		if err != nil {
			log.Printf("error fetching on-call from %s: %v", source, err)
			e.store.ClearOnCall(source)
			e.store.RecordPollError(source, fmt.Errorf("on-call lookup failed: %w", err))
		} else if onCall == nil {
			e.store.ClearOnCall(source)
			e.store.RecordPollSuccess(source)
		} else {
			e.store.UpdateOnCall(source, *onCall)
			e.store.RecordPollSuccess(source)
		}
	} else {
		e.store.RecordPollSuccess(source)
	}

	select {
	case ch <- DiffEvent{Source: source, Diff: diff}:
	default:
		// Channel full, skip — non-blocking
	}
}
