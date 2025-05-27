package engine

import "time"

// Config represents the MCP configuration
// Renamed from MasterConfig for clarity and brevity
type Config struct {
	Version  string                        `json:"version"`
	Servers  map[string]ServerWithMetadata `json:"servers"`
	Settings Settings                      `json:"settings,omitempty"`
	Targets  map[string]TargetConfig       `json:"targets,omitempty"` // Legacy field
	Metadata map[string]interface{}        `json:"metadata,omitempty"`
}

// Settings contains global configuration settings
// Renamed from GlobalSettings for brevity
type Settings struct {
	AutoSync           AutoSyncSettings         `json:"autoSync,omitempty"`
	Backup             BackupSettings           `json:"backup,omitempty"`
	Sync               SyncSettings             `json:"sync,omitempty"`
	ConflictResolution ConflictSettings         `json:"conflictResolution,omitempty"`
	ProjectScanning    ProjectScanSettings      `json:"projectScanning,omitempty"`
	Validation         ValidationSettings       `json:"validation,omitempty"`
	DefaultTransport   string                   `json:"defaultTransport,omitempty"`
	Projects           map[string]ProjectConfig `json:"projects,omitempty"`
}

// ServerConfig represents a basic server configuration
type ServerConfig struct {
	Transport string                 `json:"transport"` // "stdio" or "sse"
	Command   string                 `json:"command,omitempty"`
	Args      []string               `json:"args,omitempty"`
	Env       map[string]string      `json:"env,omitempty"`
	URL       string                 `json:"url,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ServerWithMetadata includes internal metadata
type ServerWithMetadata struct {
	ServerConfig
	Internal InternalMetadata `json:"internal,omitempty"`
}

// InternalMetadata contains engine-specific metadata
type InternalMetadata struct {
	Enabled            bool      `json:"enabled,omitempty"`
	LastSynced         time.Time `json:"lastSynced,omitempty"`
	LastModified       time.Time `json:"lastModified,omitempty"`
	Source             string    `json:"source,omitempty"`
	CreatedBy          string    `json:"createdBy,omitempty"`
	Version            string    `json:"version,omitempty"`
	SyncTargets        []string  `json:"syncTargets,omitempty"`
	ExcludeFromTargets []string  `json:"excludeFromTargets,omitempty"`
	Tags               []string  `json:"tags,omitempty"`
	ProjectPath        string    `json:"projectPath,omitempty"`
	ProjectSpecific    bool      `json:"projectSpecific,omitempty"`
	ErrorCount         int       `json:"errorCount,omitempty"`
}

// AutoSyncSettings controls automatic synchronization
type AutoSyncSettings struct {
	Enabled       bool          `json:"enabled"`
	WatchInterval time.Duration `json:"watchInterval,omitempty"`
	DebounceDelay time.Duration `json:"debounceDelay,omitempty"`
	Destinations  []string      `json:"destinations,omitempty"`
}

// BackupSettings controls backup behavior
type BackupSettings struct {
	Enabled     bool   `json:"enabled"`
	MaxBackups  int    `json:"maxBackups,omitempty"`
	BackupPath  string `json:"backupPath,omitempty"`
	Location    string `json:"location,omitempty"` // Alternative to BackupPath
	BeforeSync  bool   `json:"beforeSync,omitempty"`
	Compression bool   `json:"compression,omitempty"`
}

// SyncSettings controls synchronization behavior
type SyncSettings struct {
	Strategy           string        `json:"strategy,omitempty"`           // "merge", "replace", "selective"
	ConflictResolution string        `json:"conflictResolution,omitempty"` // "master-wins", "target-wins", "manual"
	PreserveMissing    bool          `json:"preserveMissing,omitempty"`
	BatchSize          int           `json:"batchSize,omitempty"`
	Timeout            time.Duration `json:"timeout,omitempty"`
}

// ProjectConfig represents project-specific configuration
type ProjectConfig struct {
	Name         string                        `json:"name"`
	Path         string                        `json:"path"`
	Servers      map[string]ServerWithMetadata `json:"servers,omitempty"`
	Destinations []string                      `json:"destinations,omitempty"`
	AutoSync     bool                          `json:"autoSync,omitempty"`
	Metadata     map[string]interface{}        `json:"metadata,omitempty"`
}

// SyncOptions controls a synchronization operation
type SyncOptions struct {
	DryRun            bool              `json:"dryRun,omitempty"`
	Force             bool              `json:"force,omitempty"`
	CreateBackup      bool              `json:"createBackup,omitempty"`
	BackupFirst       bool              `json:"backupFirst,omitempty"` // Alias for CreateBackup
	IncludeDisabled   bool              `json:"includeDisabled,omitempty"`
	ServerFilter      []string          `json:"serverFilter,omitempty"`
	DestinationConfig map[string]string `json:"destinationConfig,omitempty"`
	Verbose           bool              `json:"verbose,omitempty"`
}

// SyncError represents an error during sync
type SyncError struct {
	Error       string `json:"error"`
	Recoverable bool   `json:"recoverable"`
}

// SyncResult represents the outcome of a sync operation
type SyncResult struct {
	Target         string        `json:"target,omitempty"` // Legacy field for compatibility
	Destination    string        `json:"destination"`
	Success        bool          `json:"success"`
	ServersAdded   int           `json:"serversAdded"`
	ServersUpdated int           `json:"serversUpdated"`
	ServersRemoved int           `json:"serversRemoved"`
	Changes        []Change      `json:"changes,omitempty"`
	Errors         []SyncError   `json:"errors,omitempty"`
	BackupPath     string        `json:"backupPath,omitempty"`
	ConfigPath     string        `json:"configPath,omitempty"`
	Duration       time.Duration `json:"duration"`
	Timestamp      time.Time     `json:"timestamp"`
}

// Change represents a configuration change
type Change struct {
	Type   string      `json:"type"` // "add", "update", "remove"
	Server string      `json:"server"`
	Before interface{} `json:"before,omitempty"`
	After  interface{} `json:"after,omitempty"`
}

// ImportOptions controls import behavior
type ImportOptions struct {
	Overwrite         bool     `json:"overwrite"`
	OverwriteExisting bool     `json:"overwriteExisting"` // Alias for Overwrite
	MergeStrategy     string   `json:"mergeStrategy"`     // "replace", "merge", "skip"
	MergeMode         string   `json:"mergeMode"`         // Alias for MergeStrategy
	ServerWhitelist   []string `json:"serverWhitelist,omitempty"`
	ServerBlacklist   []string `json:"serverBlacklist,omitempty"`
	ImportMetadata    bool     `json:"importMetadata"`
	SkipInvalid       bool     `json:"skipInvalid"`
	SubstituteEnvVars bool     `json:"substituteEnvVars"` // Whether to replace ${ENV_VAR} patterns
}

// ImportFormat represents supported import formats
type ImportFormat string

const (
	ImportFormatJSON ImportFormat = "json"
	ImportFormatYAML ImportFormat = "yaml"
	ImportFormatTOML ImportFormat = "toml"
)

// ImportResult contains the outcome of an import operation
type ImportResult struct {
	Source          string   `json:"source"`
	ServersImported int      `json:"serversImported"`
	ServersSkipped  int      `json:"serversSkipped"`
	ServersUpdated  int      `json:"serversUpdated"`
	Errors          []string `json:"errors,omitempty"`
}

// ConfigChange represents a configuration change event
type ConfigChange struct {
	Type      string                 `json:"type"` // "server-added", "server-removed", "server-updated", "settings-changed"
	Name      string                 `json:"name,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"` // "user", "sync", "import", "auto-sync"
	Details   map[string]interface{} `json:"details,omitempty"`
}

// FileChange represents a file system change
type FileChange struct {
	Path      string    `json:"path"`
	Type      string    `json:"type"` // "create", "modify", "delete"
	Timestamp time.Time `json:"timestamp"`
}

// SyncPreview shows what will happen in a sync
type SyncPreview struct {
	Destination    string        `json:"destination"`
	Changes        []Change      `json:"changes"`
	EstimatedTime  time.Duration `json:"estimatedTime"`
	RequiresBackup bool          `json:"requiresBackup"`
}

// MultiSyncResult aggregates multiple sync results
type MultiSyncResult struct {
	Results       []SyncResult  `json:"results"`
	TotalDuration time.Duration `json:"totalDuration"`
	SuccessCount  int           `json:"successCount"`
	FailureCount  int           `json:"failureCount"`
}

// ExportFormat represents supported export formats
type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatYAML ExportFormat = "yaml"
	ExportFormatTOML ExportFormat = "toml"
)

// ServerInfo contains detailed server information
type ServerInfo struct {
	Name            string                 `json:"name"`
	Config          ServerConfig           `json:"config"`
	Transport       string                 `json:"transport,omitempty"`
	Enabled         bool                   `json:"enabled"`
	Internal        InternalMetadata       `json:"internal,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	SyncTargetCount int                    `json:"syncTargetCount,omitempty"`
	LastModified    time.Time              `json:"lastModified,omitempty"`
	HasErrors       bool                   `json:"hasErrors,omitempty"`
}

