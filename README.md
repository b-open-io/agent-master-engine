# 🔧 Agent Master Engine

Agent Master Engine is a Go library for managing Model Context Protocol (MCP) server configurations. It helps you keep MCP servers in sync across different tools.

---

## 🚀 What It Does

- **No tool-specific code**: Works with any system; nothing is hardcoded
- **Multi-format support**: Parse Claude Desktop, VS Code/GitHub, and flat MCP configurations
- **Multi-destination sync**: Synchronize to multiple targets concurrently
- **Preview changes**: See what will be modified before applying
- **Auto-sync**: Watch for configuration changes and sync automatically
- **Custom validation**: Define your own rules for server names and settings
- **Sync anywhere**: Send configurations to any target by implementing the Destination interface
- **Choose your storage**: Use files, in-memory, or Redis (more adapters can be added)
- **Track changes**: Get notified when configurations change or sync operations occur
- **Variable substitution**: Optional environment variable substitution
- **Import configurations**: Import from various MCP format files

---

## 📦 Installation

```bash
go get github.com/b-open-io/agent-master-engine
```

---

## 🧪 Quick Start

```go
package main

import (
    "context"
    "log"
    agent "github.com/b-open-io/agent-master-engine"
)

func main() {
    engine, err := agent.NewEngine(nil)
    if err != nil {
        log.Fatal(err)
    }

    server := agent.ServerConfig{
        Transport: "stdio",
        Command:   "npx",
        Args:      []string{"my-mcp-server"},
    }

    err = engine.AddServer("my-server", server)
    if err != nil {
        log.Fatal(err)
    }

    dest := agent.NewFileDestination("vscode", "~/.vscode/mcp.json", nil)
    engine.RegisterDestination("vscode", dest)

    // Preview changes before syncing
    preview, err := engine.PreviewSync(dest)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Will make %d changes", len(preview.Changes))

    // Sync to single destination
    ctx := context.Background()
    result, err := engine.SyncTo(ctx, dest, agent.SyncOptions{})
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Sync completed: %d servers synced", result.ServersAdded)
}
```

### Multi-Destination Sync

```go
// Sync to multiple destinations at once
dest1 := agent.NewFileDestination("vscode", "~/.vscode/mcp.json", nil)
dest2 := agent.NewFileDestination("cursor", "~/.cursor/mcp.json", nil)
dest3 := agent.NewFileDestination("claude", "~/Library/Application Support/Claude/mcp.json", nil)

dests := []agent.Destination{dest1, dest2, dest3}
result, err := engine.SyncToMultiple(ctx, dests, agent.SyncOptions{})
if err != nil {
    log.Fatal(err)
}
log.Printf("Synced to %d/%d destinations successfully", result.SuccessCount, len(dests))
```

### Import MCP Configurations

```go
// Import from various MCP formats
data, err := os.ReadFile("github-mcp-config.json")
if err != nil {
    log.Fatal(err)
}

err = engine.Import(data, agent.ImportFormat("mcp"), agent.ImportOptions{
    OverwriteExisting: true,
    SubstituteEnvVars: true, // Replace ${ENV_VAR} with actual values
})
if err != nil {
    log.Fatal(err)
}
```

### Auto-Sync

```go
// Enable auto-sync to watch for changes
err = engine.StartAutoSync(agent.AutoSyncConfig{
    Enabled:       true,
    WatchInterval: 1 * time.Second,
    DebounceDelay: 500 * time.Millisecond,
    Destinations:  []string{"vscode", "cursor"},
})
if err != nil {
    log.Fatal(err)
}

// Stop auto-sync when done
defer engine.StopAutoSync()
```

---

## 🧠 Core Concepts

### MCP Servers

MCP servers provide tools and resources to AI assistants. They support two communication transports:
- **stdio Transport**: Communication via standard input/output streams
- **sse Transport**: Server-Sent Events over HTTP for real-time updates

### Destinations

