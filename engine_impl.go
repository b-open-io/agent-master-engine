package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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
		Version: "1.0.1",
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

// Configuration Management
func (e *engineImpl) LoadConfig(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.configPath = path

	// Try to load from storage
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

func (e *engineImpl) GetConfig() (*Config, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return copy to prevent external modification
	configCopy := *e.config
	return &configCopy, nil
}

func (e *engineImpl) SetConfigPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.configPath = path
}

func (e *engineImpl) SetConfig(config *Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	e.config = config
	return e.saveConfigNoLock()
}

// ValidateServer validates a server configuration
func ValidateServer(name string, config ServerConfig) error {
	// Basic validation
	if name == "" {
		return fmt.Errorf("server name cannot be empty")
	}
	if config.Transport != "stdio" && config.Transport != "sse" {
		return fmt.Errorf("invalid transport: %s", config.Transport)
	}
	if config.Transport == "stdio" && config.Command == "" {
		return fmt.Errorf("stdio transport requires command")
	}
	if config.Transport == "sse" && config.URL == "" {
		return fmt.Errorf("sse transport requires URL")
	}
	return nil
}

// SanitizeServerName sanitizes a server name
func SanitizeServerName(name string) string {
	// Basic sanitization - remove spaces and special characters
	sanitized := strings.TrimSpace(name)
	// Replace spaces with hyphens
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	// Remove any non-alphanumeric characters except hyphens and underscores
	// This is a simple implementation - can be enhanced with regex
	return sanitized
}

// Server Management
func (e *engineImpl) AddServer(name string, server ServerConfig) error {
	// Validate server
	if err := ValidateServer(name, server); err != nil {
		return err
	}

	// Optional: Test server if validator supports it
	// TODO: Add server testing capability to validator interface

	e.mu.Lock()
	defer e.mu.Unlock()

	// Check for duplicates
	if _, exists := e.config.Servers[name]; exists {
		return fmt.Errorf("server %q already exists", name)
	}

	// Add with default metadata
	e.config.Servers[name] = ServerWithMetadata{
		ServerConfig: server,
		Internal: InternalMetadata{
			Enabled:      true,
			SyncTargets:  []string{"all"},
			Source:       "user",
			LastModified: time.Now(),
		},
	}

	// Save config (without lock since we already hold it)
	if err := e.saveConfigNoLock(); err != nil {
		return err
	}

	e.eventBus.emit(EventServerAdded, ConfigChange{
		Type:      "server-added",
		Name:      name,
		Timestamp: time.Now(),
		Source:    "user",
	})

	return nil
}

func (e *engineImpl) UpdateServer(name string, server ServerConfig) error {
	// Validate server
	if err := ValidateServer(name, server); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.config.Servers[name]
	if !exists {
		return fmt.Errorf("server %q not found", name)
	}

	// Update config, preserve metadata
	existing.ServerConfig = server
	existing.Internal.LastModified = time.Now()
	e.config.Servers[name] = existing

	// Save config (without lock since we already hold it)
	if err := e.saveConfigNoLock(); err != nil {
		return err
	}

	e.eventBus.emit(EventServerUpdated, ConfigChange{
		Type:      "server-updated",
		Name:      name,
		Timestamp: time.Now(),
		Source:    "user",
	})

	return nil
}

func (e *engineImpl) RemoveServer(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.config.Servers[name]; !exists {
		return fmt.Errorf("server %q not found", name)
	}

	delete(e.config.Servers, name)

	// Save config (without lock since we already hold it)
	if err := e.saveConfigNoLock(); err != nil {
		return err
	}

	e.eventBus.emit(EventServerRemoved, ConfigChange{
		Type:      "server-removed",
		Name:      name,
		Timestamp: time.Now(),
		Source:    "user",
	})

	return nil
}

func (e *engineImpl) GetServer(name string) (*ServerWithMetadata, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	server, exists := e.config.Servers[name]
	if !exists {
		return nil, fmt.Errorf("server %q not found", name)
	}

	// Return copy
	serverCopy := server
	return &serverCopy, nil
}

func (e *engineImpl) ListServers(filter ServerFilter) ([]*ServerInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var servers []*ServerInfo

	for name, server := range e.config.Servers {
		// Apply filters
		if filter.Enabled != nil && server.Internal.Enabled != *filter.Enabled {
			continue
		}

		if filter.Transport != "" && server.Transport != filter.Transport {
			continue
		}

		if filter.Source != "" && server.Internal.Source != filter.Source {
			continue
		}

		// TODO: Implement other filters

		info := &ServerInfo{
			Name:            name,
			Transport:       server.Transport,
			Enabled:         server.Internal.Enabled,
			SyncTargetCount: len(server.Internal.SyncTargets),
			LastModified:    server.Internal.LastModified,
			HasErrors:       server.Internal.ErrorCount > 0,
		}

		servers = append(servers, info)
	}

	return servers, nil
}

