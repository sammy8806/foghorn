package config

import (
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

// OnChangeFunc is called when the config file changes with the new config.
type OnChangeFunc func(*Config)

// Watch starts a file watcher on the config path and calls onChange when
// the config is successfully reloaded. Runs until ctx is cancelled.
func Watch(path string, onChange OnChangeFunc) (stop func(), err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(path); err != nil {
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
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					debounce = time.After(300 * time.Millisecond)
				}
			case <-debounce:
				cfg, err := Load(path)
				if err != nil {
					log.Printf("config: reload failed: %v", err)
					continue
				}
				log.Printf("config: reloaded from %s", path)
				onChange(cfg)
				debounce = nil
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
