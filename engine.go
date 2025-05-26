package engine

import (
	"context"
	"time"
)

// Engine is a generic MCP server configuration manager
type Engine interface {
	// Configuration Management
	LoadConfig(path string) error
	SaveConfig() error
	GetConfig() (*Config, error)
	SetConfig(config *Config) error

	// Server Management
	AddServer(name string, server ServerConfig) error
	UpdateServer(name string, server ServerConfig) error
	RemoveServer(name string) error
	GetServer(name string) (*ServerWithMetadata, error)
	ListServers(filter ServerFilter) ([]*ServerInfo, error)

	// Destination Management
	RegisterDestination(name string, dest Destination) error
	RemoveDestination(name string) error
	GetDestination(name string) (Destination, error)
	ListDestinations() map[string]Destination

	// Generic Sync Operations
	SyncTo(ctx context.Context, dest Destination, options SyncOptions) (*SyncResult, error)
	SyncToMultiple(ctx context.Context, dests []Destination, options SyncOptions) (*MultiSyncResult, error)
	PreviewSync(dest Destination) (*SyncPreview, error)

	// Import/Export (format agnostic)
	Export(format ExportFormat) ([]byte, error)
	Import(data []byte, format ImportFormat, options ImportOptions) error
	MergeConfigs(configs ...*Config) (*Config, error)

	// Validation (pluggable)
	SetValidator(validator ServerValidator)
	SetSanitizer(sanitizer NameSanitizer)
	ValidateServer(name string, server ServerConfig) error
	SanitizeName(name string) string

	// Project Management
	ScanForProjects(paths []string, detector ProjectDetector) ([]*ProjectConfig, error)
	RegisterProject(path string, config ProjectConfig) error
	GetProjectConfig(path string) (*ProjectConfig, error)
	ListProjects() ([]*ProjectInfo, error)

	// Auto-sync Management
	StartAutoSync(config AutoSyncConfig) error
	StopAutoSync() error
	GetAutoSyncStatus() (*AutoSyncStatus, error)

	// Backup/Restore
	CreateBackup(description string) (*BackupInfo, error)
	ListBackups() ([]*BackupInfo, error)
	RestoreBackup(backupID string) error

	// Event Handling
	OnConfigChange(handler ConfigChangeHandler) func()
	OnSyncComplete(handler SyncCompleteHandler) func()
	OnError(handler ErrorHandler) func()
}

// Storage interface for persistence layer abstraction
type Storage interface {
	Read(key string) ([]byte, error)
	Write(key string, data []byte) error
	Delete(key string) error
	List(prefix string) ([]string, error)
	Watch(key string, handler func([]byte)) (func(), error)
}

// Moved to types.go

// Moved to types.go

// Moved to types.go

// Moved to types.go for better organization

// Moved to types.go

// Destination represents any sync target (file, API, etc.)
type Destination interface {
	// Identity
	GetID() string
	GetDescription() string

	// Configuration transformation
	Transform(config *Config) (interface{}, error)

	// IO operations
	Read() ([]byte, error)
	Write(data []byte) error
	Exists() bool

	// Optional features
	SupportsBackup() bool
	Backup() (string, error)
}

// Moved to types.go

// Types moved to types.go

// Moved to types.go

// Moved to types.go

// AutoSyncConfig configures automatic synchronization
type AutoSyncConfig struct {
	Enabled         bool          `json:"enabled"`
	WatchInterval   time.Duration `json:"watchInterval"`
	DebounceDelay   time.Duration `json:"debounceDelay"`
	TargetWhitelist []string      `json:"targetWhitelist,omitempty"`
	TargetBlacklist []string      `json:"targetBlacklist,omitempty"`
	IgnorePatterns  []string      `json:"ignorePatterns"`
}

// Moved to types.go

// ServerFilter for listing servers
type ServerFilter struct {
	Enabled         *bool    `json:"enabled,omitempty"`
	Transport       string   `json:"transport,omitempty"`
	SyncTargets     []string `json:"syncTargets,omitempty"`
	ProjectSpecific *bool    `json:"projectSpecific,omitempty"`
	Source          string   `json:"source,omitempty"`
	NamePattern     string   `json:"namePattern,omitempty"`
}

// Moved to types.go

// Types moved to types.go

// ServerValidator can validate server configurations
type ServerValidator interface {
	ValidateName(name string) error
	ValidateConfig(config ServerConfig) error
}

// NameSanitizer can sanitize server names
type NameSanitizer interface {
	Sanitize(name string) string
	NeedsSanitization(name string) bool
}

// ProjectDetector can detect project configurations
type ProjectDetector interface {
	DetectProject(path string) (*ProjectConfig, error)
	IsProjectRoot(path string) bool
}

// Event handler types
type ConfigChangeHandler func(change ConfigChange)
type SyncCompleteHandler func(result SyncResult)
type ErrorHandler func(err error)

// Types moved to types.go
