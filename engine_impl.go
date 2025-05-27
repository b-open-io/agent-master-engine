package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// engineImpl is the concrete implementation of Engine interface
type engineImpl struct {
	storage      Storage
	config       *Config
	configPath   string
	destinations map[string]Destination
	autoSync     *autoSyncManager
	syncManager  *SyncManager
	eventBus     *eventBus
	validator    ServerValidator
	sanitizer    NameSanitizer
	mu           sync.RWMutex
}

// NewEngine creates a new engine instance
func NewEngine(opts ...Option) (Engine, error) {
	e := &engineImpl{
		destinations: make(map[string]Destination),
		eventBus:     newEventBus(),
	}

	// Apply options
	cfg := &engineConfig{
		storagePath: "~/.agent-master",
	}

	for _, opt := range opts {
		if opt != nil {
			if err := opt(cfg); err != nil {
				return nil, err
			}
		}
	}

	// Initialize storage
	if cfg.storage != nil {
		e.storage = cfg.storage
	} else {
		storage, err := NewFileStorage(cfg.storagePath)
		if err != nil {
			return nil, err
		}
		e.storage = storage
	}

	// Claude adapter is now optional and should be set explicitly if needed
	// e.SetClaudeAdapter(adapter)

	// Initialize sync manager
	e.syncManager = NewSyncManager(e)

	// Initialize auto-sync manager
	e.autoSync = newAutoSyncManager(e)

	// Initialize config
	e.config = &Config{
		Version: "1.0.2",
		Servers: make(map[string]ServerWithMetadata),
		Settings: Settings{
			AutoSync: AutoSyncSettings{
				Enabled:       false,
				WatchInterval: 1 * time.Second,
				DebounceDelay: 500 * time.Millisecond,
			},
			Backup: BackupSettings{
				Enabled:    true,
				Location:   "~/.agent-master/backups",
				MaxBackups: 10,
			},
			ConflictResolution: ConflictSettings{
				Mode: "interactive",
			},
			ProjectScanning: ProjectScanSettings{
				Enabled:      true,
				ScanPaths:    []string{"~/code"},
				ExcludePaths: []string{"node_modules", ".git", "dist"},
				MaxDepth:     5,
			},
			Validation: ValidationSettings{
				Enabled:             true,
				ValidateBeforeWrite: true,
				ValidateAfterWrite:  true,
				StrictMode:          false,
			},
		},
		Targets: make(map[string]TargetConfig), // Legacy field
	}

	return e, nil
}

// Configuration Management methods moved to config_manager.go

// Server Management methods moved to server_manager.go

// Target Management methods moved to destination_manager.go

// Helper methods
// NOTE: Default destinations are now handled by the presets package
// Use presets.LoadPreset("claude") instead of hardcoding platform specifics

// shouldSyncToTarget moved to destination_manager.go

// Helper functions moved to helpers.go

// Option configuration
type Option func(*engineConfig) error

type engineConfig struct {
	storage           Storage
	storagePath       string
	useDefaultTargets bool
}

func WithStorage(storage Storage) Option {
	return func(cfg *engineConfig) error {
		cfg.storage = storage
		return nil
	}
}

func WithFileStorage(path string) Option {
	return func(cfg *engineConfig) error {
		cfg.storagePath = path
		return nil
	}
}

func WithMemoryStorage() Option {
	return func(cfg *engineConfig) error {
		cfg.storage = NewMemoryStorage()
		return nil
	}
}

func WithDefaultTargets() Option {
	return func(cfg *engineConfig) error {
		cfg.useDefaultTargets = true
		return nil
	}
}

// Sync operations - delegate to sync manager
func (e *engineImpl) SyncToTarget(ctx context.Context, targetName string, options SyncOptions) (*SyncResult, error) {
	result, err := e.syncManager.SyncToTarget(ctx, targetName, options)
	if err == nil && result.Success {
		e.eventBus.emit(EventSyncCompleted, *result)
	} else if err != nil {
		e.eventBus.emit(EventSyncFailed, *result)
	}
	return result, err
}

