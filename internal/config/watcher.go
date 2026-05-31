package config

import (
	"log/slog"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ReloadFunc is called when the config file changes with the newly loaded Config.
type ReloadFunc func(*Config) error

// Watch monitors the config file for changes and calls onReload when it does.
// Runs until the returned stop function is called.
// Uses a debounce timer to avoid reacting to partial writes (editor save patterns).
func Watch(path string, onReload ReloadFunc, logger *slog.Logger) (stop func(), err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return nil, err
	}

	stopCh := make(chan struct{})
	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	go func() {
		defer watcher.Close()
		defer debounce.Stop()

		for {
			select {
			case <-stopCh:
				return

			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// React to write events (including symlink writes from tools like vim)
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				logger.Info("config file changed, scheduling reload", "event", event.Name)
				debounce.Reset(500 * time.Millisecond)

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("config watcher error", "error", err)

			case <-debounce.C:
				logger.Info("reloading config", "path", path)
				cfg, err := Load(path)
				if err != nil {
					logger.Error("config reload failed, keeping previous config", "error", err)
					continue
				}
				if err := onReload(cfg); err != nil {
					logger.Error("config reload handler failed, keeping previous config", "error", err)
				} else {
					logger.Info("config reloaded successfully")
				}
			}
		}
	}()

	stop = func() {
		close(stopCh)
	}
	return stop, nil
}