Destinations are targets where server configurations are synchronized. The engine provides a Destination interface that can be implemented for any target system.

### Validation

Customize server name and configuration validation by implementing the ServerValidator interface. This allows enforcement of specific rules and constraints.

---

## 📚 Documentation

- 📖 [Usage Guide](docs/USAGE_GUIDE.md) - Detailed instructions and examples
- 🔧 [API Reference](docs/API_REFERENCE.md) - Complete API documentation

---

## 🏗️ Architecture Overview

The engine follows a clean architecture pattern with distinct layers:

```
┌─────────────────┐
│     Engine      │  ← Main interface
├─────────────────┤
│  Storage Layer  │  ← File/Memory backends
├─────────────────┤
│  Sync Manager   │  ← Handles synchronization
├─────────────────┤
│  Event System   │  ← Configuration monitoring
└─────────────────┘
```

---

## 🔌 Custom Integrations

### Implementing a Custom Destination

```go
type MyDestination struct {
    apiEndpoint string
}

func (d *MyDestination) GetID() string {
    return "my-destination"
}

func (d *MyDestination) GetDescription() string {
    return "Custom API destination"
}

func (d *MyDestination) Transform(config *agent.Config) (interface{}, error) {
    // Transform the configuration to your desired format
    return myFormat, nil
}

func (d *MyDestination) Write(data []byte) error {
    // Write the transformed data to your destination
    return nil
}

func (d *MyDestination) Read() ([]byte, error) {
    // Read existing configuration
    return existingData, nil
}

func (d *MyDestination) Exists() bool {
    return true
}

func (d *MyDestination) SupportsBackup() bool {
    return false
}

func (d *MyDestination) Backup() (string, error) {
    return "", nil
}

// Register the custom destination with the engine
engine.RegisterDestination("my-api", &MyDestination{
    apiEndpoint: "https://api.example.com",
})
```

### Custom Storage Adapter

```go
// Implement the Storage interface for any backend
type Storage interface {
    Read(key string) ([]byte, error)
    Write(key string, data []byte) error
    Delete(key string) error
    List(prefix string) ([]string, error)
    Watch(key string, handler func([]byte)) (func(), error)
}

// Example: Redis storage adapter
type RedisStorage struct {
    client *redis.Client
    prefix string
}

func (r *RedisStorage) Read(key string) ([]byte, error) {
    return r.client.Get(context.Background(), r.prefix+":"+key).Bytes()
}

func (r *RedisStorage) Write(key string, data []byte) error {
    return r.client.Set(context.Background(), r.prefix+":"+key, data, 0).Err()
}

// ... implement other methods

// Use custom storage with the engine
storage := &RedisStorage{client: redisClient, prefix: "agent-master"}
engine, err := agent.NewEngine(agent.WithStorage(storage))
```

### Custom Validation

```go
type MyValidator struct{}

func (v *MyValidator) ValidateName(name string) error {
    // Implement custom name validation logic
    if len(name) > 50 {
        return fmt.Errorf("name too long (max 50 characters)")
    }
    return nil
}

func (v *MyValidator) ValidateConfig(config agent.ServerConfig) error {
    // Implement custom configuration validation
    if config.Transport == "stdio" && config.Command == "" {
        return fmt.Errorf("stdio transport requires a command")
    }
    return nil
}

// Set the custom validator on the engine
engine.SetValidator(&MyValidator{})
```

---

## 🧪 Testing

```bash
# Run unit tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./...

# Run with coverage
go test -cover ./...
```

---

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- Built for managing [Model Context Protocol](https://modelcontextprotocol.io) servers
- Inspired by the need for unified MCP server management across AI tools

---

## 📞 Support

- 📧 Create an issue for bug reports or feature requests
- 💬 Join the discussion in our [GitHub Discussions](https://github.com/b-open-io/agent-master-engine/discussions)
- 📖 Check out the [examples](examples/) directory for more usage patterns