func (e *engineImpl) SyncTo(ctx context.Context, dest Destination, options SyncOptions) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{
		Destination:    dest.GetID(),
		Success:        false,
		Changes:        []Change{},
		Errors:         []SyncError{},
		Duration:       0,
		Timestamp:      start,
		ServersAdded:   0,
		ServersUpdated: 0,
		ServersRemoved: 0,
	}

	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	// Transform config for destination
	transformedConfig, err := dest.Transform(config)
	if err != nil {
		result.Errors = append(result.Errors, SyncError{
			Error:       fmt.Sprintf("transform failed: %v", err),
			Recoverable: false,
		})
		return result, fmt.Errorf("failed to transform config: %w", err)
	}

	// Convert to JSON for writing
	data, err := json.Marshal(transformedConfig)
	if err != nil {
		result.Errors = append(result.Errors, SyncError{
			Error:       fmt.Sprintf("marshal failed: %v", err),
			Recoverable: false,
		})
		return result, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create backup if requested and destination supports it
	if options.CreateBackup && dest.SupportsBackup() && dest.Exists() && !options.DryRun {
		backupPath, err := dest.Backup()
		if err != nil {
			result.Errors = append(result.Errors, SyncError{
				Error:       fmt.Sprintf("backup failed: %v", err),
				Recoverable: true,
			})
		} else {
			result.BackupPath = backupPath
		}
	}

	// Calculate changes by comparing with existing
	if dest.Exists() {
		existingData, err := dest.Read()
		if err == nil {
			var existing interface{}
			json.Unmarshal(existingData, &existing)
			result.Changes = e.calculateChanges(existing, transformedConfig)
		}
	} else {
		// New destination - all servers are added
		result.ServersAdded = len(config.Servers)
		for name := range config.Servers {
			result.Changes = append(result.Changes, Change{
				Type:   "add",
				Server: name,
				After:  config.Servers[name].ServerConfig,
			})
		}
	}

	// Write if not dry run
	if !options.DryRun {
		if err := dest.Write(data); err != nil {
			result.Errors = append(result.Errors, SyncError{
				Error:       fmt.Sprintf("write failed: %v", err),
				Recoverable: false,
			})
			return result, fmt.Errorf("failed to write to destination: %w", err)
		}
	}

	result.Success = true
	result.Duration = time.Since(start)

	// Emit sync complete event
	e.eventBus.emit(EventSyncCompleted, *result)

	return result, nil
}

func (e *engineImpl) SyncToMultiple(ctx context.Context, dests []Destination, options SyncOptions) (*MultiSyncResult, error) {
	if len(dests) == 0 {
		return nil, fmt.Errorf("no destinations provided")
	}

	result := &MultiSyncResult{
		Results:       make([]SyncResult, 0, len(dests)),
		TotalDuration: 0,
		SuccessCount:  0,
		FailureCount:  0,
	}

	start := time.Now()
	
	// Use a wait group to sync concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for _, dest := range dests {
		wg.Add(1)
		go func(d Destination) {
			defer wg.Done()
			
			syncResult, err := e.SyncTo(ctx, d, options)
			
			mu.Lock()
			defer mu.Unlock()
			
			if err != nil {
				// Still add the result even if there was an error
				if syncResult == nil {
					syncResult = &SyncResult{
						Destination: d.GetID(),
						Success:     false,
						Errors: []SyncError{{
							Error:       err.Error(),
							Recoverable: false,
						}},
						Timestamp: time.Now(),
					}
				}
				result.FailureCount++
			} else if syncResult.Success {
				result.SuccessCount++
			} else {
				result.FailureCount++
			}
			
			result.Results = append(result.Results, *syncResult)
		}(dest)
	}
	
	wg.Wait()
	result.TotalDuration = time.Since(start)
	
	return result, nil
}

func (e *engineImpl) SyncToAllTargets(ctx context.Context, options SyncOptions) (*MultiSyncResult, error) {
	return e.syncManager.SyncToAllTargets(ctx, options)
}

func (e *engineImpl) GenerateTargetConfig(targetName string) (interface{}, error) {
	target, err := e.GetTarget(targetName)
	if err != nil {
		return nil, err
	}
	return e.syncManager.generateTargetConfig(targetName, target)
}

func (e *engineImpl) PreviewSync(dest Destination) (*SyncPreview, error) {
	e.mu.RLock()
	currentConfig := e.config
	e.mu.RUnlock()

	preview := &SyncPreview{
		Destination:    dest.GetID(),
		Changes:        []Change{},
		EstimatedTime:  100 * time.Millisecond, // Rough estimate
		RequiresBackup: false,
	}

	// Get the existing config at destination
	var existingConfig *Config
	if dest.Exists() {
		data, err := dest.Read()
		if err == nil && len(data) > 0 {
			// Try to parse existing config
			existingConfig, _ = ParseMCPConfig(data)
		}
	}

	// If destination exists and has data, we might want backup
	if existingConfig != nil && len(existingConfig.Servers) > 0 {
		preview.RequiresBackup = true
	}

	// Compare configs to determine changes
	if existingConfig == nil {
		// Everything is new
		for name, server := range currentConfig.Servers {
			if server.Internal.Enabled {
				preview.Changes = append(preview.Changes, Change{
					Type:   "add",
					Server: name,
					Before: nil,
					After:  server.ServerConfig,
				})
			}
		}
	} else {
		// Compare existing vs new
		// Check for updates and removals
		for name, existingServer := range existingConfig.Servers {
			if newServer, exists := currentConfig.Servers[name]; exists {
				if newServer.Internal.Enabled {
					// Check if changed (simple comparison for now)
					if !isServerEqual(existingServer.ServerConfig, newServer.ServerConfig) {
						preview.Changes = append(preview.Changes, Change{
							Type:   "update",
							Server: name,
							Before: existingServer.ServerConfig,
							After:  newServer.ServerConfig,
						})
					}
				} else {
					// Disabled, so remove
					preview.Changes = append(preview.Changes, Change{
						Type:   "remove",
						Server: name,
						Before: existingServer.ServerConfig,
						After:  nil,
					})
				}
			} else {
				// Server removed
				preview.Changes = append(preview.Changes, Change{
					Type:   "remove",
					Server: name,
					Before: existingServer.ServerConfig,
					After:  nil,
				})
			}
		}

		// Check for new servers
		for name, server := range currentConfig.Servers {
			if server.Internal.Enabled {
				if _, exists := existingConfig.Servers[name]; !exists {
					preview.Changes = append(preview.Changes, Change{
						Type:   "add",
						Server: name,
						Before: nil,
						After:  server.ServerConfig,
					})
				}
			}
		}
	}

	// Estimate time based on number of changes
	preview.EstimatedTime = time.Duration(len(preview.Changes)*50) * time.Millisecond

	return preview, nil
}

