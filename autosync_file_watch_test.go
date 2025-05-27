package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAutoSyncFileWatchingMCPConfig tests that auto-sync detects changes to the MCP config file
func TestAutoSyncFileWatchingMCPConfig(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "engine-test-autosync-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create MCP config file path
	mcpConfigPath := filepath.Join(tmpDir, "mcp.json")

	// Create initial MCP config
	initialConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"test-server": map[string]interface{}{
				"serverName": "test-server",
				"type":       "stdio",
				"command":    "python",
				"args":       []string{"-m", "test"},
				"enabled":    true,
			},
		},
	}

	// Write initial config
	initialData, err := json.MarshalIndent(initialConfig, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mcpConfigPath, initialData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create engine with file storage pointing to our config
	engine, err := NewEngine(WithFileStorage(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	// Load config from file
	if err := engine.LoadConfig(mcpConfigPath); err != nil {
		t.Fatal(err)
	}

	// Register a test destination
	syncCount := 0
	syncChan := make(chan bool, 10)
	testDest := &fileWatchTestDestination{
		id:   "test-dest",
		path: filepath.Join(tmpDir, "test-dest.json"),
		onSync: func() {
			syncCount++
			select {
			case syncChan <- true:
			default:
			}
		},
	}
	if err := engine.RegisterDestination("test-dest", testDest); err != nil {
		t.Fatal(err)
	}

	// Start auto-sync with aggressive settings for testing
	autoSyncConfig := AutoSyncConfig{
		Enabled:         true,
		WatchInterval:   50 * time.Millisecond,  // Check every 50ms
		DebounceDelay:   100 * time.Millisecond, // Debounce 100ms
		TargetWhitelist: []string{"test-dest"},
	}

	if err := engine.StartAutoSync(autoSyncConfig); err != nil {
		t.Fatal(err)
	}
	defer engine.StopAutoSync()

	// Wait a bit for file watching to start
	time.Sleep(200 * time.Millisecond)

	// Verify auto-sync is running
	status, err := engine.GetAutoSyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Running {
		t.Fatal("Expected auto-sync to be running")
	}

	// Reset sync count
	syncCount = 0

	// Test 1: Modify the MCP config file
	t.Run("detect file modification", func(t *testing.T) {
		// Add a new server to the config
		modifiedConfig := map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"test-server": map[string]interface{}{
					"serverName": "test-server",
					"type":       "stdio",
					"command":    "python",
					"args":       []string{"-m", "test"},
					"enabled":    true,
				},
				"test-server-2": map[string]interface{}{
					"serverName": "test-server-2",
					"type":       "stdio",
					"command":    "node",
					"args":       []string{"test.js"},
					"enabled":    false,
				},
			},
		}

		modifiedData, err := json.MarshalIndent(modifiedConfig, "", "  ")
		if err != nil {
			t.Fatal(err)
		}

		// Write the modified config
		if err := os.WriteFile(mcpConfigPath, modifiedData, 0644); err != nil {
			t.Fatal(err)
		}

		// Wait for sync to happen
		select {
		case <-syncChan:
			t.Logf("Sync triggered after file modification")
		case <-time.After(3 * time.Second):
			t.Error("Timed out waiting for sync after file modification")
		}
	})

	// Test 2: Multiple rapid changes should be debounced
	t.Run("debounce rapid changes", func(t *testing.T) {
		preSyncCount := syncCount

		// Make 3 rapid changes
		for i := 0; i < 3; i++ {
			modifiedConfig := map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"test-server": map[string]interface{}{
						"serverName": "test-server",
						"type":       "stdio",
						"command":    "python",
						"args":       []string{"-m", "test", "--version", string(rune('0' + i))},
						"enabled":    true,
					},
				},
			}

			modifiedData, err := json.MarshalIndent(modifiedConfig, "", "  ")
			if err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(mcpConfigPath, modifiedData, 0644); err != nil {
				t.Fatal(err)
			}
			time.Sleep(30 * time.Millisecond) // Less than debounce delay
		}

		// Wait for debounced sync
		select {
		case <-syncChan:
			t.Logf("Sync triggered after rapid changes")
		case <-time.After(3 * time.Second):
			t.Error("Timed out waiting for debounced sync")
		}

		// Should only sync once due to debouncing
		postSyncCount := syncCount
		if postSyncCount-preSyncCount > 2 {
			t.Errorf("Expected at most 2 syncs due to debouncing, got %d", postSyncCount-preSyncCount)
		}
	})

	// Test 3: File deletion and recreation
	t.Run("detect file deletion and recreation", func(t *testing.T) {
		// Delete the file
		if err := os.Remove(mcpConfigPath); err != nil {
			t.Fatal(err)
		}

		// Wait a bit
		time.Sleep(200 * time.Millisecond)

		// Recreate the file
		newConfig := map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"new-server": map[string]interface{}{
					"serverName": "new-server",
					"type":       "sse",
					"url":        "http://example.com",
					"enabled":    true,
				},
			},
		}

		newData, err := json.MarshalIndent(newConfig, "", "  ")
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(mcpConfigPath, newData, 0644); err != nil {
			t.Fatal(err)
		}

		// Wait for sync
		select {
		case <-syncChan:
			t.Logf("Sync triggered after file recreation")
		case <-time.After(3 * time.Second):
			t.Error("Timed out waiting for sync after file recreation")
		}
	})
}

// fileWatchTestDestination is a test destination that tracks syncs
type fileWatchTestDestination struct {
	id     string
	path   string
	onSync func()
	data   []byte
}

func (d *fileWatchTestDestination) GetID() string {
	return d.id
}

func (d *fileWatchTestDestination) GetDescription() string {
	return "File watch test destination"
}

func (d *fileWatchTestDestination) Transform(config *Config) (interface{}, error) {
	return map[string]interface{}{
		"mcpServers": config.Servers,
	}, nil
}

func (d *fileWatchTestDestination) Read() ([]byte, error) {
	if d.data != nil {
		return d.data, nil
	}
	return os.ReadFile(d.path)
}

func (d *fileWatchTestDestination) Write(data []byte) error {
	d.data = data
	if d.onSync != nil {
		d.onSync()
	}
	return os.WriteFile(d.path, data, 0644)
}

func (d *fileWatchTestDestination) Exists() bool {
	_, err := os.Stat(d.path)
	return err == nil || d.data != nil
}

func (d *fileWatchTestDestination) SupportsBackup() bool {
	return true
}

func (d *fileWatchTestDestination) Backup() (string, error) {
	backupPath := d.path + ".backup"
	data, err := d.Read()
	if err != nil {
		return "", err
	}
	return backupPath, os.WriteFile(backupPath, data, 0644)
}