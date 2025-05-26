# Storage Adapters

This directory contains example storage adapter implementations for the Agent Master Engine.

## Available Adapters

### Redis Storage

The `redis` package provides a Redis-based storage implementation. This is useful for:

- **Distributed systems** - Share configuration across multiple instances
- **Persistence** - Survive application restarts
- **High performance** - Redis's in-memory speed with optional persistence
- **Pub/Sub** - Real-time updates across instances

## Creating Custom Storage Adapters

To create a custom storage adapter, implement the `Storage` interface:

```go
type Storage interface {
    Read(key string) ([]byte, error)
    Write(key string, data []byte) error
    Delete(key string) error
    List(prefix string) ([]string, error)
    Watch(key string, handler func([]byte)) (func(), error)
}
```

### Example: PostgreSQL Storage

```go
package postgres

import (
    "database/sql"
    "encoding/json"
)

type Storage struct {
    db     *sql.DB
    table  string
}

func (s *Storage) Read(key string) ([]byte, error) {
    var data []byte
    err := s.db.QueryRow(
        "SELECT value FROM "+s.table+" WHERE key = $1", 
        key,
    ).Scan(&data)
    return data, err
}

func (s *Storage) Write(key string, data []byte) error {
    _, err := s.db.Exec(
        "INSERT INTO "+s.table+" (key, value) VALUES ($1, $2) "+
        "ON CONFLICT (key) DO UPDATE SET value = $2",
        key, data,
    )
    return err
}

// ... implement other methods
```

## Using Storage Adapters

```go
import (
    agent "github.com/b-open-io/agent-master-engine"
    "github.com/b-open-io/agent-master-engine/storage/redis"
)

// Create storage adapter
storage := redis.New(redisClient, "agent-master")

// Use with engine
engine, err := agent.NewEngine(
    agent.WithStorage(storage),
)
```

## Best Practices

1. **Key Namespacing** - Use prefixes to avoid key collisions
2. **Error Handling** - Return appropriate errors for "not found" vs actual errors
3. **Atomic Operations** - Ensure write operations are atomic when possible
4. **Connection Management** - Handle connection pooling and cleanup
5. **Watch Implementation** - Use native pub/sub features when available