package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Configuration Management functionality for engineImpl

// LoadConfig loads configuration from the specified path
func (e *engineImpl) LoadConfig(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.configPath = path

	// If a file path is provided and exists, load from file
	if path != "" {
		expandedPath := expandPath(path)
		if data, err := os.ReadFile(expandedPath); err == nil {
			// Parse the config from file
			var fileConfig Config
			if err := json.Unmarshal(data, &fileConfig); err != nil {
				return fmt.Errorf("failed to parse config from file: %w", err)
			}
			
			// Update the engine's config
			e.config = &fileConfig
			
			// Ensure servers map is initialized
			if e.config.Servers == nil {
				e.config.Servers = make(map[string]ServerWithMetadata)
			}
			
			// Also save to storage for consistency
			if err := SaveJSON(e.storage, Keys.Config(), e.config); err != nil {
				// Log but don't fail - file is the source of truth
				e.eventBus.emit(EventWarning, fmt.Sprintf("failed to save config to storage: %v", err))
			}
			
			// Emit event
			e.eventBus.emit(EventConfigLoaded, ConfigChange{
				Type:      "config-loaded",
				Timestamp: time.Now(),
				Source:    "file",
			})
			
			return nil
		}
	}

	// Fall back to loading from storage
	if err := LoadJSON(e.storage, Keys.Config(), &e.config); err != nil {
		// If not found, that's OK - we'll use defaults
		if !isNotFoundError(err) {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Emit event
	e.eventBus.emit(EventConfigLoaded, ConfigChange{
		Type:      "config-loaded",
		Timestamp: time.Now(),
		Source:    "storage",
	})

	return nil
}

// SaveConfig saves the current configuration
func (e *engineImpl) SaveConfig() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.saveConfigNoLock()
}

// saveConfigNoLock saves config without acquiring lock (caller must hold lock)
func (e *engineImpl) saveConfigNoLock() error {
	if err := SaveJSON(e.storage, Keys.Config(), e.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	e.eventBus.emit(EventConfigSaved, ConfigChange{
		Type:      "config-saved",
		Timestamp: time.Now(),
		Source:    "user",
	})

	return nil
}

// GetConfig returns a copy of the current configuration
func (e *engineImpl) GetConfig() (*Config, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return copy to prevent external modification
	configCopy := *e.config
	return &configCopy, nil
}

// SetConfigPath sets the configuration file path
func (e *engineImpl) SetConfigPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.configPath = path
}

// SetConfig replaces the entire configuration
func (e *engineImpl) SetConfig(config *Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	e.config = config
	err := e.saveConfigNoLock()
	
	// Trigger auto-sync if enabled
	if err == nil && e.autoSync != nil && e.autoSync.isRunning {
		go e.autoSync.debouncedSync()
	}
	
	return err
}