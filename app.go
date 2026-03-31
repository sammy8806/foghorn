package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"foghorn/internal/action"
	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/notify"
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
	providers  map[string]provider.Provider
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
	a.providers = providers
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

// ResolveDiff applies display resolvers to diff payloads before they are sent to the UI.
func (a *App) ResolveDiff(diff model.Diff) model.Diff {
	a.mu.RLock()
	resolveEng := a.resolveEng
	a.mu.RUnlock()

	if resolveEng == nil {
		return diff
	}

	diff.New = resolveEng.ResolveAlerts(diff.New)
	diff.Resolved = resolveEng.ResolveAlerts(diff.Resolved)
	diff.Changed = resolveEng.ResolveAlerts(diff.Changed)
	return diff
}

// GetSeverityCounts returns current severity counts.
func (a *App) GetSeverityCounts() model.SeverityCounts {
	return a.store.SeverityCounts()
}

// GetSeverityConfig returns the normalized severity configuration.
func (a *App) GetSeverityConfig() config.NormalizedSeverityConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()

	normalized, err := config.NormalizeSeverityConfig(a.cfg.Severities)
	if err != nil {
		normalized, _ = config.NormalizeSeverityConfig(config.DefaultSeverityConfig())
	}
	return normalized
}

// GetSourcesHealth returns the poll health for all configured sources.
func (a *App) GetSourcesHealth() []model.SourceHealth {
	return a.store.SourcesHealth()
}

func (a *App) GetSourceCapabilities() map[string]model.SourceCapabilities {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make(map[string]model.SourceCapabilities, len(a.providers))
	for source, p := range a.providers {
		out[source] = model.SourceCapabilities{
			SupportsSilence: p.SupportsSilence(),
		}
	}
	return out
}

func (a *App) GetOnCallStatus() []model.OnCallStatus {
	return a.store.OnCalls()
}

// RefreshAlerts forces an immediate fetch from every configured provider.
func (a *App) RefreshAlerts() error {
	a.mu.RLock()
	ctx := a.ctx
	providers := make(map[string]provider.Provider, len(a.providers))
	for source, p := range a.providers {
		providers[source] = p
	}
	a.mu.RUnlock()

	if ctx == nil {
		ctx = context.Background()
	}

	for source, p := range providers {
		alerts, err := p.Fetch(ctx)
		if err != nil {
			a.store.RecordPollError(source, err)
			continue
		}

		a.store.Update(source, alerts)
		if onCallProvider, ok := p.(provider.OnCallProvider); ok {
			onCall, err := onCallProvider.FetchOnCall(ctx)
			if err != nil {
				a.store.ClearOnCall(source)
				a.store.RecordPollError(source, fmt.Errorf("on-call lookup failed: %w", err))
				continue
			}
			if onCall == nil {
				a.store.ClearOnCall(source)
			} else {
				a.store.UpdateOnCall(source, *onCall)
			}
		}

		a.store.RecordPollSuccess(source)
	}

	return nil
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
	ui := a.cfg.UI
	ui.DefaultCreatedBy = config.ResolveCreatedByDefault(ui.DefaultCreatedBy)
	log.Printf("app: GetUIConfig returning default_created_by=%q", ui.DefaultCreatedBy)
	return ui
}

func (a *App) LayoutPopup(width, height, rightMargin, topMargin, bottomMargin int) {
	layoutPopupWindow(width, height, rightMargin, topMargin, bottomMargin)
}

func (a *App) TestNotificationForAlert(alertID, source string) error {
	for _, alert := range a.store.All() {
		if alert.ID == alertID && alert.Source == source {
			return notify.SendNewAlertNotification(alert)
		}
	}
	return fmt.Errorf("alert %s/%s not found", source, alertID)
}

func (a *App) GetNotificationPermissionStatus() string {
	return notify.NotificationPermissionStatus()
}

func (a *App) OpenNotificationSettings() error {
	return notify.OpenNotificationSettings()
}

// SilenceAlert creates a silence for an alert via its source provider.
func (a *App) SilenceAlert(alertID, source, duration, createdBy, comment string) error {
	a.mu.RLock()
	silenceMgr := a.silenceMgr
	ctx := a.ctx
	defaultCreatedBy := config.ResolveCreatedByDefault(a.cfg.UI.DefaultCreatedBy)
	a.mu.RUnlock()

	if silenceMgr == nil {
		return fmt.Errorf("silence manager not initialized")
	}
	alerts := a.store.All()
	for _, alert := range alerts {
		if alert.ID == alertID && alert.Source == source {
			_, err := silenceMgr.SilenceAlert(ctx, alert, duration, createdBy, comment, defaultCreatedBy)
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
