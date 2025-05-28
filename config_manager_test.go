package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileConsistency(t *testing.T) {
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")

	// Create initial config with one server
	initialConfig := &Config{
		Version: "1.0.0",
		Servers: map[string]ServerWithMetadata{
			"server1": {
				ServerConfig: ServerConfig{
					Transport: "stdio",
					Command:   "echo",
					Args:      []string{"hello"},
				},
				Internal: InternalMetadata{
					Enabled: true,
					Source:  "test",
				},
			},
		},
	}

	// Write initial config to file
	data, err := json.MarshalIndent(initialConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Create engine and load config
	engine, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Load config from file
	if err := engine.LoadConfig(configPath); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify server1 is loaded
	server1, err := engine.GetServer("server1")
	if err != nil {
		t.Fatalf("Failed to get server1: %v", err)
	}
	if server1.Command != "echo" {
		t.Errorf("Expected command 'echo', got '%s'", server1.Command)
	}

	// Add a second server
	server2Config := ServerConfig{
		Transport: "stdio",
		Command:   "cat",
		Args:      []string{"file.txt"},
	}
	if err := engine.AddServer("server2", server2Config); err != nil {
		t.Fatalf("Failed to add server2: %v", err)
	}

	// Verify both servers exist
	servers, err := engine.ListServers(ServerFilter{})
	if err != nil {
		t.Fatalf("Failed to list servers: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("Expected 2 servers, got %d", len(servers))
	}

	// Verify server1 still exists (this was the bug)
	server1Again, err := engine.GetServer("server1")
	if err != nil {
		t.Fatalf("Server1 was lost after adding server2: %v", err)
	}
	if server1Again.Command != "echo" {
		t.Errorf("Server1 command changed: expected 'echo', got '%s'", server1Again.Command)
	}

	// Verify server2 exists
	server2, err := engine.GetServer("server2")
	if err != nil {
		t.Fatalf("Failed to get server2: %v", err)
	}
	if server2.Command != "cat" {
		t.Errorf("Expected command 'cat', got '%s'", server2.Command)
	}

	// Verify the file was updated correctly
	fileData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var fileConfig Config
	if err := json.Unmarshal(fileData, &fileConfig); err != nil {
		t.Fatalf("Failed to parse config file: %v", err)
	}

	if len(fileConfig.Servers) != 2 {
		t.Fatalf("Config file should have 2 servers, got %d", len(fileConfig.Servers))
	}

	if _, exists := fileConfig.Servers["server1"]; !exists {
		t.Error("server1 missing from config file")
	}
	if _, exists := fileConfig.Servers["server2"]; !exists {
		t.Error("server2 missing from config file")
	}
}

func TestLoadConfigStorageBackend(t *testing.T) {
	// Test that storage backend still works when no file path is provided
	engine, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Add a server without loading from file
	serverConfig := ServerConfig{
		Transport: "stdio",
		Command:   "test",
	}
	if err := engine.AddServer("test-server", serverConfig); err != nil {
		t.Fatalf("Failed to add server: %v", err)
	}

	// Save config (should use storage backend)
	if err := engine.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create new engine and verify server persists
	engine2, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create second engine: %v", err)
	}

	// This should load from storage backend since no file path was set
	config, err := engine2.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// Note: Memory storage doesn't persist between engine instances
	// This test verifies the code path works without errors
	if config == nil {
		t.Error("Config should not be nil")
	}
}
