package main

import (
	"context"
	"fmt"
	"log"

	agent "github.com/b-open-io/agent-master-engine"
	"github.com/b-open-io/agent-master-engine/presets"
)

func main() {
	fmt.Println("Agent Master Engine - Basic Usage Example")
	fmt.Println("=========================================")

	// Create a new engine with default settings
	engine, err := agent.NewEngine(nil)
	if err != nil {
		log.Fatal("Failed to create engine:", err)
	}

	// Add some example servers
	servers := []struct {
		name   string
		config agent.ServerConfig
	}{
		{
			name: "memory",
			config: agent.ServerConfig{
				Transport: "stdio",
				Command:   "npx",
				Args:      []string{"-y", "@modelcontextprotocol/server-memory"},
			},
		},
		{
			name: "filesystem",
			config: agent.ServerConfig{
				Transport: "stdio",
				Command:   "npx",
				Args:      []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Env: map[string]string{
					"FILESYSTEM_ROOT": "/tmp",
				},
			},
		},
	}

	// Add servers to the engine
	for _, s := range servers {
		if err := engine.AddServer(s.name, s.config); err != nil {
			log.Printf("Failed to add server %s: %v", s.name, err)
			continue
		}
		fmt.Printf("✓ Added server: %s\n", s.name)
	}

	// Create a file destination for syncing
	dest := agent.NewFileDestination("my-config", "mcp-config.json", agent.ExportFormatJSON)
	err = engine.RegisterDestination("my-config", dest)
	if err != nil {
		log.Fatal("Failed to register destination:", err)
	}

	// Sync to the destination
	ctx := context.Background()
	result, err := engine.SyncTo(ctx, dest, agent.SyncOptions{})
	if err != nil {
		log.Fatal("Sync failed:", err)
	}

	fmt.Printf("✓ Synced %d servers to %s\n", result.ServersAdded, dest.GetID())

	// Example using presets
	fmt.Println("\nUsing presets:")
	claudeDest, err := presets.NewDestination("claude")
	if err == nil && claudeDest != nil {
		err = engine.RegisterDestination("claude", claudeDest)
		if err == nil {
			fmt.Printf("✓ Created Claude destination: %s\n", claudeDest.GetID())
		}
	}

	fmt.Println("\n✅ Example completed!")
}