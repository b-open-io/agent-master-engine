# Agent Master Daemon gRPC Client

A Go client library for interacting with the Agent Master daemon via gRPC.

## Features

- **Connection Management**: Support for both TCP and Unix socket connections
- **Automatic Retries**: Built-in retry logic with exponential backoff
- **Connection Keepalive**: Automatic connection health monitoring
- **Simplified API**: Wraps complex gRPC calls in simple Go methods
- **Event Streaming**: Subscribe to real-time daemon events
- **Thread-Safe**: Safe for concurrent use

## Installation

```bash
go get github.com/b-open-io/agent-master-engine/daemon/client
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/b-open-io/agent-master-engine/daemon/client"
    pb "github.com/b-open-io/agent-master-engine/daemon/proto"
)

func main() {
    // Create client with default options (TCP on localhost:50051)
    c := client.NewClient(client.DefaultOptions())
    
    // Connect to daemon
    ctx := context.Background()
    if err := c.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Add a server
    serverInfo, err := c.AddServer(ctx, "my-server", &pb.ServerConfig{
        Transport: "stdio",
        Command:   "node",
        Args:      []string{"server.js"},
        Enabled:   true,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Added server: %s", serverInfo.Name)
}
```

## Connection Options

### TCP Connection (Default)
```go
c := client.NewClient(&client.ClientOptions{
    Type:    client.ConnectionTCP,
    Address: "localhost:50051",
})
```

### Unix Socket Connection
```go
c := client.NewClient(&client.ClientOptions{
    Type:    client.ConnectionUnix,
    Address: "/tmp/agent-master.sock",
})
```

### Advanced Options
```go
c := client.NewClient(&client.ClientOptions{
    Type:                   client.ConnectionTCP,
    Address:                "localhost:50051",
    MaxRetries:             5,
    RetryDelay:             2 * time.Second,
    RetryBackoffMultiplier: 1.5,
    KeepaliveTime:          30 * time.Second,
    KeepaliveTimeout:       10 * time.Second,
    RequestTimeout:         30 * time.Second,
})
```

## API Methods

### Server Management
- `AddServer(ctx, name, config)` - Add a new server
- `UpdateServer(ctx, name, config)` - Update existing server
- `RemoveServer(ctx, name)` - Remove a server
- `GetServer(ctx, name)` - Get server details
- `ListServers(ctx, filter)` - List servers with optional filter
- `EnableServer(ctx, name)` - Enable a server
- `DisableServer(ctx, name)` - Disable a server

### Destination Management
- `RegisterDestination(ctx, name, type, path, options)` - Register a destination
- `RemoveDestination(ctx, name)` - Remove a destination
- `ListDestinations(ctx)` - List all destinations

### Sync Operations
- `SyncTo(ctx, destination, options)` - Sync to a single destination
- `SyncToMultiple(ctx, destinations, options)` - Sync to multiple destinations
- `PreviewSync(ctx, destination)` - Preview sync changes

### Auto-sync
- `StartAutoSync(ctx, config)` - Start auto-sync with configuration
- `StopAutoSync(ctx)` - Stop auto-sync
- `GetAutoSyncStatus(ctx)` - Get auto-sync status

### Configuration
- `GetConfig(ctx)` - Get current configuration
- `SetConfig(ctx, config)` - Update configuration
- `LoadConfig(ctx, path)` - Load configuration from file
- `SaveConfig(ctx)` - Save current configuration

### Daemon Control
- `GetStatus(ctx)` - Get daemon status
- `Shutdown(ctx)` - Shutdown the daemon

### Events
- `Subscribe(ctx, eventTypes, handler)` - Subscribe to daemon events

## Event Handling

Subscribe to real-time events from the daemon:

```go
err := c.Subscribe(ctx, []pb.EventType{
    pb.EventType_CONFIG_CHANGE,
    pb.EventType_SYNC_COMPLETE,
    pb.EventType_ERROR,
}, func(event *pb.Event) error {
    switch event.Type {
    case pb.EventType_CONFIG_CHANGE:
        log.Printf("Config changed: %v", event.GetConfigChange())
    case pb.EventType_SYNC_COMPLETE:
        sync := event.GetSyncComplete()
        log.Printf("Sync to %s: %v", sync.Destination, sync.Success)
    case pb.EventType_ERROR:
        err := event.GetError()
        log.Printf("Error: %s", err.Message)
    }
    return nil
})
```

## Error Handling

The client includes automatic retry logic for transient errors:
- `Unavailable` - Service temporarily unavailable
- `DeadlineExceeded` - Request timeout
- `ResourceExhausted` - Rate limiting

Non-retryable errors are returned immediately.

## Thread Safety

The client is thread-safe and can be used concurrently from multiple goroutines. A single client instance can be shared across your application.

## Examples

See `example_test.go` for comprehensive examples including:
- Basic server management
- Batch operations
- Auto-sync configuration
- Event subscription
- Multi-destination sync