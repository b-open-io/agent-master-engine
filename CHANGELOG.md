# Changelog

All notable changes to the Agent Master Engine will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.2] - 2024-05-26

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

## [1.0.1] - 2024-05-26

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

## [1.0.0] - 2024-05-26

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