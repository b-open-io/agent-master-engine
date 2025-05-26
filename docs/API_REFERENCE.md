# API Reference

## Package agent

The agent package provides a generic engine for managing Model Context Protocol (MCP) server configurations.

### Types

#### Engine

The main interface for managing MCP configurations.

```go
type Engine interface {
    // Configuration Management
    LoadConfig(path string) error
    SaveConfig() error
    GetConfig() *Config
    SetConfigPath(path string)
    
    // Server Management
    AddServer(name string, config ServerConfig) error
    UpdateServer(name string, config ServerConfig) error
    RemoveServer(name string) error
    GetServer(name string) (*ServerWithMetadata, error)
    ListServers() ([]*ServerInfo, error)
    
    // Synchronization
    SyncToDestination(destinationID string, options *SyncOptions) (*SyncResult, error)
    SyncToAllDestinations(options *SyncOptions) ([]*SyncResult, error)
    PreviewSync(destinationID string) (*SyncPreview, error)
    
    // Destination Management
    RegisterDestination(dest Destination) error
    UnregisterDestination(id string) error
    ListDestinations() []string
    
    // Validation
    SetValidator(validator ServerValidator)
    SetSanitizer(sanitizer NameSanitizer)
    
    // Events
    OnConfigChange(handler ConfigChangeHandler) func()
    OnSyncComplete(handler SyncCompleteHandler) func()
    OnError(handler ErrorHandler) func()
}
```

#### ServerConfig

Basic configuration for an MCP server.

```go
type ServerConfig struct {
    Transport string            `json:"transport"`     // "stdio" or "sse"
    Command   string            `json:"command,omitempty"`
    Args      []string          `json:"args,omitempty"`
    Env       map[string]string `json:"env,omitempty"`
    URL       string            `json:"url,omitempty"`
    Headers   map[string]string `json:"headers,omitempty"`
}
```

#### Destination

Interface for sync destinations.

```go
type Destination interface {
    GetID() string
    GetPath() string
    Transform(config *Config) (interface{}, error)
    Read() ([]byte, error)
    Write(data []byte) error
    Validate(config *Config) error
}
```

#### SyncOptions

Options for synchronization operations.

```go
type SyncOptions struct {
    DryRun           bool
    Force            bool
    CreateBackup     bool
    IncludeDisabled  bool
    ServerFilter     []string
    Verbose          bool
}
```

#### SyncResult

Result of a synchronization operation.

```go
type SyncResult struct {
    Destination    string
    Success        bool
    ServersAdded   int
    ServersUpdated int
    ServersRemoved int
    Changes        []Change
    Errors         []SyncError
    BackupPath     string
    Duration       time.Duration
    Timestamp      time.Time
}
```

### Functions

#### NewEngine

Creates a new engine instance.

```go
func NewEngine(options *EngineOptions) (Engine, error)
```

#### NewFileDestination

Creates a file-based destination.

```go
func NewFileDestination(id, path string, transformer ConfigTransformer) *FileDestination
```

### Interfaces

#### ServerValidator

```go
type ServerValidator interface {
    ValidateName(name string) error
    ValidateConfig(config ServerConfig) error
}
```

#### NameSanitizer

```go
type NameSanitizer interface {
    SanitizeName(name string) string
}
```

#### ConfigTransformer

```go
type ConfigTransformer interface {
    Transform(config *Config) (interface{}, error)
    Format() string
}
```

### Built-in Implementations

#### Validators

- `DefaultValidator` - Basic MCP validation
- `PatternValidator` - Regex-based validation

#### Sanitizers

- `ReplacementSanitizer` - Character replacement

#### Transformers

- `FlatConfigTransformer` - Flat JSON format
- `NestedConfigTransformer` - Nested by transport
- `ProjectNestedTransformer` - Nested by project
- `DirectTransformer` - No transformation

### Error Types

```go
var (
    ErrServerNotFound   = errors.New("server not found")
    ErrServerExists     = errors.New("server already exists")
    ErrInvalidConfig    = errors.New("invalid configuration")
    ErrDestinationNotFound = errors.New("destination not found")
)
```

### Events

The engine emits the following events:

- `ConfigLoaded` - Configuration loaded
- `ConfigSaved` - Configuration saved
- `ServerAdded` - Server added
- `ServerUpdated` - Server updated
- `ServerRemoved` - Server removed
- `SyncStarted` - Sync operation started
- `SyncCompleted` - Sync operation completed
- `Error` - Error occurred

### Storage Backends

#### FileStorage

File-based storage with atomic writes.

```go
storage := NewFileStorage("~/.agent-master")
```

#### MemoryStorage

In-memory storage for testing.

```go
storage := NewMemoryStorage()
```

## Examples

See the [Usage Guide](USAGE_GUIDE.md) for practical examples and integration patterns.