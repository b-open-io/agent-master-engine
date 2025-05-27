# MCP Protocol Version Support

This document describes the Model Context Protocol (MCP) versions supported by Agent Master Engine and the differences between them.

## Supported Versions

- **2024-11-05** - Full support
- **2025-03-26** - Full backward compatibility, new features preserved in metadata

## Version Differences

### MCP 2024-11-05 (Initial Release)

The initial public release of MCP included:

- **Transport Mechanisms**:
  - stdio (standard input/output)
  - HTTP with Server-Sent Events (SSE)
  
- **Core Features**:
  - Resources (context/data provision)
  - Prompts (templated workflows)
  - Tools (executable functions)
  - Sampling (server-initiated LLM interactions)
  
- **Message Format**: JSON-RPC 2.0

### MCP 2025-03-26 (Current Version)

This version introduced several enhancements while maintaining backward compatibility:

#### New Features

1. **OAuth 2.1 Authorization Framework**
   - Comprehensive authentication support
   - Scoped permissions for servers
   - Example: GitHub server can specify required OAuth scopes

2. **Streamable HTTP Transport**
   - Replaces HTTP+SSE with more flexible streaming
   - Better support for long-running operations
   - Improved connection management

3. **JSON-RPC Batching**
   - Multiple requests in a single message
   - Improved efficiency for bulk operations
   - Reduces network overhead

4. **Tool Annotations**
   - `readOnly`: Indicates if a tool only reads data
   - `destructive`: Warns if a tool can delete/modify data
   - Better safety and user consent mechanisms

5. **Audio Content Support**
   - New content type alongside text and images
   - Enables audio processing MCP servers
   - Standardized audio data handling

6. **Completions Capability**
   - Argument autocompletion support
   - Better developer experience
   - IDE-like features for tool usage

7. **Progress Notifications Enhancement**
   - Added `message` field to ProgressNotification
   - More descriptive status updates
   - Better user feedback during long operations

## Backward Compatibility

The 2025-03-26 version is fully backward compatible with 2024-11-05:

- All 2024-11-05 configurations work without modification
- New fields are optional and don't break older parsers
- SSE transport still supported (though Streamable HTTP is preferred)
- Core protocol semantics remain unchanged

## Implementation in Agent Master Engine

### Version Detection

Currently, Agent Master Engine does not explicitly detect MCP versions. Instead, it:

1. Parses all known configuration formats
2. Preserves unknown fields in metadata
3. Supports both old and new transport types
4. Handles missing optional fields gracefully

### Feature Support Matrix

| Feature | 2024-11-05 | 2025-03-26 | Engine Support |
|---------|------------|------------|----------------|
| stdio transport | ✅ | ✅ | ✅ Full |
| SSE transport | ✅ | ✅ (legacy) | ✅ Full |
| Streamable HTTP | ❌ | ✅ | 🟡 Treated as stdio |
| Basic auth | ✅ | ✅ | ✅ Via headers |
| OAuth 2.1 | ❌ | ✅ | 🟡 Metadata only |
| Tool annotations | ❌ | ✅ | 🟡 Preserved |
| Audio content | ❌ | ✅ | 🟡 Metadata only |
| JSON-RPC batch | ❌ | ✅ | 🟡 Metadata only |
| Completions | ❌ | ✅ | 🟡 Metadata only |

Legend:
- ✅ Full support
- 🟡 Partial support (data preserved but not actively used)
- ❌ Not supported

### Migration Considerations

When upgrading from 2024-11-05 to 2025-03-26:

1. **No breaking changes** - Existing configs continue to work
2. **Consider adding annotations** - Improve safety with readOnly/destructive flags
3. **OAuth migration** - Add OAuth configuration for better security
4. **Transport upgrade** - Consider moving from SSE to Streamable HTTP (when supported)

### Testing

Agent Master Engine includes version-specific test vectors:

- `testdata/mcp_v2024_11_05.json` - Tests 2024-11-05 features
- `testdata/mcp_v2025_03_26.json` - Tests 2025-03-26 features
- `mcp_version_test.go` - Compatibility test suite

Run version tests with:
```bash
go test -run TestMCPVersion
```