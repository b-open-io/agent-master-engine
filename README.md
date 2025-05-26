# Agent Master Engine

A flexible Go library for managing Model Context Protocol (MCP) server configurations. This engine provides a generic, extensible framework for synchronizing MCP servers across different AI development tools.

## Features

- **Platform-agnostic design** - No hardcoded tool references
- **Pluggable validation** - Customize validation rules for your needs
- **Flexible synchronization** - Sync to any destination using the Destination interface
- **Multiple storage backends** - File-based or in-memory storage
- **Event system** - Monitor configuration changes and sync operations
- **Transaction support** - Atomic bulk operations
- **Extensible architecture** - Easy to add new features and integrations

## Installation

```bash
go get github.com/b-open-io/agent-master-engine
```

## Quick Start

```go
package main

import (
    "log"
    agent "github.com/b-open-io/agent-master-engine"
)

func main() {
    // Create a new engine
    engine, err := agent.NewEngine(nil)
    if err != nil {
        log.Fatal(err)
    }

    // Add an MCP server
    server := agent.ServerConfig{
        Transport: "stdio",
        Command: "npx",
        Args: []string{"my-mcp-server"},
    }
    
    err = engine.AddServer("my-server", server)
    if err != nil {
        log.Fatal(err)
    }

    // Sync to a destination
    result, err := engine.SyncToDestination("my-config", nil)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Sync completed: %+v", result)
}
```

## Core Concepts

### MCP Servers

Model Context Protocol servers provide tools and resources to AI assistants. They can use:
- **stdio transport** - Communication via standard input/output
- **SSE transport** - Server-sent events over HTTP

### Destinations

Destinations define where server configurations are synced to. The engine provides a generic `Destination` interface that can be implemented for any target system.

### Validation

The engine supports pluggable validation through the `ServerValidator` interface, allowing you to enforce custom rules for server names and configurations.

## Documentation

- [Usage Guide](docs/USAGE_GUIDE.md) - Detailed usage instructions and examples
- [API Reference](docs/API_REFERENCE.md) - Complete API documentation

## Architecture

The engine follows a clean architecture pattern with well-defined interfaces:

```
┌─────────────────┐
│     Engine      │  Main interface
├─────────────────┤
│  Storage Layer  │  File/Memory backends
├─────────────────┤
│  Sync Manager   │  Handles synchronization
├─────────────────┤
│  Event System   │  Configuration monitoring
└─────────────────┘
```

## Custom Integrations

### Implementing a Custom Destination

```go
type MyDestination struct {
    apiEndpoint string
}

func (d *MyDestination) GetID() string {
    return "my-destination"
}

func (d *MyDestination) Transform(config *agent.Config) (interface{}, error) {
    // Transform to your format
    return myFormat, nil
}

func (d *MyDestination) Write(data []byte) error {
    // Write to your destination
    return nil
}

// Register with engine
engine.RegisterDestination(&MyDestination{
    apiEndpoint: "https://api.example.com",
})
```

### Custom Validation

```go
type MyValidator struct{}

func (v *MyValidator) ValidateName(name string) error {
    // Your validation logic
    if len(name) > 50 {
        return fmt.Errorf("name too long")
    }
    return nil
}

func (v *MyValidator) ValidateConfig(config agent.ServerConfig) error {
    // Validate configuration
    return nil
}

// Set on engine
engine.SetValidator(&MyValidator{})
```

## Testing

```bash
# Run unit tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built for managing [Model Context Protocol](https://modelcontextprotocol.io) servers
- Inspired by the need for unified MCP server management across AI tools