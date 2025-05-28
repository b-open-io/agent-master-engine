# CLAUDE.md - Agent Master Engine

This file provides guidance to Claude Code when working with the Agent Master Engine codebase.

## Current State (2025-05-27)

### 🎉 MAJOR UPDATE: Project Scanning Implemented (v0.1.7)

**The Issue**: Project scanning functionality was returning "not implemented" errors, blocking the daemon migration strategy.

**The Solution**: Fully implemented project management functionality with protocol-compliant MCP parsing.

**What's Now Available**:
- ✅ `ScanForProjects()` - Recursively scan directories for MCP projects
- ✅ `RegisterProject()` - Store project configurations  
- ✅ `GetProjectConfig()` - Retrieve projects by path
- ✅ `ListProjects()` - List all registered projects
- ✅ `DefaultProjectDetector` - Detect common project types
- ✅ Full MCP format support (Claude Desktop, GitHub MCP, flat formats)
- ✅ Working example in `examples/project_scanning/`

**Impact**: The user's Wails app can now successfully scan for projects, eliminating the "not implemented" error and enabling the Phase 2 daemon migration.

### 🔥 CRITICAL CONTEXT: Daemon Architecture Clarification

After 2+ hours of investigation and implementation, we discovered a fundamental misunderstanding:

**The Vision**: 
- ONE shared daemon that both Wails app and CLI connect to
- Auto-sync continues running when either app is closed
- Daemon should be its own binary living in the engine repo
- Uses gRPC for communication (with TODO for HTTP/socket adapters)

**What Was Built Wrong**:
- Spent time building daemon infrastructure in the CLI repo
- Assumed daemon was CLI-specific
- Didn't realize the intent was for a shared service

**Current Issues**:
1. Multiple daemons spawning due to lock file race conditions
2. Auto-sync state not persisting properly
3. Every CLI command tries to start a daemon (even `daemon-status`!)
4. Misleading "Starting daemon..." messages

**Next Steps**:
1. Move daemon implementation to engine repo as separate binary
2. Both CLI and Wails app become clients to this shared daemon
3. Implement proper gRPC service with clean interfaces
4. Use git worktrees for parallel development

## Project Overview

Agent Master Engine is a **generic** Go library for managing Model Context Protocol (MCP) server configurations. It provides core functionality for server management, validation, and synchronization without any hardcoded knowledge of specific platforms or tools.

## Design Philosophy

The engine is intentionally platform-agnostic:
- **No hardcoded references** to Claude Code, VS Code, Cursor, or any specific tools
- **Pluggable validation** and sanitization through interfaces
- **Generic destinations** for sync operations
- **Optional presets** for common platforms (in separate package)

## Architecture

### Core Components

1. **Engine Interface** (`engine.go`) - Defines the generic MCP management interface
2. **Implementation** (`engine_impl.go`) - Core engine implementation
3. **Type Definitions** (`types.go`) - All data structures and types
4. **Storage Layer** (`storage.go`) - Abstracted storage with file and memory implementations
5. **Validation** (`validation.go`) - Pluggable validation without platform specifics
6. **Sync Manager** (`sync.go`) - Handles synchronization to generic destinations
7. **Generic Destinations** (`generic_destinations.go`) - File-based destination implementation
8. **Utilities** (`utils.go`) - Common helper functions

### Optional Components

1. **Presets Package** (`presets/`) - Platform-specific configurations (Claude Code, VS Code, etc.)
2. **Claude Integration** (`claude_integration.go`) - Optional Claude Code SDK integration

## Current Implementation Status

### ✅ Fully Implemented
- Configuration management (load, save, get, set)
- Server CRUD operations with validation
- Storage abstraction with file, memory, and Redis implementations
- Event system with typed events
- Single destination sync (SyncTo)
- Multi-destination sync (SyncToMultiple) - sync to multiple targets concurrently
- Sync preview (PreviewSync) - see changes before applying
- Auto-sync functionality with file watching and debouncing
- Import functionality with MCP format parsing
- Generic destination system with transformers
- Validation and sanitization interfaces
- Type definitions for all major structures
- Variable substitution (optional for environment variables)
- Support for multiple MCP configuration formats
- Project management functions (ScanForProjects, RegisterProject, GetProjectConfig, ListProjects)

