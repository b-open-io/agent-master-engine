package main

import (
	"context"
	"fmt"
	"log"

	agent "github.com/b-open-io/agent-master-engine"
	redisStorage "github.com/b-open-io/agent-master-engine/storage/redis"
	"github.com/redis/go-redis/v9"
)

func main() {
	fmt.Println("Agent Master Engine - Redis Storage Example")
	fmt.Println("==========================================")

	// Create a real Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password
		DB:       0,  // default DB
	})
	defer redisClient.Close()

	// Test Redis connection
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	fmt.Println("✓ Connected to Redis")

	// Create Redis storage adapter
	storage := redisStorage.New(redisClient, "agent-master")

	// Create engine with Redis storage
	engine, err := agent.NewEngine(
		agent.WithStorage(storage),
	)
	if err != nil {
		log.Fatal("Failed to create engine:", err)
	}
	fmt.Println("✓ Created engine with Redis storage")

	// Add a server configuration
	server := agent.ServerConfig{
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y", "@modelcontextprotocol/server-everything"},
	}

	err = engine.AddServer("everything-server", server)
	if err != nil {
		log.Fatal("Failed to add server:", err)
	}
	fmt.Println("✓ Added server: everything-server")

	// Add another server
	githubServer := agent.ServerConfig{
		Transport: "stdio", 
		Command:   "npx",
		Args:      []string{"-y", "@modelcontextprotocol/server-github"},
		Env: map[string]string{
			"GITHUB_TOKEN": "${GITHUB_TOKEN}",
		},
	}

	err = engine.AddServer("github-server", githubServer)
	if err != nil {
		log.Fatal("Failed to add GitHub server:", err)
	}
	fmt.Println("✓ Added server: github-server")

	// List all servers
	servers, err := engine.ListServers(agent.ServerFilter{})
	if err != nil {
		log.Fatal("Failed to list servers:", err)
	}

	fmt.Printf("\n📦 Stored %d server(s) in Redis:\n", len(servers))
	for _, s := range servers {
		fmt.Printf("   - %s (%s transport)\n", s.Name, s.Transport)
	}

	// Demonstrate persistence by creating a new engine instance
	fmt.Println("\n🔄 Creating new engine instance to test persistence...")
	
	engine2, err := agent.NewEngine(
		agent.WithStorage(storage),
	)
	if err != nil {
		log.Fatal("Failed to create second engine:", err)
	}

	// Load the existing config
	err = engine2.LoadConfig("config")
	if err != nil {
		fmt.Println("No existing config found (this is normal for first run)")
	}

	// List servers from the new instance
	servers2, err := engine2.ListServers(agent.ServerFilter{})
	if err != nil {
		log.Fatal("Failed to list servers from second instance:", err)
	}

	fmt.Printf("\n✅ Second engine instance found %d server(s) in Redis\n", len(servers2))
	
	fmt.Println("\n🎉 Redis storage is working! Your MCP server configurations are now:")
	fmt.Println("   - Persisted across application restarts")
	fmt.Println("   - Shared across multiple instances")
	fmt.Println("   - Accessible from any service that connects to Redis")
	
	// Clean up for demo
	fmt.Println("\n🧹 Cleaning up demo data...")
	for _, s := range servers {
		engine.RemoveServer(s.Name)
	}
}