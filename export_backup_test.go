package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExport(t *testing.T) {
	// Create engine with memory storage
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}

	// Add test servers
	servers := map[string]ServerConfig{
		"test1": {Transport: "stdio", Command: "test1"},
		"test2": {Transport: "sse", URL: "http://test.com"},
	}

	for name, cfg := range servers {
		if err := e.AddServer(name, cfg); err != nil {
			t.Fatal(err)
		}
	}

	// Test JSON export
	t.Run("Export JSON", func(t *testing.T) {
		data, err := e.Export(ExportFormatJSON)
		if err != nil {
			t.Fatal(err)
		}

		// Verify it's valid JSON
		var config Config
		if err := json.Unmarshal(data, &config); err != nil {
			t.Errorf("Invalid JSON: %v", err)
		}

		// Check servers are included
		if len(config.Servers) != 2 {
			t.Errorf("Expected 2 servers, got %d", len(config.Servers))
		}
	})

	// Test unsupported format
	t.Run("Unsupported format", func(t *testing.T) {
		_, err := e.Export(ExportFormat("xml"))
		if err == nil {
			t.Error("Expected error for unsupported format")
		}
	})
}

func TestExportToFile(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create engine
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}

	// Add a server
	e.AddServer("test", ServerConfig{Transport: "stdio", Command: "test"})

	// Export to file
	exportPath := filepath.Join(tmpDir, "export.json")
	data, err := e.Export(ExportFormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	
	// Write to file manually
	if err := os.WriteFile(exportPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Error("Export file not created")
	}

	// Read and verify content
	readData, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}

	var config Config
	if err := json.Unmarshal(readData, &config); err != nil {
		t.Errorf("Invalid JSON in export file: %v", err)
	}
}

func TestBackupRestore(t *testing.T) {
	// Create engine with memory storage
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}

	// Add initial servers
	originalServers := map[string]ServerConfig{
		"server1": {Transport: "stdio", Command: "cmd1"},
		"server2": {Transport: "sse", URL: "http://test.com"},
	}

	for name, cfg := range originalServers {
		e.AddServer(name, cfg)
	}

	// Create backup
	backup, err := e.CreateBackup("test backup")
	if err != nil {
		t.Fatal(err)
	}

	if backup.ID == "" {
		t.Error("Backup ID should not be empty")
	}
	if backup.Description != "test backup" {
		t.Errorf("Expected description 'test backup', got %s", backup.Description)
	}

	// Modify config
	e.RemoveServer("server1")
	e.AddServer("server3", ServerConfig{Transport: "stdio", Command: "new"})

	// Verify changes
	config, _ := e.GetConfig()
	if len(config.Servers) != 2 {
		t.Error("Expected 2 servers after modification")
	}

	// List backups
	backups, err := e.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 {
		t.Error("Expected at least one backup")
	}

	// Restore from backup
	err = e.RestoreBackup(backup.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify restoration
	config, _ = e.GetConfig()
	if len(config.Servers) != 2 {
		t.Errorf("Expected 2 servers after restore, got %d", len(config.Servers))
	}
	if _, exists := config.Servers["server1"]; !exists {
		t.Error("server1 should exist after restore")
	}
	if _, exists := config.Servers["server3"]; exists {
		t.Error("server3 should not exist after restore")
	}
}

func TestBackupCleanup(t *testing.T) {
	// Create engine with custom backup settings
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}

	// Add a server first (need something to backup)
	e.AddServer("test", ServerConfig{Transport: "stdio", Command: "test"})

	// Set max backups to 3
	config, _ := e.GetConfig()
	config.Settings.Backup.MaxBackups = 3
	e.SetConfig(config)

	// Create 5 backups
	createdBackups := []string{}
	for i := 0; i < 5; i++ {
		desc := string(rune('A' + i))
		backup, err := e.CreateBackup(desc)
		if err != nil {
			t.Fatal(err)
		}
		createdBackups = append(createdBackups, backup.ID)
		t.Logf("Created backup %d: %s (%s)", i, desc, backup.ID)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List backups - should only have 3
	backups, err := e.ListBackups()
	if err != nil {
		t.Fatal(err)
	}

	// Debug: print all backup descriptions
	t.Logf("Found %d backups:", len(backups))
	for i, b := range backups {
		t.Logf("  [%d] %s - %s", i, b.Description, b.Timestamp.Format("15:04:05.000"))
	}

	if len(backups) != 3 {
		t.Errorf("Expected 3 backups after cleanup, got %d", len(backups))
	}

	// Verify we kept the newest ones
	for i, backup := range backups {
		expectedDesc := string(rune('E' - i)) // E, D, C (newest first)
		if backup.Description != expectedDesc {
			t.Errorf("Expected backup %d to have description %s, got %s", 
				i, expectedDesc, backup.Description)
		}
	}
}

func TestMergeConfigs(t *testing.T) {
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}

	// Create test configs
	config1 := &Config{
		Version: "1.0.0",
		Servers: map[string]ServerWithMetadata{
			"server1": {
				ServerConfig: ServerConfig{Transport: "stdio", Command: "cmd1"},
			},
			"shared": {
				ServerConfig: ServerConfig{Transport: "stdio", Command: "old"},
			},
		},
	}

	config2 := &Config{
		Version: "1.0.1",
		Servers: map[string]ServerWithMetadata{
			"server2": {
				ServerConfig: ServerConfig{Transport: "sse", URL: "http://test"},
			},
			"shared": {
				ServerConfig: ServerConfig{Transport: "stdio", Command: "new"},
			},
		},
	}

	// Merge configs
	merged, err := e.MergeConfigs(config1, config2)
	if err != nil {
		t.Fatal(err)
	}

	// Check results
	if len(merged.Servers) != 3 {
		t.Errorf("Expected 3 servers, got %d", len(merged.Servers))
	}

	// Check conflict was tracked
	if merged.Metadata["conflicts"] == nil {
		t.Error("Expected conflicts to be tracked")
	}

	conflicts := merged.Metadata["conflicts"].([]map[string]interface{})
	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(conflicts))
	}

	// Check last-one-wins behavior
	if merged.Servers["shared"].Command != "new" {
		t.Error("Expected 'shared' server to have command 'new' (last one wins)")
	}

	// Check metadata
	if merged.Metadata["sourceCount"].(int) != 2 {
		t.Error("Expected sourceCount to be 2")
	}
}

func TestMergeConfigsEmpty(t *testing.T) {
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}

	// Test with no configs
	_, err = e.MergeConfigs()
	if err == nil {
		t.Error("Expected error when merging zero configs")
	}

	// Test with nil configs
	merged, err := e.MergeConfigs(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Servers) != 0 {
		t.Error("Expected no servers when merging nil configs")
	}
}