func (e *engineImpl) ValidateServer(name string, server ServerConfig) error {
	return ValidateServer(name, server)
}

func (e *engineImpl) SanitizeServerName(name string) string {
	return SanitizeServerName(name)
}

func (e *engineImpl) SanitizeName(name string) string {
	// Use sanitizer if set
	if e.sanitizer != nil {
		return e.sanitizer.Sanitize(name)
	}
	// Fall back to basic sanitization
	return SanitizeServerName(name)
}

func (e *engineImpl) SetSanitizer(sanitizer NameSanitizer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sanitizer = sanitizer
}

func (e *engineImpl) SetValidator(validator ServerValidator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.validator = validator
}

// Target Management (Legacy - use Destinations instead)
func (e *engineImpl) RegisterTarget(target TargetConfig) error {
	// Legacy method - targets are now handled as destinations
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.Targets == nil {
		e.config.Targets = make(map[string]TargetConfig)
	}
	e.config.Targets[target.Name] = target

	return e.saveConfigNoLock()
}

func (e *engineImpl) RemoveTarget(name string) error {
	// Legacy method - targets are now handled as destinations
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.Targets == nil {
		return fmt.Errorf("target %q not found", name)
	}

	if _, exists := e.config.Targets[name]; !exists {
		return fmt.Errorf("target %q not found", name)
	}

	delete(e.config.Targets, name)

	return e.saveConfigNoLock()
}

func (e *engineImpl) GetTarget(name string) (*TargetConfig, error) {
	// Legacy method
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config.Targets == nil {
		return nil, fmt.Errorf("target %q not found", name)
	}

	target, exists := e.config.Targets[name]
	if !exists {
		return nil, fmt.Errorf("target %q not found", name)
	}

	targetCopy := target
	return &targetCopy, nil
}

func (e *engineImpl) ListTargets() ([]*TargetInfo, error) {
	// Legacy method
	e.mu.RLock()
	defer e.mu.RUnlock()

	var targets []*TargetInfo

	if e.config.Targets != nil {
		for name, target := range e.config.Targets {
			info := &TargetInfo{
				Name:       name,
				Type:       target.Type,
				Enabled:    target.Enabled,
				ConfigPath: target.ConfigPath,
			}

			// Count servers for this target
			count := 0
			for _, server := range e.config.Servers {
				if server.Internal.Enabled && e.shouldSyncToTarget(server, name) {
					count++
				}
			}
			info.ServerCount = count

			targets = append(targets, info)
		}
	}

	return targets, nil
}

// Helper methods
// NOTE: Default destinations are now handled by the presets package
// Use presets.LoadPreset("claude") instead of hardcoding platform specifics

func (e *engineImpl) shouldSyncToTarget(server ServerWithMetadata, target string) bool {
	// Check exclusions first
	for _, excluded := range server.Internal.ExcludeFromTargets {
		if excluded == target {
			return false
		}
	}

	// Check inclusions
	for _, included := range server.Internal.SyncTargets {
		if included == "all" || included == target {
			return true
		}
	}

	return false
}

// Helper functions
func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

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

func WithDefaultTargets() Option {
	return func(cfg *engineConfig) error {
		cfg.useDefaultTargets = true
		return nil
	}
}

// Event bus implementation
type eventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]interface{}
}

func newEventBus() *eventBus {
	return &eventBus{
		handlers: make(map[EventType][]interface{}),
	}
}

func (eb *eventBus) on(event EventType, handler interface{}) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[event] = append(eb.handlers[event], handler)

	// Return unsubscribe function
	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()

		if handlers, ok := eb.handlers[event]; ok {
			for i, h := range handlers {
				if &h == &handler {
					eb.handlers[event] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}
		}
	}
}

