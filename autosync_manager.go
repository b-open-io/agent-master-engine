package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// fileWatcher wraps fsnotify.Watcher for easier testing
type fileWatcher struct {
	watcher *fsnotify.Watcher
}

func newFileWatcher() (*fileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &fileWatcher{watcher: watcher}, nil
}

func (fw *fileWatcher) Add(path string) error {
	return fw.watcher.Add(path)
}

func (fw *fileWatcher) Remove(path string) error {
	return fw.watcher.Remove(path)
}

func (fw *fileWatcher) Close() error {
	return fw.watcher.Close()
}

// autoSyncManager handles automatic configuration synchronization
type autoSyncManager struct {
	engine        *engineImpl
	config        AutoSyncConfig
	isRunning     bool
	stopChan      chan struct{}
	watcher       *fileWatcher
	debounceTimer *time.Timer
	lastSync      time.Time
	mu            sync.Mutex
	wg            sync.WaitGroup
}

// newAutoSyncManager creates a new auto-sync manager
func newAutoSyncManager(engine *engineImpl) *autoSyncManager {
	return &autoSyncManager{
		engine: engine,
	}
}

// StartAutoSync starts automatic synchronization
func (e *engineImpl) StartAutoSync(config AutoSyncConfig) error {
	return e.autoSync.Start(config)
}

// StopAutoSync stops automatic synchronization
func (e *engineImpl) StopAutoSync() error {
	return e.autoSync.Stop()
}

// GetAutoSyncStatus returns the current auto-sync status
func (e *engineImpl) GetAutoSyncStatus() (*AutoSyncStatus, error) {
	return e.autoSync.GetStatus()
}

// Start starts the auto-sync manager
func (asm *autoSyncManager) Start(config AutoSyncConfig) error {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	if asm.isRunning {
		return fmt.Errorf("auto-sync is already running")
	}

	// Create file watcher
	watcher, err := newFileWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	asm.config = config
	asm.watcher = watcher
	asm.isRunning = true
	asm.stopChan = make(chan struct{})

	// Update and persist auto-sync settings in engine config
	asm.engine.mu.Lock()
	asm.engine.config.Settings.AutoSync.Enabled = true
	asm.engine.config.Settings.AutoSync.WatchInterval = config.WatchInterval
	asm.engine.config.Settings.AutoSync.DebounceDelay = config.DebounceDelay
	asm.engine.config.Settings.AutoSync.Destinations = config.TargetWhitelist
	// Save config without holding the lock
	saveErr := asm.engine.saveConfigNoLock()
	asm.engine.mu.Unlock()

	if saveErr != nil {
		// Log warning but don't fail the start operation
		asm.engine.eventBus.emit(EventWarning, fmt.Sprintf("failed to persist auto-sync settings: %v", saveErr))
	}

	// Watch the config file path
	configPath := asm.engine.configPath
	if configPath == "" {
		// Try to use storage path
		if storage, ok := asm.engine.storage.(*FileStorage); ok {
			configPath = filepath.Join(storage.GetBasePath(), "config.json")
		}
	}

	if configPath != "" {
		// Ensure the config file exists
		if _, err := os.Stat(configPath); err == nil {
			if err := asm.watcher.Add(configPath); err != nil {
				asm.watcher.Close()
				asm.isRunning = false
				return fmt.Errorf("failed to watch config file: %w", err)
			}

			// Also watch the directory for new files
			dir := filepath.Dir(configPath)
			if err := asm.watcher.Add(dir); err != nil {
				// Non-fatal: log warning but continue
				asm.engine.eventBus.emit(EventWarning, fmt.Sprintf("failed to watch directory %s: %v", dir, err))
			}
		}
	}

	// Start the watcher goroutine
	asm.wg.Add(1)
	go func() {
		defer asm.wg.Done()
		asm.watchLoop()
	}()

	// Emit event
	asm.engine.eventBus.emit(EventAutoSyncStarted, ConfigChange{
		Type:      "autosync-started",
		Timestamp: time.Now(),
		Source:    "autosync",
	})

	return nil
}

// Stop stops the auto-sync manager
func (asm *autoSyncManager) Stop() error {
	asm.mu.Lock()
	
	if !asm.isRunning {
		asm.mu.Unlock()
		return fmt.Errorf("auto-sync is not running")
	}

	// Signal stop
	close(asm.stopChan)
	asm.isRunning = false
	asm.mu.Unlock()

	// Wait for goroutines to finish
	asm.wg.Wait()

	// Close watcher
	if asm.watcher != nil {
		asm.watcher.Close()
	}

	// Cancel any pending timer
	asm.mu.Lock()
	if asm.debounceTimer != nil {
		asm.debounceTimer.Stop()
		asm.debounceTimer = nil
	}
	asm.mu.Unlock()

	// Update and persist auto-sync disabled state in engine config
	asm.engine.mu.Lock()
	asm.engine.config.Settings.AutoSync.Enabled = false
	// Keep other settings intact for when it's re-enabled
	saveErr := asm.engine.saveConfigNoLock()
	asm.engine.mu.Unlock()

	if saveErr != nil {
		// Log warning but don't fail the stop operation
		asm.engine.eventBus.emit(EventWarning, fmt.Sprintf("failed to persist auto-sync disabled state: %v", saveErr))
	}

	// Emit event
	asm.engine.eventBus.emit(EventAutoSyncStopped, ConfigChange{
		Type:      "autosync-stopped",
		Timestamp: time.Now(),
		Source:    "autosync",
	})

	return nil
}