// ProjectInfo contains project information
type ProjectInfo struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	ServerCount int      `json:"serverCount"`
	Servers     []string `json:"servers,omitempty"`
}

// AutoSyncStatus represents auto-sync state
type AutoSyncStatus struct {
	Enabled       bool          `json:"enabled"`
	Running       bool          `json:"running"`
	LastSync      time.Time     `json:"lastSync,omitempty"`
	NextSync      time.Time     `json:"nextSync,omitempty"`
	WatchInterval time.Duration `json:"watchInterval"`
}

// BackupInfo contains backup details
type BackupInfo struct {
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
	Type      string    `json:"type"` // "manual", "auto", "pre-sync"
}

// TargetConfig represents legacy target configuration
type TargetConfig struct {
	Name                 string `json:"name"`
	Type                 string `json:"type"`
	Enabled              bool   `json:"enabled"`
	ConfigPath           string `json:"configPath"`
	RequiresSanitization bool   `json:"requiresSanitization"`
	SupportsProjects     bool   `json:"supportsProjects"`
	ConfigFormat         string `json:"configFormat"`
	ServerNamePattern    string `json:"serverNamePattern,omitempty"`
}

// TargetInfo contains target information
type TargetInfo struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Enabled     bool      `json:"enabled"`
	ConfigPath  string    `json:"configPath"`
	LastSync    time.Time `json:"lastSync,omitempty"`
	ServerCount int       `json:"serverCount"`
}

// ConfigTransformer transforms configuration for specific formats
type ConfigTransformer interface {
	Transform(config *Config) (interface{}, error)
	Format() string
}

// ConflictSettings controls conflict resolution behavior
type ConflictSettings struct {
	Mode string `json:"mode"` // "interactive", "master-wins", "target-wins"
}

// ProjectScanSettings controls project scanning behavior
type ProjectScanSettings struct {
	Enabled      bool     `json:"enabled"`
	ScanPaths    []string `json:"scanPaths,omitempty"`
	ExcludePaths []string `json:"excludePaths,omitempty"`
	MaxDepth     int      `json:"maxDepth,omitempty"`
}

// ValidationSettings controls validation behavior
type ValidationSettings struct {
	Enabled             bool `json:"enabled"`
	ValidateBeforeWrite bool `json:"validateBeforeWrite"`
	ValidateAfterWrite  bool `json:"validateAfterWrite"`
	StrictMode          bool `json:"strictMode"`
}
