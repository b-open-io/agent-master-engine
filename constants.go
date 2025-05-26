package engine

// EventType represents different event types in the system
type EventType string

// Event type constants
const (
	// Configuration Events
	EventConfigLoaded  EventType = "config.loaded"
	EventConfigSaved   EventType = "config.saved"
	EventServerAdded   EventType = "server.added"
	EventServerUpdated EventType = "server.updated"
	EventServerRemoved EventType = "server.removed"

	// Sync Events
	EventSyncStarted      EventType = "sync.started"
	EventSyncCompleted    EventType = "sync.completed"
	EventSyncFailed       EventType = "sync.failed"
	EventConflictDetected EventType = "sync.conflict"

	// Auto-Sync Events
	EventAutoSyncStarted EventType = "autosync.started"
	EventAutoSyncStopped EventType = "autosync.stopped"
	EventFileChanged     EventType = "autosync.file.changed"

	// Project Events
	EventProjectDiscovered EventType = "project.discovered"
	EventProjectRegistered EventType = "project.registered"
	EventProjectRemoved    EventType = "project.removed"

	// Error Events
	EventError   EventType = "error"
	EventWarning EventType = "warning"
)

// LogLevel represents logging levels
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Default configuration values
const (
	DefaultWatchInterval  = 1000 // milliseconds
	DefaultDebounceDelay  = 500  // milliseconds
	DefaultMaxBackups     = 10
	DefaultMaxSyncWorkers = 5
	DefaultConfigVersion  = "1.0.0"
)

// File patterns
const (
	ClaudeConfigFile = ".claude.json"
	MCPConfigFile    = ".mcp.json"
	BackupExtension  = ".backup"
)

// Error messages
const (
	ErrServerNotFound    = "server not found"
	ErrTargetNotFound    = "target not found"
	ErrInvalidTransport  = "invalid transport type"
	ErrInvalidServerName = "invalid server name"
	ErrDuplicateServer   = "server already exists"
)
