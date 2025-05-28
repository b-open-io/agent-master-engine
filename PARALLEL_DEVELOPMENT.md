# Parallel Development Instructions

## Current Setup
You have 3 worktrees set up:
1. **agent-master-engine** (main repo) - Where the daemon lives
2. **agent-master-cli-client** - CLI that will connect to daemon
3. **agent-master-wails-client** - Wails app that will connect to daemon

## What to Run in Each Worktree

### Terminal 1: Engine Daemon Development
```bash
cd /Users/satchmo/code/agent-master-engine

# First, generate the proto files
protoc --go_out=. --go-grpc_out=. daemon/proto/daemon.proto

# Build the daemon
go build -o agent-master-daemon ./cmd/daemon

# Run the daemon (will create lock file and start listening)
./agent-master-daemon --port 50051 --storage ~/.agent-master/daemon
```

### Terminal 2: CLI Client Development
```bash
cd /Users/satchmo/code/agent-master-cli-client

# The CLI needs to be updated to use the daemon client
# For now, you can test the existing CLI functionality
go build -o am ./cmd

# Test commands (these will need updating to use daemon)
./am list
./am sync
```

### Terminal 3: Wails App Development
```bash
cd /Users/satchmo/code/agent-master-wails-client

# Run the Wails app in dev mode
~/go/bin/wails dev
```

## Development Tasks

### Engine (agent-master-engine)
1. Complete `daemon/conversions.go` - convert between proto and engine types
2. Finish `daemon/service.go` implementation
3. Add client library at `daemon/client/client.go`
4. Test daemon with auto-sync persistence

### CLI (agent-master-cli-client)
1. Remove old daemon code from `daemon/` directory
2. Import engine's daemon client
3. Update all commands to use daemon client instead of direct engine
4. Update `autosync` command to talk to daemon

### Wails (agent-master-wails-client)
1. Remove any daemon code
2. Import engine's daemon client
3. Update `engine_integration.go` to use daemon client
4. Ensure auto-sync UI reflects daemon state

## Testing Integration
Once daemon is running in Terminal 1:
- Start auto-sync from CLI in Terminal 2
- Open Wails app in Terminal 3 - should see auto-sync is running
- Close CLI - auto-sync should continue
- Check Wails app - should still show auto-sync active