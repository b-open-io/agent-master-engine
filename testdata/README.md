# Agent Master Engine Test Data

This directory contains comprehensive test vectors and scenarios for the Agent Master Engine library. These test files are based on real MCP (Model Context Protocol) server configurations and cover various edge cases, transport types, and complex synchronization scenarios.

## Test Files Overview

### 1. `stdio_servers.json`
- Real examples of stdio transport MCP servers
- Includes servers like memory, puppeteer, filesystem, git, github, etc.
- Tests command execution with various argument patterns
- Includes environment variable usage examples

### 2. `sse_servers.json`
- Real examples of SSE (Server-Sent Events) transport servers
- Includes Zapier MCP, hosted BSV MCP, Bubble, n8n
- Tests URL-based connections with headers
- Shows both authenticated and public SSE endpoints

### 3. `edge_cases.json`
- Problematic server names requiring sanitization
- Tests special characters: @, /, spaces, !, #, $, etc.
- Includes expected sanitized names for validation
- Covers all character replacement scenarios

### 4. `internal_fields.json`
- Agent-master specific metadata not written to targets
- Shows enabled/disabled states, sync targets, exclusions
- Demonstrates project-specific server configurations
- Includes error tracking and source information

### 5. `project_configs.json`
- Claude Code style project-level configurations
- Multiple projects with their own MCP server sets
- Tests inheritance and project-specific overrides
- Real project paths from the user's system

### 6. `master_config.json`
- Complete master configuration example
- Mix of stdio and SSE servers
- Environment variable references
- Represents a typical ~/.agent-master/mcp.json

### 7. `edge_case_scenarios.json`
- Complex synchronization scenarios
- Documents 10 different edge cases:
  - Project folder as sync target
  - Nested project configurations
  - Server name conflicts after sanitization
  - Circular sync dependencies
  - Environment variable conflicts
  - Disabled server in active project
  - Target-specific server configurations
  - Auto-sync file collision
  - Server removal propagation
  - Mixed transport project migration

### 8. `sync_settings.json`
- All configurable settings affecting sync behavior
- Global settings (auto-sync, backup, conflict resolution)
- Target-specific settings and capabilities
- Server defaults and sync filters
- Advanced options (parallel sync, validation, logging)

### 9. `complex_scenario.json`
- Real-world scenario with multiple interacting features
- Global servers with various states
- Project-specific configurations
- Expected sync results for each target
- Demonstrates sanitization, conflicts, and overrides

### 10. `autosync_test_cases.json`
- File watching and auto-sync test scenarios
- 10 different trigger scenarios:
  - Adding/removing servers
  - Project config modifications
  - External config changes
  - Rapid consecutive changes
  - File deletion recovery
  - Circular dependency prevention
  - Server rename cascades
  - Disabled server handling
  - Target-specific exclusions
  - Concurrent updates

### 11. `validation_rules.json`
- MCP protocol validation requirements
- Transport-specific field requirements
- Server name validation patterns
- Internal field specifications
- Common validation error messages

## Usage in Tests

The `engine_test.go` file demonstrates how to use these test vectors:

```go
// Load test data
var data struct {
    Servers map[string]MCPServer `json:"servers"`
}
LoadTestData(t, "stdio_servers.json", &data)

// Use in tests
for name, server := range data.Servers {
    // Test server validation, sanitization, sync, etc.
}
```

## Key Test Scenarios

### 1. Name Sanitization
- Test that problematic names are correctly sanitized for Claude Code
- Verify duplicate handling (appending numbers)
- Ensure valid names pass through unchanged

### 2. Transport Validation
- Verify stdio servers have required `command` field
- Verify SSE servers have required `url` field
- Ensure forbidden field combinations are rejected

### 3. Internal Field Handling
- Confirm internal fields are preserved in master config
- Verify internal fields are stripped when writing to targets
- Test that sync decisions respect internal field values

### 4. Project-Level Configs
- Test merging global and project-specific servers
- Verify project overrides work correctly
- Ensure project-only servers don't sync globally

### 5. Auto-Sync Behavior
- Test file watching triggers correct sync actions
- Verify debouncing prevents excessive syncs
- Ensure circular dependencies are avoided

## Adding New Test Cases

When adding new test cases:

1. Use real examples when possible
2. Include expected outcomes for validation
3. Document edge cases and their resolutions
4. Consider interactions with existing features
5. Test both success and failure scenarios

## MCP Protocol Compliance

These test vectors aim to be compliant with the official MCP specification while also supporting Agent Master's additional features for cross-tool synchronization.