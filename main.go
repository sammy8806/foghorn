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
		StartHidden:       false, // tray is currently stubbed; show window on launch
		HideWindowOnClose: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)

			bgCtx, cancel := context.WithCancel(context.Background())
			app.cancel = cancel

			notifier := notify.New(cfg.Notifications)
			pollEng := poll.New(store, cfg.Sources, nil)
			diffCh := pollEng.Start(bgCtx)

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

			// Config hot-reload: watch for changes and notify frontend
			if stopWatch, err := config.Watch(cfgPath, func(newCfg *config.Config) {
				app.UpdateConfig(newCfg)
				wailsruntime.EventsEmit(ctx, "config:reloaded")
			}); err != nil {
				log.Printf("config: watcher not started: %v", err)
			} else {
				_ = stopWatch // cleaned up when process exits
			}

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
