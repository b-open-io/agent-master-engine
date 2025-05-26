# CLAUDE.md - Agent Master Engine

This file provides guidance to Claude Code when working with the Agent Master Engine codebase.

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
- Storage abstraction with file and memory implementations
- Event system with typed events
- Basic sync to destinations
- Generic destination system with transformers
- Validation and sanitization interfaces
- Type definitions for all major structures

### 🚧 Partially Implemented
- Sync operations (basic structure complete, needs advanced features)
- Change detection (simple comparison implemented)
- Destination management (basic registration/listing)

### ❌ Not Yet Implemented
- Project management functions
- Auto-sync functionality
- Import/Export operations
- Backup/Restore system
- Advanced sync features (merge strategies, conflict resolution)
- Health checks and diagnostics

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