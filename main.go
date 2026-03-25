package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"os"
	"path/filepath"

	"foghorn/internal/config"
	"foghorn/internal/notify"
	"foghorn/internal/poll"
	"foghorn/internal/state"
	"foghorn/internal/tray"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfgPath := configPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Config file not found at %s, using defaults", cfgPath)
			cfg = config.Default()
		} else {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	store := state.New()
	app := NewApp(cfg, store)

	windowVisible := false

	// trayMgr is created here but Run() is called from a goroutine in OnStartup.
	// On macOS, systray needs the main thread — a proper solution requires CGO
	// dispatch_async. For initial builds, this goroutine approach is used.
	trayMgr := tray.NewManager(
		func() {
			if app.ctx == nil {
				return
			}
			if windowVisible {
				wailsruntime.WindowHide(app.ctx)
				windowVisible = false
			} else {
				wailsruntime.WindowShow(app.ctx)
				wailsruntime.WindowSetAlwaysOnTop(app.ctx, true)
				windowVisible = true
			}
		},
		func() {
			if app.cancel != nil {
				app.cancel()
			}
			if app.ctx != nil {
				wailsruntime.Quit(app.ctx)
			}
		},
	)

	if err := wails.Run(&options.App{
		Title:             "Foghorn",
		Width:             cfg.UI.PopupWidth,
		Height:            cfg.UI.PopupHeight,
		StartHidden:       true,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)

			// Start background services
			bgCtx, cancel := context.WithCancel(context.Background())
			app.cancel = cancel

			notifier := notify.New(cfg.Notifications)
			diffCh := poll.New(store, cfg.Sources, nil).Start(bgCtx)

			go func() {
				for {
					select {
					case <-bgCtx.Done():
						return
					case event := <-diffCh:
						counts := store.SeverityCounts()
						trayMgr.UpdateState(counts)
						notifier.OnDiff(event.Diff)
						wailsruntime.EventsEmit(ctx, "alerts:updated")
					}
				}
			}()

			// Launch systray in a goroutine. Note: on macOS a proper
			// implementation requires calling this on the main thread via
			// CGO dispatch_async. This is a known limitation to address.
			go trayMgr.Run(nil)
		},
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "foghorn-alert-monitor-001",
		},
	}); err != nil {
		log.Fatalf("Wails error: %v", err)
	}
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "foghorn", "config.yaml")
}
