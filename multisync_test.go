package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestSyncToMultiple(t *testing.T) {
	// Create engine with memory storage
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Add some test servers
	testServers := map[string]ServerConfig{
		"server1": {
			Transport: "stdio",
			Command:   "test1",
			Args:      []string{"arg1"},
		},
		"server2": {
			Transport: "sse",
			URL:       "http://test.com",
			Headers:   map[string]string{"Auth": "token"},
		},
	}

	for name, cfg := range testServers {
		if err := e.AddServer(name, cfg); err != nil {
			t.Fatalf("Failed to add server %s: %v", name, err)
		}
	}

	// Create multiple test destinations
	dest1 := &mockDestination{id: "dest1", exists: false}
	dest2 := &mockDestination{id: "dest2", exists: false}
	dest3 := &mockDestination{id: "dest3", exists: true, failWrite: true} // This one will fail

	// Sync to multiple destinations
	ctx := context.Background()
	result, err := e.SyncToMultiple(ctx, []Destination{dest1, dest2, dest3}, SyncOptions{})
	if err != nil {
		t.Fatalf("SyncToMultiple failed: %v", err)
	}

	// Verify results
	if result.SuccessCount != 2 {
		t.Errorf("Expected 2 successes, got %d", result.SuccessCount)
	}
	if result.FailureCount != 1 {
		t.Errorf("Expected 1 failure, got %d", result.FailureCount)
	}
	if len(result.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result.Results))
	}

	// Check that successful destinations received data
	if dest1.writeCount != 1 {
		t.Error("Destination 1 should have been written to")
	}
	if dest2.writeCount != 1 {
		t.Error("Destination 2 should have been written to")
	}
	if dest3.writeCount != 0 {
		t.Error("Destination 3 should not have been written to (fails)")
	}

	// Verify timing
	if result.TotalDuration == 0 {
		t.Error("Total duration should be > 0")
	}
}

func TestSyncToMultipleEmpty(t *testing.T) {
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}

	// Try with empty destinations
	_, err = e.SyncToMultiple(context.Background(), []Destination{}, SyncOptions{})
	if err == nil {
		t.Error("Expected error for empty destinations")
	}
}

func TestPreviewSync(t *testing.T) {
	// Create engine with memory storage
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Add test servers
	if err := e.AddServer("server1", ServerConfig{
		Transport: "stdio",
		Command:   "test1",
		Args:      []string{"arg1"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := e.AddServer("server2", ServerConfig{
		Transport: "sse",
		URL:       "http://test.com",
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("Preview to new destination", func(t *testing.T) {
		dest := &mockDestination{id: "new-dest", exists: false}
		preview, err := e.PreviewSync(dest)
		if err != nil {
			t.Fatalf("PreviewSync failed: %v", err)
		}

		// Should show all servers as additions
		if len(preview.Changes) != 2 {
			t.Errorf("Expected 2 changes, got %d", len(preview.Changes))
		}

		for _, change := range preview.Changes {
			if change.Type != "add" {
				t.Errorf("Expected add change, got %s", change.Type)
			}
			if change.Before != nil {
				t.Error("Before should be nil for additions")
			}
			if change.After == nil {
				t.Error("After should not be nil for additions")
			}
		}

		if preview.RequiresBackup {
			t.Error("New destination should not require backup")
		}
	})

	t.Run("Preview to existing destination with changes", func(t *testing.T) {
		// Create destination with existing but different config
		existingConfig := &Config{
			Servers: map[string]ServerWithMetadata{
				"server1": {
					ServerConfig: ServerConfig{
						Transport: "stdio",
						Command:   "old-command", // Different command
					},
				},
				"server3": { // This server will be removed
					ServerConfig: ServerConfig{
						Transport: "stdio",
						Command:   "to-remove",
					},
				},
			},
		}

		dest := &mockDestination{
			id:             "existing-dest",
			exists:         true,
			existingConfig: existingConfig,
		}

		preview, err := e.PreviewSync(dest)
		if err != nil {
			t.Fatalf("PreviewSync failed: %v", err)
		}

		// Should show: server1 update, server2 add, server3 remove
		if len(preview.Changes) != 3 {
			t.Errorf("Expected 3 changes, got %d", len(preview.Changes))
		}

		// Count change types
		changeTypes := map[string]int{}
		for _, change := range preview.Changes {
			changeTypes[change.Type]++
		}

		if changeTypes["add"] != 1 {
			t.Errorf("Expected 1 addition, got %d", changeTypes["add"])
		}
		if changeTypes["update"] != 1 {
			t.Errorf("Expected 1 update, got %d", changeTypes["update"])
		}
		if changeTypes["remove"] != 1 {
			t.Errorf("Expected 1 removal, got %d", changeTypes["remove"])
		}

		if !preview.RequiresBackup {
			t.Error("Existing destination should require backup")
		}
	})

	t.Run("Preview with disabled server", func(t *testing.T) {
		// Disable server2 by removing and re-adding as disabled
		e.RemoveServer("server2")
		// Get the current config and manually update the server's enabled state
		config, _ := e.GetConfig()
		if server, exists := config.Servers["server2"]; exists {
			server.Internal.Enabled = false
			config.Servers["server2"] = server
			e.SetConfig(config)
		} else {
			// Re-add as disabled
			e.AddServer("server2", ServerConfig{
				Transport: "sse",
				URL:       "http://test.com",
			})
			config, _ = e.GetConfig()
			if server, exists := config.Servers["server2"]; exists {
				server.Internal.Enabled = false
				config.Servers["server2"] = server
				e.SetConfig(config)
			}
		}

		dest := &mockDestination{id: "test-dest", exists: false}
		preview, err := e.PreviewSync(dest)
		if err != nil {
			t.Fatal(err)
		}

		// Should only show server1 (server2 is disabled)
		if len(preview.Changes) != 1 {
			t.Errorf("Expected 1 change, got %d", len(preview.Changes))
		}
		if preview.Changes[0].Server != "server1" {
			t.Errorf("Expected server1, got %s", preview.Changes[0].Server)
		}
	})
}

// mockDestination for testing
type mockDestination struct {
	id             string
	exists         bool
	failWrite      bool
	writeCount     int
	existingConfig *Config
	writtenData    []byte
}

func (m *mockDestination) GetID() string {
	return m.id
}

func (m *mockDestination) GetDescription() string {
	return "Mock destination: " + m.id
}

func (m *mockDestination) Transform(config *Config) (interface{}, error) {
	// Simple transform - just return servers
	servers := make(map[string]ServerConfig)
	for name, server := range config.Servers {
		if server.Internal.Enabled {
			servers[name] = server.ServerConfig
		}
	}
	return map[string]interface{}{"servers": servers}, nil
}

func (m *mockDestination) Read() ([]byte, error) {
	if !m.exists || m.existingConfig == nil {
		return nil, nil
	}
	// Return existing config as JSON
	data, _ := json.Marshal(m.existingConfig)
	return data, nil
}

func (m *mockDestination) Write(data []byte) error {
	if m.failWrite {
		return fmt.Errorf("mock write failure")
	}
	m.writeCount++
	m.writtenData = data
	return nil
}

func (m *mockDestination) Exists() bool {
	return m.exists
}

func (m *mockDestination) SupportsBackup() bool {
	return true
}

func (m *mockDestination) Backup() (string, error) {
	return "/mock/backup/path", nil
}

func (m *mockDestination) Validate() error {
	return nil
}