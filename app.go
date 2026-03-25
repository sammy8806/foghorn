package main

import (
	"context"
	"fmt"
	"sync"

	"foghorn/internal/action"
	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/provider"
	"foghorn/internal/resolve"
	"foghorn/internal/silence"
	"foghorn/internal/state"
)

// App is the Wails-bound struct. Its exported methods become JS bindings.
type App struct {
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	cfg        *config.Config
	store      *state.Store
	silenceMgr *silence.Manager
	actionEng  *action.Engine
	resolveEng *resolve.Engine
}

var currentApp *App

func NewApp(cfg *config.Config, store *state.Store) *App {
	app := &App{
		cfg:        cfg,
		store:      store,
		actionEng:  action.New(cfg.Actions),
		resolveEng: resolve.New(cfg.Resolvers),
	}
	currentApp = app
	return app
}

func activeApp() *App {
	return currentApp
}

// SetProviders wires providers into the app after startup.
func (a *App) SetProviders(providers map[string]provider.Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.silenceMgr = silence.New(providers)
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {}

// UpdateConfig replaces the active config (called on hot-reload).
func (a *App) UpdateConfig(cfg *config.Config) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cfg
	a.actionEng = action.New(cfg.Actions)
	a.resolveEng = resolve.New(cfg.Resolvers)
}

// --- Wails-bound methods (called from Svelte frontend) ---

// GetAlerts returns all current alerts.
func (a *App) GetAlerts() []model.Alert {
	a.mu.RLock()
	resolveEng := a.resolveEng
	a.mu.RUnlock()

	return resolveEng.ResolveAlerts(a.store.All())
}

// GetSeverityCounts returns current severity counts.
func (a *App) GetSeverityCounts() model.SeverityCounts {
	return a.store.SeverityCounts()
}

// GetSourcesHealth returns the poll health for all configured sources.
func (a *App) GetSourcesHealth() []model.SourceHealth {
	return a.store.SourcesHealth()
}

// GetDisplayConfig returns the normalized display configuration.
func (a *App) GetDisplayConfig() config.NormalizedDisplayConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Display.Normalize()
}

// GetActions returns configured actions.
func (a *App) GetActions() []config.ActionConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Actions
}

// GetUIConfig returns UI preferences.
func (a *App) GetUIConfig() config.UIConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.UI
}

func (a *App) LayoutPopup(width, height, rightMargin, topMargin, bottomMargin int) {
	layoutPopupWindow(width, height, rightMargin, topMargin, bottomMargin)
}

// SilenceAlert creates a silence for an alert via its source provider.
func (a *App) SilenceAlert(alertID, source, duration, comment string) error {
	a.mu.RLock()
	silenceMgr := a.silenceMgr
	ctx := a.ctx
	a.mu.RUnlock()

	if silenceMgr == nil {
		return fmt.Errorf("silence manager not initialized")
	}
	alerts := a.store.All()
	for _, alert := range alerts {
		if alert.ID == alertID && alert.Source == source {
			_, err := silenceMgr.SilenceAlert(ctx, alert, duration, comment)
			return err
		}
	}
	return fmt.Errorf("alert %s/%s not found", source, alertID)
}

// Unsilence expires a silence by ID.
func (a *App) Unsilence(source, silenceID string) error {
	a.mu.RLock()
	silenceMgr := a.silenceMgr
	ctx := a.ctx
	a.mu.RUnlock()

	if silenceMgr == nil {
		return fmt.Errorf("silence manager not initialized")
	}
	return silenceMgr.Unsilence(ctx, source, silenceID)
}

// GetActionsForAlert returns actions that match the given alert.
func (a *App) GetActionsForAlert(alertID, source string) []config.ActionConfig {
	a.mu.RLock()
	actionEng := a.actionEng
	a.mu.RUnlock()

	for _, alert := range a.store.All() {
		if alert.ID == alertID && alert.Source == source {
			return actionEng.ActionsForAlert(alert)
		}
	}
	return nil
}

// ExecuteAction runs a configured action for a given alert.
func (a *App) ExecuteAction(actionName, alertID, source string) (string, error) {
	a.mu.RLock()
	actionEng := a.actionEng
	a.mu.RUnlock()

	for _, alert := range a.store.All() {
		if alert.ID == alertID && alert.Source == source {
			for _, act := range actionEng.ActionsForAlert(alert) {
				if act.Name == actionName {
					return actionEng.Execute(act, alert)
				}
			}
			return "", fmt.Errorf("action %q not found for alert", actionName)
		}
	}
	return "", fmt.Errorf("alert %s/%s not found", source, alertID)
}