func (e *engineImpl) ScanForProjects(paths []string, detector ProjectDetector) ([]*ProjectConfig, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

func (e *engineImpl) RegisterProject(path string, config ProjectConfig) error {
	// TODO: Implement
	return fmt.Errorf("not implemented")
}

func (e *engineImpl) GetProjectConfig(path string) (*ProjectConfig, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

func (e *engineImpl) ListProjects() ([]*ProjectInfo, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}


// Import/Export and Backup methods moved to import_export.go and backup_manager.go

func (e *engineImpl) OnConfigChange(handler ConfigChangeHandler) func() {
	// Subscribe to all config-related events
	unsubscribers := []func(){
		e.eventBus.on(EventConfigLoaded, handler),
		e.eventBus.on(EventConfigSaved, handler),
		e.eventBus.on(EventConfigChanged, handler),
		e.eventBus.on(EventAutoSyncStarted, handler),
		e.eventBus.on(EventAutoSyncStopped, handler),
		e.eventBus.on(EventFileChanged, handler),
	}
	
	// Return a function that unsubscribes from all
	return func() {
		for _, unsub := range unsubscribers {
			unsub()
		}
	}
}

func (e *engineImpl) OnSyncComplete(handler SyncCompleteHandler) func() {
	return e.eventBus.on(EventSyncCompleted, handler)
}

func (e *engineImpl) OnError(handler ErrorHandler) func() {
	// TODO: Implement
	return func() {}
}

// calculateChanges compares existing and transformed configs to detect changes
func (e *engineImpl) calculateChanges(existing, transformed interface{}) []Change {
	changes := []Change{}

	// Extract servers from both configs
	existingServers := make(map[string]interface{})
	transformedServers := make(map[string]interface{})

	// Handle different config formats
	switch ex := existing.(type) {
	case map[string]interface{}:
		if servers, ok := ex["mcpServers"].(map[string]interface{}); ok {
			existingServers = servers
		}
	case *Config:
		for name, server := range ex.Servers {
			existingServers[name] = server.ServerConfig
		}
	}

	switch tr := transformed.(type) {
	case map[string]interface{}:
		if servers, ok := tr["mcpServers"].(map[string]interface{}); ok {
			transformedServers = servers
		}
	case *Config:
		for name, server := range tr.Servers {
			transformedServers[name] = server.ServerConfig
		}
	}

	// Find added servers
	for name, server := range transformedServers {
		if _, exists := existingServers[name]; !exists {
			changes = append(changes, Change{
				Type:   "add",
				Server: name,
				After:  server,
			})
		}
	}

	// Find updated servers
	for name, transformedServer := range transformedServers {
		if existingServer, exists := existingServers[name]; exists {
			// Simple comparison - in real implementation would do deep comparison
			existingJSON, _ := json.Marshal(existingServer)
			transformedJSON, _ := json.Marshal(transformedServer)
			if string(existingJSON) != string(transformedJSON) {
				changes = append(changes, Change{
					Type:   "update",
					Server: name,
					Before: existingServer,
					After:  transformedServer,
				})
			}
		}
	}

	// Find removed servers
	for name := range existingServers {
		if _, exists := transformedServers[name]; !exists {
			changes = append(changes, Change{
				Type:   "remove",
				Server: name,
				Before: existingServers[name],
			})
		}
	}

	return changes
}

// Destination Management methods moved to destination_manager.go

// isServerEqual compares two ServerConfig instances
func isServerEqual(a, b ServerConfig) bool {
	// Basic comparison - can be enhanced
	if a.Transport != b.Transport || a.Command != b.Command || a.URL != b.URL {
		return false
	}
	
	// Compare args
	if len(a.Args) != len(b.Args) {
		return false
	}
	for i := range a.Args {
		if a.Args[i] != b.Args[i] {
			return false
		}
	}
	
	// Compare env vars
	if len(a.Env) != len(b.Env) {
		return false
	}
	for k, v := range a.Env {
		if bv, ok := b.Env[k]; !ok || v != bv {
			return false
		}
	}
	
	// Compare headers
	if len(a.Headers) != len(b.Headers) {
		return false
	}
	for k, v := range a.Headers {
		if bv, ok := b.Headers[k]; !ok || v != bv {
			return false
		}
	}
	
	return true
}

