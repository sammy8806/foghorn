package main

import (
	"context"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/state"
)

// App is the Wails-bound struct. Its exported methods become JS bindings.
type App struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	store  *state.Store
}

func NewApp(cfg *config.Config, store *state.Store) *App {
	return &App{cfg: cfg, store: store}
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