### 🚧 Partially Implemented
- Change detection (basic comparison implemented)
- Destination management (basic registration/listing)

### ❌ Not Yet Implemented
- Export operations (Export, ExportToFile)
- Backup/Restore system
- Config merging (MergeConfigs)
- Import from target (ImportFromTarget)
- Advanced sync features (merge strategies, conflict resolution)
- Health checks and diagnostics
- Input variable collection (${input:xxx} patterns)

## Code Style Guidelines

- Use meaningful variable names
- Keep functions focused and small
- Handle errors explicitly
- Follow Go idioms and best practices
- No hardcoded platform references in core

## Testing

```bash
# Run unit tests
go test ./...

# Run integration test
go run ./cmd/test

# Test with real configs (be careful!)
go run ./cmd/test --real-config
```

## Common Tasks

### Adding a new destination type
1. Implement the `Destination` interface
2. Add transformer if needed
3. Test with engine
4. Optionally add to presets package

### Creating custom validation
1. Implement `ServerValidator` interface
2. Set on engine with `SetValidator()`
3. Test validation rules

### Adding new sync format
1. Create a `ConfigTransformer`
2. Use with `FileDestination`
3. Test transformation

## Important Patterns

### Creating Custom Destinations
```go
type MyDestination struct {
    endpoint string
}

func (m *MyDestination) Transform(config *Config) (interface{}, error) {
    // Custom transformation logic
    return myFormat, nil
}

func (m *MyDestination) Write(data []byte) error {
    // Custom write logic (API call, etc.)
    return nil
}
```

### Custom Validation
```go
type MyValidator struct {
    rules []Rule
}

func (v *MyValidator) ValidateName(name string) error {
    // Custom validation logic
    return nil
}

engine.SetValidator(&MyValidator{})
```

## Migration Notes

The engine has been refactored to be completely generic:
- Removed all references to specific platforms (Claude Code, VS Code, etc.)
- Replaced "Target" system with generic "Destination" interface
- Made validation and sanitization pluggable
- Moved platform-specific code to optional presets package

## Best Practices

1. **Use generic destinations** for new integrations
2. **Create presets** for common configurations
3. **Keep validation rules** flexible and configurable
4. **Test with multiple destination types**
5. **Document custom destinations** clearly

## Common Pitfalls

1. **Deadlocks**: Be careful with mutex usage, use `saveConfigNoLock()` when already holding lock
2. **Path Expansion**: Remember to expand ~ in paths using `expandPath()`
3. **Async Events**: Event handlers run in goroutines, don't assume synchronous execution
4. **Validation**: Always validate before operations, but make it pluggable

## Debugging Tips

1. Use memory storage for testing to avoid file system issues
2. Create mock destinations for unit tests
3. Test transformations independently
4. Validate against real MCP servers when possible
5. Check `~/.agent-master/` for actual config files

## Future Considerations

- Consider adding middleware pattern for sync operations
- Explore plugin system for destinations
- Add metrics and observability
- Consider gRPC interface for remote operations

## Questions to Consider

When making changes, consider:
1. Will this work with all possible destinations?
2. Does this maintain backward compatibility?
3. Is the implementation generic enough?
4. Are errors handled gracefully?
5. Does this introduce platform-specific code to core?

## MCP Configuration Format Support

### Discovered MCP Configuration Formats

Through investigation of the MCP ecosystem, we've identified several configuration formats in use:

#### 1. **Claude Desktop Format** (`mcpServers` wrapper)
```json
{
  "mcpServers": {
    "server-name": {
      "transport": "stdio",
      "command": "bunx",
      "args": ["@modelcontextprotocol/server-filesystem", "/path"]
    }
  }
}
```
Used by: Claude Desktop, Cursor

#### 2. **GitHub MCP Format** (nested `mcp` with `inputs`)
```json
{
  "mcp": {
    "inputs": [
      {
        "type": "promptString",
        "id": "github_token",
        "description": "GitHub Personal Access Token",
        "password": true
      }
    ],
    "servers": {
      "github": {
        "command": "docker",
        "args": ["run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghcr.io/github/github-mcp-server"],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
        }
      }
    }
  }
}
```
Used by: GitHub's official MCP server