func (eb *eventBus) emit(event EventType, data interface{}) {
	eb.mu.RLock()
	handlers := eb.handlers[event]
	eb.mu.RUnlock()

	for _, handler := range handlers {
		// Call handler in goroutine to prevent blocking
		go func(h interface{}) {
			switch event {
			case EventConfigLoaded, EventConfigSaved, EventServerAdded, EventServerUpdated, EventServerRemoved:
				if fn, ok := h.(func(ConfigChange)); ok {
					fn(data.(ConfigChange))
				}
				// Add other event types...
			}
		}(handler)
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
	// TODO: Implement sync to multiple destinations
	return nil, fmt.Errorf("not implemented")
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
	// TODO: Implement preview for destination
	return nil, fmt.Errorf("not implemented")
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

func (e *engineImpl) StartAutoSync(config AutoSyncConfig) error {
	return e.autoSync.Start(config)
}

func (e *engineImpl) StopAutoSync() error {
	return e.autoSync.Stop()
}

func (e *engineImpl) GetAutoSyncStatus() (*AutoSyncStatus, error) {
	return e.autoSync.GetStatus()
}

func (e *engineImpl) ImportFromTarget(targetName string, options ImportOptions) (*ImportResult, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

func (e *engineImpl) ExportToFile(path string, format ExportFormat) error {
	// TODO: Implement
	return fmt.Errorf("not implemented")
}

func (e *engineImpl) MergeConfigs(configs ...*Config) (*Config, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

func (e *engineImpl) CreateBackup(description string) (*BackupInfo, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

func (e *engineImpl) ListBackups() ([]*BackupInfo, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

func (e *engineImpl) RestoreBackup(backupID string) error {
	// TODO: Implement
	return fmt.Errorf("not implemented")
}

func (e *engineImpl) Export(format ExportFormat) ([]byte, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

func (e *engineImpl) Import(data []byte, format ImportFormat, options ImportOptions) error {
	// TODO: Implement
	return fmt.Errorf("not implemented")
}

func (e *engineImpl) OnConfigChange(handler ConfigChangeHandler) func() {
	return e.eventBus.on(EventConfigLoaded, handler)
}

func (e *engineImpl) OnSyncComplete(handler SyncCompleteHandler) func() {
	return e.eventBus.on(EventSyncCompleted, handler)
}

func (e *engineImpl) OnError(handler ErrorHandler) func() {
	// TODO: Implement
	return func() {}
}

// fileWatcher wraps fsnotify functionality
type fileWatcher struct {
	watcher *fsnotify.Watcher
	paths   map[string]bool
	mu      sync.RWMutex
}

// newFileWatcher creates a new file watcher
func newFileWatcher() (*fileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &fileWatcher{
		watcher: watcher,
		paths:   make(map[string]bool),
	}, nil
}

// Add adds a path to watch
func (fw *fileWatcher) Add(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Check if already watching
	if fw.paths[absPath] {
		return nil
	}

	if err := fw.watcher.Add(absPath); err != nil {
		return err
	}

	fw.paths[absPath] = true
	return nil
}

// Remove removes a path from watching
func (fw *fileWatcher) Remove(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if !fw.paths[absPath] {
		return nil
	}

	if err := fw.watcher.Remove(absPath); err != nil {
		return err
	}

	delete(fw.paths, absPath)
	return nil
}

// Close closes the watcher
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
		engine:   engine,
		stopChan: make(chan struct{}),
	}
}

// calculateChanges compares existing and transformed configs to find differences
func (e *engineImpl) calculateChanges(existing, transformed interface{}) []Change {
	changes := []Change{}

	// Convert to maps for comparison
	existingMap, ok1 := existing.(map[string]interface{})
	transformedMap, ok2 := transformed.(map[string]interface{})

	if !ok1 || !ok2 {
		return changes
	}

	// Get servers from both configs
	existingServers, _ := existingMap["mcpServers"].(map[string]interface{})
	transformedServers, _ := transformedMap["mcpServers"].(map[string]interface{})

	if existingServers == nil {
		existingServers = make(map[string]interface{})
	}
	if transformedServers == nil {
		transformedServers = make(map[string]interface{})
	}

	// Find added servers
	for name := range transformedServers {
		if _, exists := existingServers[name]; !exists {
			changes = append(changes, Change{
				Type:   "add",
				Server: name,
				After:  transformedServers[name],
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

// Destination Management
func (e *engineImpl) RegisterDestination(name string, dest Destination) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.destinations[name] = dest
	return nil
}

func (e *engineImpl) RemoveDestination(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.destinations[name]; !exists {
		return fmt.Errorf("destination %q not found", name)
	}

	delete(e.destinations, name)
	return nil
}

func (e *engineImpl) GetDestination(name string) (Destination, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dest, exists := e.destinations[name]
	if !exists {
		return nil, fmt.Errorf("destination %q not found", name)
	}

	return dest, nil
}

func (e *engineImpl) ListDestinations() map[string]Destination {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return copy to prevent external modification
	result := make(map[string]Destination)
	for name, dest := range e.destinations {
		result[name] = dest
	}

	return result
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
		return nil
	}

	// Signal stop
	close(asm.stopChan)
	
	// Cancel any pending debounce timer
	if asm.debounceTimer != nil {
		asm.debounceTimer.Stop()
		asm.debounceTimer = nil
	}
	
	// Close watcher before unlocking to ensure it's closed before goroutine exits
	if asm.watcher != nil {
		asm.watcher.Close()
	}
	
	// Mark as not running before unlocking so performSync will exit early
	asm.isRunning = false
	asm.mu.Unlock()
	
	// Wait for goroutine to finish
	asm.wg.Wait()
	
	// Now safe to nil the watcher
	asm.mu.Lock()
	asm.watcher = nil
	asm.mu.Unlock()

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

	status := &AutoSyncStatus{
		Enabled:       asm.config.Enabled,
		Running:       asm.isRunning,
		LastSync:      asm.lastSync,
		WatchInterval: asm.config.WatchInterval,
	}

	// Calculate next sync time if running
	if asm.isRunning && !asm.lastSync.IsZero() {
		status.NextSync = asm.lastSync.Add(asm.config.WatchInterval)
	}

	return status, nil
}

// watchLoop is the main watch loop
func (asm *autoSyncManager) watchLoop() {
	// Default to 1 second if not specified
	interval := asm.config.WatchInterval
	if interval <= 0 {
		interval = 1 * time.Second
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Ensure watcher is initialized
	if asm.watcher == nil || asm.watcher.watcher == nil {
		asm.engine.eventBus.emit(EventError, fmt.Errorf("file watcher not initialized"))
		return
	}
	
	// Get references to channels while holding the lock
	// This prevents nil pointer access if Stop() is called concurrently
	eventsChan := asm.watcher.watcher.Events
	errorsChan := asm.watcher.watcher.Errors

	for {
		select {
		case <-asm.stopChan:
			return

		case event, ok := <-eventsChan:
			if !ok {
				return
			}

			// Filter out unwanted events
			if asm.shouldIgnoreEvent(event) {
				continue
			}

			// Emit file changed event
			asm.engine.eventBus.emit(EventFileChanged, FileChange{
				Path:      event.Name,
				Type:      asm.getChangeType(event),
				Timestamp: time.Now(),
			})

			// Debounce the sync
			asm.debouncedSync()

		case err, ok := <-errorsChan:
			if !ok {
				return
			}
			asm.engine.eventBus.emit(EventError, err)

		case <-ticker.C:
			// Periodic sync check (in case file events were missed)
			if asm.config.Enabled {
				asm.performSync()
			}
		}
	}
}

// shouldIgnoreEvent checks if a file event should be ignored
func (asm *autoSyncManager) shouldIgnoreEvent(event fsnotify.Event) bool {
	// Ignore chmod events unless it's also a write
	if event.Op == fsnotify.Chmod {
		return true
	}

	// Check ignore patterns
	for _, pattern := range asm.config.IgnorePatterns {
		matched, err := filepath.Match(pattern, filepath.Base(event.Name))
		if err == nil && matched {
			return true
		}
	}

	// Ignore temporary files
	if strings.HasSuffix(event.Name, "~") || strings.HasPrefix(filepath.Base(event.Name), ".") {
		return true
	}

	return false
}

// getChangeType converts fsnotify operation to change type
func (asm *autoSyncManager) getChangeType(event fsnotify.Event) string {
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		return "create"
	case event.Op&fsnotify.Write == fsnotify.Write:
		return "modify"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		return "delete"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		return "rename"
	default:
		return "unknown"
	}
}

// debouncedSync debounces sync operations
func (asm *autoSyncManager) debouncedSync() {
	asm.mu.Lock()
	defer asm.mu.Unlock()

	// Cancel existing timer
	if asm.debounceTimer != nil {
		asm.debounceTimer.Stop()
	}

	// Create new timer
	asm.debounceTimer = time.AfterFunc(asm.config.DebounceDelay, func() {
		asm.performSync()
	})
}

// performSync performs the actual synchronization
func (asm *autoSyncManager) performSync() {
	asm.mu.Lock()
	if !asm.isRunning || !asm.config.Enabled {
		asm.mu.Unlock()
		return
	}
	asm.mu.Unlock()

	// Reload config first to get latest changes
	if asm.engine.configPath != "" {
		if err := asm.engine.LoadConfig(asm.engine.configPath); err != nil {
			asm.engine.eventBus.emit(EventError, fmt.Errorf("failed to reload config: %w", err))
			return
		}
	}

	// Create sync options
	options := SyncOptions{
		DryRun:       false,
		CreateBackup: true,
		Verbose:      false,
	}

	// Get destinations to sync to
	destinations := asm.getDestinationsToSync()

	// Perform sync to each destination
	ctx := context.Background()
	for _, destName := range destinations {
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
