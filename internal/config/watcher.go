package config

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// OnChangeFunc is called when the config file changes and produced a usable
// config. Diagnostics may be non-empty: entries that could not be loaded were
// dropped from cfg.
type OnChangeFunc func(*Config, Diagnostics)

// OnFailureFunc is called when a changed config file could not be read or
// parsed at all. The caller keeps its running config — tearing a working
// session down because of a half-saved file would be destructive.
type OnFailureFunc func(error)

// Watch starts a file watcher on the config path, calling onChange for every
// successful reload and onFailure when a reload could not produce a config.
func Watch(path string, onChange OnChangeFunc, onFailure OnFailureFunc) (stop func(), err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		watcher.Close()
		return nil, err
	}
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}

	done := make(chan struct{})
	go func() {
		defer watcher.Close()
		// Debounce: wait for quiet period before reloading
		var debounce <-chan time.Time
		for {
			select {
			case <-done:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Base(event.Name) != base {
					continue
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					debounce = time.After(300 * time.Millisecond)
				}
			case <-debounce:
				debounce = nil
				cfg, diags, err := Load(path)
				if err != nil {
					log.Printf("config: reload failed, keeping running config: %v", err)
					if onFailure != nil {
						onFailure(err)
					}
					continue
				}
				log.Printf("config: reloaded from %s", path)
				for _, diag := range diags {
					log.Printf("config: %s: %s", diag.Field, diag.Message)
				}
				onChange(cfg, diags)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("config: watcher error: %v", err)
			}
		}
	}()

	return func() { close(done) }, nil
}
