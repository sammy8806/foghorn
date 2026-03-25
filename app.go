package main

import (
	"context"
	"fmt"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/provider"
	"foghorn/internal/silence"
	"foghorn/internal/state"
)

// App is the Wails-bound struct. Its exported methods become JS bindings.
type App struct {
	ctx       context.Context
	cancel    context.CancelFunc
	cfg       *config.Config
	store     *state.Store
	silenceMgr *silence.Manager
}

func NewApp(cfg *config.Config, store *state.Store) *App {
	return &App{cfg: cfg, store: store}
}

// SetProviders wires providers into the app after startup.
func (a *App) SetProviders(providers map[string]provider.Provider) {
	a.silenceMgr = silence.New(providers)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {}

// --- Wails-bound methods (called from Svelte frontend) ---

// GetAlerts returns all current alerts.
func (a *App) GetAlerts() []model.Alert {
	return a.store.All()
}

// GetSeverityCounts returns current severity counts.
func (a *App) GetSeverityCounts() model.SeverityCounts {
	return a.store.SeverityCounts()
}

// GetDisplayConfig returns the display configuration.
func (a *App) GetDisplayConfig() config.DisplayConfig {
	return a.cfg.Display
}

// GetActions returns configured actions.
func (a *App) GetActions() []config.ActionConfig {
	return a.cfg.Actions
}

// GetUIConfig returns UI preferences.
func (a *App) GetUIConfig() config.UIConfig {
	return a.cfg.UI
}

// SilenceAlert creates a silence for an alert via its source provider.
func (a *App) SilenceAlert(alertID, source, duration, comment string) error {
	if a.silenceMgr == nil {
		return fmt.Errorf("silence manager not initialized")
	}
	alerts := a.store.All()
	for _, alert := range alerts {
		if alert.ID == alertID && alert.Source == source {
			_, err := a.silenceMgr.SilenceAlert(a.ctx, alert, duration, comment)
			return err
		}
	}
	return fmt.Errorf("alert %s/%s not found", source, alertID)
}

// Unsilence expires a silence by ID.
func (a *App) Unsilence(source, silenceID string) error {
	if a.silenceMgr == nil {
		return fmt.Errorf("silence manager not initialized")
	}
	return a.silenceMgr.Unsilence(a.ctx, source, silenceID)
}