#### 3. **Flat Format** (direct `servers`)
```json
{
  "servers": {
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"],
      "transport": "stdio"
    }
  }
}
```
Used by: Some VS Code extensions, simple configurations

#### 4. **Docker-based Configurations**
Many production MCP servers use Docker for isolation:
- `docker run` commands with environment variable mapping
- `docker-compose` integration for complex setups
- Container registries (ghcr.io, Docker Hub)

### Variable Substitution Patterns

MCP configurations support several variable substitution patterns:

1. **Environment Variables**: `${ENV_VAR_NAME}`
2. **Input Variables**: `${input:variable_id}` (requires `inputs` definition)
3. **Nested Variables**: `${input:api_key}-${USER}`

### Transport Types

1. **stdio** - Standard input/output communication
   - Most common for local servers
   - Uses `command` and `args`

2. **sse** - Server-Sent Events over HTTP
   - For cloud-hosted servers
   - Uses `url` and `headers`

### MCP Protocol Versions

The MCP specification has evolved through several versions:
- `2024-11-05` - Initial public release
- `2025-03-26` - Current version with OAuth 2.1, tool annotations, audio support
- `draft` - Experimental features (not yet supported)

See `docs/MCP_PROTOCOL_VERSIONS.md` for detailed version differences and compatibility information.

### Implementation Status

Currently, the engine supports:
- ✅ Basic stdio and SSE transports
- ✅ Environment variable substitution
- ✅ Multiple storage backends
- ❌ `mcp.inputs` configuration format
- ❌ `${input:xxx}` variable substitution
- ❌ Docker-compose integration helpers

### TODO for Full MCP Compatibility

1. **Add MCPConfig type** that can parse all format variations
2. **Implement input prompt system** for collecting user inputs
3. **Add variable substitution engine** for both env and input variables
4. **Create format converters** to normalize different formats
5. **Add Docker integration helpers** for container-based servers
6. **Support MCP version detection** and compatibility checks

## Recent Fixes (2025-05-27)

### Fixed: Nil Map Panic on Config Load
- **Issue**: When loading configuration from file, the Servers map was not initialized, causing panic on server operations
- **Fix**: Added initialization check after loading config to ensure Servers map exists
- **File**: `config_manager.go:33`

```go
// Ensure servers map is initialized
if e.config.Servers == nil {
    e.config.Servers = make(map[string]ServerConfig)
}
```

### Implemented: Project Management Functionality
- **Issue**: ScanForProjects and related methods returned "not implemented" errors
- **Fix**: Implemented full project scanning functionality in `project_manager.go`
- **Features Added**:
  - `ScanForProjects()` - Recursively scans directories for MCP projects
  - `RegisterProject()` - Stores project configurations
  - `GetProjectConfig()` - Retrieves project by path
  - `ListProjects()` - Lists all registered projects
  - `DefaultProjectDetector` - Detects common project types (Node.js, Go, Python, etc.)
  - MCP configuration parsing (Claude Desktop, GitHub MCP, and flat formats)
- **Files**: `project_manager.go`, `project_manager_test.go`
- **Version**: Released in v0.1.7 (2025-05-27)

### Fixed: Protocol Compliance
- **Issue**: Custom MCP parsing could have protocol violations
- **Fix**: Replaced custom parsing with existing `ParseMCPConfig()` function
- **Result**: Ensures full compliance with MCP specification and proper handling of all documented formats

### 🚨 CRITICAL FIX: LoadConfig Server Overwriting Bug
- **Issue**: CLI was overwriting entire server configuration instead of accumulating servers
- **Root Cause**: `LoadConfig()` method saved to storage backend after loading from file, creating file/storage conflicts
- **Symptoms**: 
  - Every CLI `add` command lost previous servers
  - Multiple servers could not be maintained via CLI
  - Data loss and user frustration
- **Fix**: 
  - Removed storage save from `LoadConfig()` when loading from file
  - Updated `saveConfigNoLock()` to save to file when `configPath` is set
  - File is now the single source of truth when specified
- **Files**: `config_manager.go`, `config_manager_test.go`
- **Testing**: Added `TestLoadConfigFileConsistency` to prevent regression
- **Version**: Fixed in v0.1.9 (2025-05-27)
