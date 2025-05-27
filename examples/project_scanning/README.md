# Project Scanning Example

This example demonstrates how to use the Agent Master Engine's project scanning functionality to discover MCP (Model Context Protocol) projects in your development directories.

## Features Demonstrated

- **Project Discovery**: Automatically scan directories for projects containing MCP configurations
- **Multiple Format Support**: Detects various MCP configuration formats:
  - Claude Desktop format (`mcpServers` wrapper)
  - GitHub MCP format (nested `mcp` with `servers`)
  - Flat format (direct `servers`)
- **Project Registration**: Store discovered projects in the engine's configuration
- **Project Management**: List and retrieve registered projects

## Supported Project Types

The default project detector recognizes projects with these files:
- `package.json` (Node.js)
- `go.mod` (Go)
- `Cargo.toml` (Rust)
- `pyproject.toml` (Python)
- `requirements.txt` (Python)
- `pom.xml` (Java/Maven)
- `build.gradle` (Java/Gradle)
- `.project` (Eclipse)
- `mcp.json` (MCP configuration)
- `mcp-config.json` (MCP configuration)
- `.mcp` (MCP directory)

## MCP Configuration Detection

The scanner looks for MCP server configurations in these files:
- `mcp.json`
- `mcp-config.json`
- `.mcp/config.json`

### Example MCP Configuration

```json
{
  "mcpServers": {
    "filesystem": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/directory"]
    },
    "github": {
      "transport": "stdio", 
      "command": "docker",
      "args": ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server"]
    }
  }
}
```

## Running the Example

```bash
cd examples/project_scanning
go run main.go
```

## Sample Output

```
Scanning for MCP projects...
Found 2 projects:

1. my-mcp-project
   Path: /Users/dev/code/my-mcp-project
   Servers: 2
   MCP Servers:
     - filesystem (stdio)
       Command: npx
     - github (stdio)
       Command: docker

2. another-project
   Path: /Users/dev/projects/another-project
   Servers: 1
   MCP Servers:
     - memory (stdio)
       Command: npx

Registering project: my-mcp-project
Project registered successfully!

Registered projects:
- my-mcp-project (2 servers)
```

## Configuration

The scanner can be configured through the engine's settings:

```go
// Customize scanning behavior
config := &engine.Config{
    Settings: engine.Settings{
        ProjectScanning: engine.ProjectScanSettings{
            Enabled:      true,
            ScanPaths:    []string{"~/code", "~/projects"},
            ExcludePaths: []string{"node_modules", ".git", "dist"},
            MaxDepth:     5,
        },
    },
}
```

## Custom Project Detectors

You can create custom project detectors by implementing the `ProjectDetector` interface:

```go
type CustomDetector struct {
    // Custom detection logic
}

func (d *CustomDetector) IsProjectRoot(path string) bool {
    // Check if path contains your project indicators
    return true
}

func (d *CustomDetector) DetectProject(path string) (*engine.ProjectConfig, error) {
    // Create project configuration from detected files
    return &engine.ProjectConfig{
        Name: "custom-project",
        Path: path,
        // ... other fields
    }, nil
}
``` 