// GetStatus returns the current auto-sync status
func (asm *autoSyncManager) GetStatus() (*AutoSyncStatus, error) {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	// Get persisted state from engine config
	asm.engine.mu.RLock()
	enabled := asm.engine.config.Settings.AutoSync.Enabled
	watchInterval := asm.engine.config.Settings.AutoSync.WatchInterval
	asm.engine.mu.RUnlock()

	// Use runtime values if they're set, otherwise use persisted values
	if asm.config.WatchInterval > 0 {
		watchInterval = asm.config.WatchInterval
	}

	status := &AutoSyncStatus{
		Running:       asm.isRunning,
		LastSync:      asm.lastSync,
		Enabled:       enabled,
		WatchInterval: watchInterval,
	}

	return status, nil
}

// watchLoop is the main watch loop
func (asm *autoSyncManager) watchLoop() {
	for {
		select {
		case <-asm.stopChan:
			return
			
		case event, ok := <-asm.watcher.watcher.Events:
			if !ok {
				return
			}

			// Skip ignored events
			if asm.shouldIgnoreEvent(event) {
				continue
			}

			// Emit file change event
			asm.engine.eventBus.emit(EventFileChanged, ConfigChange{
				Type:      asm.getChangeType(event),
				Timestamp: time.Now(),
				Source:    "file-watcher",
				Name:      event.Name,
			})

			// Debounce sync
			asm.debouncedSync()

		case err, ok := <-asm.watcher.watcher.Errors:
			if !ok {
				return
			}
			
			// Emit error
			asm.engine.eventBus.emit(EventError, err)
		}
	}
}

// shouldIgnoreEvent checks if an event should be ignored
func (asm *autoSyncManager) shouldIgnoreEvent(event fsnotify.Event) bool {
	// Ignore chmod events
	if event.Op&fsnotify.Chmod == fsnotify.Chmod {
		return true
	}

	// Check ignore patterns
	for _, pattern := range asm.config.IgnorePatterns {
		if matched, err := filepath.Match(pattern, filepath.Base(event.Name)); err == nil && matched {
			return true
		}
	}

	// Don't ignore write or create events on config files
	if strings.HasSuffix(event.Name, ".json") && 
		(event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
		return false
	}

	return false
}

// getChangeType returns a string representation of the change type
func (asm *autoSyncManager) getChangeType(event fsnotify.Event) string {
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		return "created"
	case event.Op&fsnotify.Write == fsnotify.Write:
		return "modified"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		return "removed"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		return "renamed"
	default:
		return "unknown"
	}
}

// debouncedSync performs a debounced sync operation
func (asm *autoSyncManager) debouncedSync() {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	// Cancel existing timer
	if asm.debounceTimer != nil {
		asm.debounceTimer.Stop()
	}

	// Set new timer
	asm.debounceTimer = time.AfterFunc(asm.config.DebounceDelay, func() {
		asm.performSync()
	})
}

// performSync performs the actual sync operation
func (asm *autoSyncManager) performSync() {
	// Reload config first
	if err := asm.engine.LoadConfig(asm.engine.configPath); err != nil {
		asm.engine.eventBus.emit(EventError, fmt.Errorf("failed to reload config: %w", err))
		return
	}

	// Get destinations to sync
	destinations := asm.getDestinationsToSync()
	if len(destinations) == 0 {
		return
	}

	// Create sync options
	options := SyncOptions{
		DryRun:       false,
		Force:        false,
		CreateBackup: false,
	}

	// Sync to each destination
	ctx := context.Background()
	for _, destName := range destinations {
		// Get destination
		dest, err := asm.engine.GetDestination(destName)
		if err != nil {
			asm.engine.eventBus.emit(EventError, fmt.Errorf("failed to get destination %s: %w", destName, err))
			continue
		}

		result, err := asm.engine.SyncTo(ctx, dest, options)
		if err != nil {
			asm.engine.eventBus.emit(EventSyncFailed, *result)
		} else {
			asm.engine.eventBus.emit(EventSyncCompleted, *result)
		}
	}

	// Update last sync time
	asm.mu.Lock()
	asm.lastSync = time.Now()
	asm.mu.Unlock()
}

// getDestinationsToSync returns the list of destinations to sync to
func (asm *autoSyncManager) getDestinationsToSync() []string {
	var destinations []string

	// Use configured destinations if specified
	if len(asm.config.TargetWhitelist) > 0 {
		destinations = asm.config.TargetWhitelist
	} else {
		// Otherwise sync to all registered destinations
		for name := range asm.engine.destinations {
			destinations = append(destinations, name)
		}
	}

	// Apply blacklist filter
	if len(asm.config.TargetBlacklist) > 0 {
		filtered := []string{}
		blacklist := make(map[string]bool)
		for _, bl := range asm.config.TargetBlacklist {
			blacklist[bl] = true
		}

		for _, dest := range destinations {
			if !blacklist[dest] {
				filtered = append(filtered, dest)
			}
		}
		destinations = filtered
	}

	return destinations
}