# Agent Master Engine Usage Guide

Agent Master Engine is a flexible Go library for managing Model Context Protocol (MCP) server configurations. This guide covers installation, basic usage, and common integration patterns.

## Installation

```bash
go get github.com/b-open-io/agent-master-engine
```

## Quick Start

```go
import "github.com/b-open-io/agent-master-engine"

// Create engine with default options
engine, err := agent.NewEngine(nil)
if err != nil {
    log.Fatal(err)
}

// Load existing configuration
err = engine.LoadConfig("~/.agent-master/config.json")

// Add a new MCP server
server := agent.ServerConfig{
    Transport: "stdio",
    Command: "npx",
    Args: []string{"my-mcp-server"},
}
err = engine.AddServer("my-server", server)

// Sync to a destination
result, err := engine.SyncToDestination("my-tool-config", nil)
```

## Core Concepts

### Servers
MCP servers provide tools and resources to AI assistants. Each server has:
- **Transport**: How to communicate (`stdio` or `sse`)
- **Configuration**: Command, arguments, environment variables, or URL
- **Metadata**: Internal tracking information

### Destinations
Destinations are where server configurations are synced to. The engine is destination-agnostic and can sync to any location using the `Destination` interface.

### Validation
The engine supports pluggable validation to ensure server configurations meet destination requirements.

## Common Operations

### Managing Servers

```go
// List all servers
servers, _ := engine.ListServers()

// Get specific server
server, _ := engine.GetServer("my-server")

// Update server
newConfig := agent.ServerConfig{
    Transport: "sse",
    URL: "http://localhost:3000",
}
engine.UpdateServer("my-server", newConfig)

// Remove server
engine.RemoveServer("my-server")
```

### Synchronization

```go
// Sync to all configured destinations
results, _ := engine.SyncToAllDestinations(nil)

// Sync with options
options := &agent.SyncOptions{
    DryRun: true,
    CreateBackup: true,
}
result, _ := engine.SyncToDestination("vscode", options)

// Preview changes before sync
preview, _ := engine.PreviewSync("cursor")
```

### Custom Destinations

```go
type MyDestination struct {
    endpoint string
}

func (d *MyDestination) GetID() string { return "my-dest" }
func (d *MyDestination) Transform(config *agent.Config) (interface{}, error) {
    // Transform to your format
    return myFormat, nil
}
func (d *MyDestination) Write(data []byte) error {
    // Write to your location
    return nil
}

// Register destination
engine.RegisterDestination(&MyDestination{endpoint: "https://api.example.com"})
```

### Custom Validation

```go
type MyValidator struct{}

func (v *MyValidator) ValidateName(name string) error {
    if len(name) > 50 {
        return fmt.Errorf("name too long")
    }
    return nil
}

func (v *MyValidator) ValidateConfig(config agent.ServerConfig) error {
    // Your validation logic
    return nil
}

engine.SetValidator(&MyValidator{})
```

## Integration Examples

### CLI Application

```go
func main() {
    engine, _ := agent.NewEngine(nil)
    
    // Add server command
    if len(os.Args) > 2 && os.Args[1] == "add" {
        name := os.Args[2]
        server := agent.ServerConfig{
            Transport: "stdio",
            Command: os.Args[3],
        }
        engine.AddServer(name, server)
    }
}
```

### Web Service

```go
func handleAddServer(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name   string               `json:"name"`
        Config agent.ServerConfig   `json:"config"`
    }
    
    json.NewDecoder(r.Body).Decode(&req)
    err := engine.AddServer(req.Name, req.Config)
    
    if err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    
    w.WriteHeader(http.StatusCreated)
}
```

## Best Practices

1. **Always validate** server configurations before adding
2. **Use transactions** for bulk operations
3. **Handle errors** gracefully in production
4. **Monitor sync operations** using the event system
5. **Test with memory storage** during development

## Events

The engine emits events for monitoring:

```go
// Subscribe to configuration changes
unsubscribe := engine.OnConfigChange(func(change agent.ConfigChange) {
    log.Printf("Config changed: %+v", change)
})

// Subscribe to sync completion
engine.OnSyncComplete(func(result agent.SyncResult) {
    log.Printf("Sync completed: %+v", result)
})
```

## Error Handling

```go
err := engine.AddServer("test", config)
if err != nil {
    switch {
    case errors.Is(err, agent.ErrServerExists):
        // Handle duplicate
    case errors.Is(err, agent.ErrInvalidConfig):
        // Handle validation error
    default:
        // Handle other errors
    }
}
```

## Advanced Features

### Projects
Group servers by project for better organization.

### Auto-sync
Automatically sync changes to destinations.

### Import/Export
Move configurations between systems.

See the API documentation for complete details on all available features.