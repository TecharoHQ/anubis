package lib

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type AllowedTargets struct {
	AllowedTargets []string `json:"allowed_targets"`
	mu             sync.RWMutex
	watcher        *fsnotify.Watcher
	filePath       string
}

func LoadAllowedTargets(fname string) (*AllowedTargets, error) {
	if fname == "" {
		return nil, nil
	}

	slog.Debug("loading allowed targets file", "file", fname)

	targets := &AllowedTargets{
		filePath: fname,
	}

	if err := targets.loadFromFile(); err != nil {
		return nil, err
	}

	if err := targets.setupWatcher(); err != nil {
		slog.Warn("could not set up file watcher for allowed targets", "error", err)
		return targets, nil
	}

	return targets, nil
}

func (a *AllowedTargets) loadFromFile() error {
	fin, err := os.Open(a.filePath)
	if err != nil {
		return fmt.Errorf("can't open allowed targets file %s: %w", a.filePath, err)
	}
	defer fin.Close()

	var targets struct {
		AllowedTargets []string `json:"allowed_targets"`
	}

	if err := json.NewDecoder(fin).Decode(&targets); err != nil {
		return fmt.Errorf("can't parse allowed targets JSON %s: %w", a.filePath, err)
	}

	a.mu.Lock()
	a.AllowedTargets = targets.AllowedTargets
	a.mu.Unlock()

	return nil
}

func (a *AllowedTargets) setupWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error creating file watcher: %w", err)
	}

	go func() {
		debounceTimer := time.NewTimer(0)
		if !debounceTimer.Stop() {
			<-debounceTimer.C
		}

		var debouncing bool

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					if !debouncing {
						debouncing = true
						debounceTimer.Reset(100 * time.Millisecond)
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("error watching allowed targets file", "error", err)

			case <-debounceTimer.C:
				debouncing = false
				slog.Info("reloading allowed targets file", "file", a.filePath)
				if err := a.loadFromFile(); err != nil {
					slog.Error("error reloading allowed targets file", "error", err)
				}
			}
		}
	}()

	if err := watcher.Add(a.filePath); err != nil {
		watcher.Close()
		return fmt.Errorf("error watching file %s: %w", a.filePath, err)
	}

	a.watcher = watcher
	return nil
}

func (a *AllowedTargets) Close() error {
	if a.watcher != nil {
		return a.watcher.Close()
	}
	return nil
}

func (a *AllowedTargets) IsAllowed(target string) bool {
	if a == nil {
		return false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, allowed := range a.AllowedTargets {
		if allowed == target {
			return true
		}
	}
	return false
}