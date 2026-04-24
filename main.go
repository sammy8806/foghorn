package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"foghorn/internal/config"
	"foghorn/internal/notify"
	"foghorn/internal/poll"
	"foghorn/internal/provider"
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
	config.MigrateLegacyPath(cfgPath)
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

	var runtimeMu sync.Mutex
	var stopRuntime context.CancelFunc
	var windowVisible atomic.Bool
	var quitting atomic.Bool

	// requestQuit marks the app as quitting and asks Wails to terminate. It is
	// used by both the tray "Quit" menu item and the SIGINT/SIGTERM handler.
	// Setting quitting=true is what allows OnBeforeClose (below) to return
	// false, which is the only way the Darwin Wails frontend will actually
	// call mainWindow.Quit() and exit the Cocoa run loop.
	requestQuit := func() {
		if !quitting.CompareAndSwap(false, true) {
			return
		}
		if app.cancel != nil {
			app.cancel()
		}
		if app.ctx != nil {
			wailsruntime.Quit(app.ctx)
		}
	}

	trayMgr := tray.NewManager(
		func() {
			if app.ctx == nil {
				return
			}
			if windowVisible.Load() {
				wailsruntime.WindowHide(app.ctx)
				windowVisible.Store(false)
			} else {
				wailsruntime.WindowShow(app.ctx)
				wailsruntime.WindowSetAlwaysOnTop(app.ctx, true)
				windowVisible.Store(true)
				wailsruntime.EventsEmit(app.ctx, "popup:opening")
			}
		},
		requestQuit,
	)

	// Handle Ctrl+C / SIGTERM when running from the CLI. Wails installs its
	// own signal handler, but on macOS its handler calls frontend.Quit(),
	// which defers to OnBeforeClose. OnBeforeClose returns true unless
	// `quitting` is set, causing the window to be hidden instead of the app
	// exiting. By registering our own handler first and flipping `quitting`
	// here we ensure a Ctrl+C actually terminates the process.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		for signalCount := 1; ; signalCount++ {
			<-sigCh
			if signalCount == 1 {
				requestQuit()
				continue
			}
			os.Exit(1)
		}
	}()

	if err := wails.Run(&options.App{
		Title:             "Foghorn",
		Width:             cfg.UI.PopupWidth,
		Height:            cfg.UI.PopupHeight,
		StartHidden:       tray.StartHiddenByDefault(),
		HideWindowOnClose: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)

			restartRuntime := func(nextCfg *config.Config) {
				runtimeMu.Lock()
				defer runtimeMu.Unlock()

				if stopRuntime != nil {
					stopRuntime()
				}

				app.UpdateConfig(nextCfg)
				app.SetProviders(buildProviders(nextCfg.Sources))
				store.SyncSources(sourceNames(nextCfg.Sources))
				severities, err := config.NormalizeSeverityConfig(nextCfg.Severities)
				if err != nil {
					log.Printf("config: invalid severities after reload, using defaults: %v", err)
					severities, _ = config.NormalizeSeverityConfig(config.DefaultSeverityConfig())
				}
				store.SetSeverityConfig(severities)
				trayMgr.SetSeverityConfig(severities)

				bgCtx, cancel := context.WithCancel(context.Background())
				stopRuntime = cancel
				app.cancel = cancel

				notifier := notify.New(nextCfg.Notifications, severities)
				pollEng := poll.New(store, nextCfg.Sources, nil)
				diffCh := pollEng.Start(bgCtx)

				go func(localCtx context.Context, localDiffCh <-chan poll.DiffEvent, localNotifier *notify.Engine) {
					for {
						select {
						case <-localCtx.Done():
							return
						case event := <-localDiffCh:
							trayMgr.UpdateState(store.SeverityBreakdown())
							localNotifier.OnDiff(event.Diff)
							wailsruntime.EventsEmit(ctx, "alerts:updated", app.ResolveDiff(event.Diff))
						}
					}
				}(bgCtx, diffCh, notifier)
			}

			restartRuntime(cfg)

			// Config hot-reload: watch for changes and notify frontend
			if stopWatch, err := config.Watch(cfgPath, func(newCfg *config.Config) {
				restartRuntime(newCfg)
				wailsruntime.EventsEmit(ctx, "config:reloaded")
			}); err != nil {
				log.Printf("config: watcher not started: %v", err)
			} else {
				_ = stopWatch // cleaned up when process exits
			}

			trayMgr.Run(nil)
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if quitting.Load() {
				return false
			}
			wailsruntime.WindowHide(ctx)
			windowVisible.Store(false)
			return true
		},
		OnShutdown: func(ctx context.Context) {
			runtimeMu.Lock()
			if stopRuntime != nil {
				stopRuntime()
			}
			runtimeMu.Unlock()
			trayMgr.Close()
			app.shutdown(ctx)
		},
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
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "foghorn", "config.yaml")
}

func buildProviders(sources []config.SourceConfig) map[string]provider.Provider {
	providers := make(map[string]provider.Provider, len(sources))
	for _, src := range sources {
		switch src.Type {
		case "alertmanager":
			providers[src.Name] = provider.NewAlertmanager(src)
		case "grafana":
			providers[src.Name] = provider.NewGrafana(src)
		case "betterstack":
			providers[src.Name] = provider.NewBetterStack(src)
		case "prometheus":
			providers[src.Name] = provider.NewPrometheus(src)
		default:
			log.Printf("unknown provider type %q for source %s", src.Type, src.Name)
		}
	}
	return providers
}

func sourceNames(sources []config.SourceConfig) []string {
	names := make([]string, 0, len(sources))
	for _, src := range sources {
		names = append(names, src.Name)
	}
	return names
}
