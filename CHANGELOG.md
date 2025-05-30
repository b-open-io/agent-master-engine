# Changelog

All notable changes to the Agent Master Engine will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.10] - 2025-05-27

### Added
- **Daemon Preset Support**
  - Daemon now automatically registers preset destinations on startup
  - Registers all presets from presets package (claude, vscode-mcp, cursor, generic-json)
  - Adds vscode alias that maps to vscode-mcp for backward compatibility
  - Registers windsurf and zed as custom destinations with proper transformers
  - Presets are now available for sync operations without manual registration
- **Version Reporting**
  - Daemon now reports actual version information
  - Added Version, GitCommit, and BuildDate variables
  - Created Makefile with proper ldflags for version injection
  - Daemon status now shows version like "0.1.9 (14ea75d)"

### Fixed
- Fixed nil result handling in daemon's syncResultToProto function
- Fixed ActiveConnections type mismatch in daemon status (int64 -> int32)

## [0.1.9] - 2025-05-27

### Fixed
- **CRITICAL**: LoadConfig server overwriting bug
  - CLI commands no longer overwrite entire configuration when adding servers
  - File-based configs now properly accumulate servers instead of losing previous ones
  - Fixed file vs storage backend conflict in `LoadConfig()` and `saveConfigNoLock()`
  - Added regression test `TestLoadConfigFileConsistency`

## [0.1.8] - 2025-05-27

### Added
- **Daemon gRPC Service Support** 
  - Implemented missing gRPC methods: `LoadConfig`, `SaveConfig`, `GetConfig`, `SetConfig`
  - Added proper type conversions between engine and protobuf formats
  - Daemon now supports TCP connections via `-port` flag (e.g., `-port 50051`)
  - Enhanced daemon service for multi-client architecture (CLI + GUI apps)

### Fixed
- Configuration management via daemon now fully functional
- Type mismatches between `ServerWithMetadata` and protobuf `ServerConfig` resolved
- Daemon service methods now properly handle engine configuration format

### Changed
- Daemon can now run on TCP port in addition to Unix sockets for better cross-application compatibility

## [0.1.7] - 2025-05-27

### Added
- Project scanning functionality
  - `ScanForProjects()` method implementation
  - `RegisterProject()`, `GetProjectConfig()`, `ListProjects()` methods
  - `DefaultProjectDetector` for common project types
  - Support for detecting MCP configurations in projects
  - Example and documentation in `examples/project_scanning/`

### Fixed
- Project management methods no longer return "not implemented" errors

## [0.1.6] - 2025-05-27

### Fixed
- Auto-sync events (`EventAutoSyncStarted`, `EventAutoSyncStopped`, `EventFileChanged`) are now properly routed through `OnConfigChange` handler
  - Fixed event bus to handle both `func(ConfigChange)` and `ConfigChangeHandler` type handlers
  - Changed auto-sync events to emit `ConfigChange` type instead of `ConfigChangeEvent` for consistency
  - `OnConfigChange` now subscribes to all config-related events including auto-sync events
  - **Note**: This is a backward-compatible fix - existing event handlers continue to work

### Changed
- `OnConfigChange` now subscribes to multiple event types (config, auto-sync, file changes) instead of just `EventConfigLoaded`

## [0.1.4] - 2025-05-27

### Fixed
- `LoadConfig()` now properly loads configuration from the specified file path when provided
  - Previously, `LoadConfig(path)` only set the internal config path but always loaded from storage
  - Now it reads from the actual file first, then falls back to storage if the file doesn't exist
  - This fixes auto-sync functionality not detecting file changes properly
  - **Note**: This is a backward-compatible fix - existing behavior is preserved when no file exists

### Added
- Test coverage for agent-master auto-sync scenario (`TestAutoSyncAgentMasterScenario`)
- Test coverage for file watching with actual file system changes (`TestAutoSyncFileWatchingMCPConfig`)

## [0.1.3] - 2025-01-27

### Changed
- Major refactoring: Broke down engine_impl.go from 1,370 lines to 571 lines (58% reduction)
- Extracted logical modules into separate files:
  - `server_manager.go` (199 lines) - Server CRUD operations
  - `config_manager.go` (93 lines) - Configuration management
  - `backup_manager.go` (169 lines) - Backup and restore functionality
  - `import_export.go` (225 lines) - Import/export operations
  - `destination_manager.go` (161 lines) - Destination and target management
  - `autosync_manager.go` (359 lines) - Auto-sync functionality
  - `event_bus.go` (108 lines) - Event handling system
  - `helpers.go` (41 lines) - Common utility functions
- Improved code organization and maintainability
- Better separation of concerns with single-responsibility modules

### Fixed
- Auto-sync now properly triggers on programmatic config changes via SetConfig()

## [0.1.2] - 2024-05-26

### Added
- MCP configuration format parsing support (`mcp_types.go`)
  - Support for VS Code/GitHub format with `mcp.inputs` and `mcp.servers`
  - Support for Claude Desktop format (`mcpServers` wrapper)
  - Support for flat format (direct `servers`)
  - Preservation of `mcp.inputs` in metadata for future implementation
- Import functionality implementation in engine
  - `Import()` method now functional
  - Support for merging and replacing configurations
  - Validation during import with skip invalid option
- Variable substitution support (optional)
  - Environment variable substitution (`${ENV_VAR}`) - opt-in via `SubstituteEnvVars` option
  - Input variable preservation (`${input:variable}` - always preserved for runtime substitution)
  - Default behavior preserves all variable patterns as-is
- MCP protocol version compatibility testing
  - Test vectors for MCP 2024-11-05 specification
  - Test vectors for MCP 2025-03-26 specification
  - Backward compatibility verification
- `Headers` field to `ServerConfig` for SSE transport authentication
- `WithMemoryStorage()` option for easier testing
- Comprehensive test suite for import functionality
- Documentation for MCP configuration formats (`docs/MCP_CONFIGURATION_FORMATS.md`)
- Documentation for MCP protocol versions (`docs/MCP_PROTOCOL_VERSIONS.md`)

### Changed
- `ServerValidator` interface now includes `ValidateServerConfig(name string, config ServerConfig) error`
- `ImportOptions` expanded with `OverwriteExisting`, `MergeMode`, and `SkipInvalid` fields

### Fixed
- Pattern validator now implements all required interface methods

### Protocol Compatibility
- **MCP 2024-11-05**: Full support for all features including SSE transport
- **MCP 2025-03-26**: Support for core features, metadata/annotations preserved but not validated
  - New features supported: Tool annotations, audio content type metadata, JSON-RPC batching metadata
  - OAuth 2.1 authorization metadata preserved for future implementation
  - Streamable HTTP transport treated as stdio for compatibility

## [0.1.1] - 2024-05-26

### Added
- Redis storage adapter implementation
- Storage adapter examples and documentation
- Internal documentation reorganization

### Changed
- Improved README with better formatting and examples
- Separated internal development docs from public documentation

### Fixed
- Git repository initialization issues
- File staging problems with .gitignore patterns

## [0.1.0] - 2024-05-26

### Added
- Initial release of Agent Master Engine
- Core engine interface and implementation
- File-based and memory-based storage
- Basic server CRUD operations
- Event system for configuration changes
- Target management for multiple AI tools
- Validation and sanitization framework
- Claude Code SDK integration
- Comprehensive test suite