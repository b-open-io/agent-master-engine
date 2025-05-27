package client_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/b-open-io/agent-master-engine/daemon/client"
	pb "github.com/b-open-io/agent-master-engine/daemon/proto"
)

func ExampleClient_tcp() {
	// Create a client with default TCP options
	c := client.NewClient(&client.ClientOptions{
		Type:           client.ConnectionTCP,
		Address:        "localhost:50051",
		RequestTimeout: 10 * time.Second,
	})

	// Connect to the daemon
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close()

	// Add a new server
	serverConfig := &pb.ServerConfig{
		Transport: "stdio",
		Command:   "node",
		Args:      []string{"/path/to/server.js"},
		Enabled:   true,
		Metadata: map[string]string{
			"version": "1.0.0",
		},
	}

	serverInfo, err := c.AddServer(ctx, "my-mcp-server", serverConfig)
	if err != nil {
		log.Fatalf("Failed to add server: %v", err)
	}
	fmt.Printf("Added server: %s\n", serverInfo.Name)

	// List all servers
	servers, err := c.ListServers(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list servers: %v", err)
	}
	fmt.Printf("Found %d servers\n", len(servers))

	// Sync to Claude
	syncResult, err := c.SyncTo(ctx, "claude", &pb.SyncOptions{
		Force:  false,
		Backup: true,
	})
	if err != nil {
		log.Fatalf("Failed to sync: %v", err)
	}
	fmt.Printf("Synced %d servers successfully\n", syncResult.ServersSynced)
}

func ExampleClient_unixSocket() {
	// Create a client for Unix socket connection
	c := client.NewClient(&client.ClientOptions{
		Type:    client.ConnectionUnix,
		Address: "/tmp/agent-master.sock",
	})

	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close()

	// Get daemon status
	status, err := c.GetStatus(ctx)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	fmt.Printf("Daemon version: %s\n", status.Version)
	fmt.Printf("Uptime: %d seconds\n", status.UptimeSeconds)
	fmt.Printf("Auto-sync running: %v\n", status.AutoSyncRunning)
}

func ExampleClient_Subscribe() {
	c := client.NewClient(client.DefaultOptions())
	
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close()

	// Subscribe to all event types
	eventTypes := []pb.EventType{
		pb.EventType_CONFIG_CHANGE,
		pb.EventType_SYNC_COMPLETE,
		pb.EventType_ERROR,
		pb.EventType_AUTO_SYNC_STATUS,
	}

	err := c.Subscribe(ctx, eventTypes, func(event *pb.Event) error {
		fmt.Printf("Received event: %s at %s\n", 
			event.Type.String(), 
			event.Timestamp.AsTime().Format(time.RFC3339))

		// Handle specific event types
		switch event.Type {
		case pb.EventType_CONFIG_CHANGE:
			if cfg := event.GetConfigChange(); cfg != nil {
				fmt.Printf("Config changed: %s\n", cfg.ChangeType)
			}
		case pb.EventType_SYNC_COMPLETE:
			if sync := event.GetSyncComplete(); sync != nil {
				fmt.Printf("Sync completed to %s: success=%v\n", 
					sync.Destination, sync.Success)
			}
		case pb.EventType_ERROR:
			if err := event.GetError(); err != nil {
				fmt.Printf("Error in %s: %s\n", err.Component, err.Message)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Keep listening for events
	select {}
}

func ExampleClient_AutoSync() {
	c := client.NewClient(client.DefaultOptions())
	
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close()

	// Configure and start auto-sync
	autoSyncConfig := &pb.AutoSyncConfig{
		Enabled:          true,
		WatchIntervalMs:  5000,  // 5 seconds
		DebounceDelayMs:  1000,  // 1 second
		TargetWhitelist:  []string{"claude", "cursor", "vscode"},
		IgnorePatterns:   []string{"*.tmp", "*.bak"},
	}

	if err := c.StartAutoSync(ctx, autoSyncConfig); err != nil {
		log.Fatalf("Failed to start auto-sync: %v", err)
	}

	// Check auto-sync status
	status, err := c.GetAutoSyncStatus(ctx)
	if err != nil {
		log.Fatalf("Failed to get auto-sync status: %v", err)
	}

	fmt.Printf("Auto-sync enabled: %v\n", status.Enabled)
	fmt.Printf("Auto-sync running: %v\n", status.Running)
	if status.LastSync != nil {
		fmt.Printf("Last sync: %s\n", status.LastSync.AsTime().Format(time.RFC3339))
	}

	// Stop auto-sync when done
	if err := c.StopAutoSync(ctx); err != nil {
		log.Fatalf("Failed to stop auto-sync: %v", err)
	}
}

func ExampleClient_BatchOperations() {
	c := client.NewClient(client.DefaultOptions())
	
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close()

	// Add multiple servers
	servers := map[string]*pb.ServerConfig{
		"typescript-server": {
			Transport: "stdio",
			Command:   "node",
			Args:      []string{"dist/index.js"},
			Enabled:   true,
		},
		"python-server": {
			Transport: "stdio", 
			Command:   "python",
			Args:      []string{"-m", "myserver"},
			Enabled:   true,
		},
		"go-server": {
			Transport: "stdio",
			Command:   "./myserver",
			Enabled:   false,
		},
	}

	for name, config := range servers {
		_, err := c.AddServer(ctx, name, config)
		if err != nil {
			log.Printf("Failed to add server %s: %v", name, err)
			continue
		}
		fmt.Printf("Added server: %s\n", name)
	}

	// List only enabled servers
	enabledServers, err := c.ListServers(ctx, &pb.ServerFilter{
		EnabledOnly: true,
	})
	if err != nil {
		log.Fatalf("Failed to list servers: %v", err)
	}
	fmt.Printf("Found %d enabled servers\n", len(enabledServers))

	// Sync to multiple destinations
	destinations := []string{"claude", "cursor", "vscode"}
	multiResult, err := c.SyncToMultiple(ctx, destinations, &pb.SyncOptions{
		Force:  false,
		Backup: true,
	})
	if err != nil {
		log.Fatalf("Failed to sync to multiple: %v", err)
	}

	fmt.Printf("Sync results - Success: %d, Failed: %d\n",
		multiResult.TotalSuccess, multiResult.TotalFailed)
	
	for dest, result := range multiResult.Results {
		fmt.Printf("  %s: %v\n", dest, result.Success)
	}
}