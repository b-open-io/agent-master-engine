package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAutoSyncAgentMasterScenario(t *testing.T) {
	// Create a temp directory to simulate ~/.agent-master
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp.json")

	// Create initial config
	initialConfig := &Config{
		Version: "1.0.2",
		Servers: map[string]ServerWithMetadata{
			"test-server": {
				ServerConfig: ServerConfig{
					Command: "echo",
					Args:    []string{"hello"},
				},
				Internal: InternalMetadata{
					Enabled: true,
				},
			},
		},
	}

	// Write initial config
	data, err := json.MarshalIndent(initialConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create engine exactly like agent-master does
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Load config from specific path (like agent-master does)
	if err := engine.LoadConfig(configPath); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Force the engine to reload from our test file to ensure it's watching the right path
	eng := engine.(*engineImpl)
	t.Logf("Engine configPath: %s", eng.configPath)

	// Track sync events
	syncCompleted := make(chan bool, 1)

	// Create a mock destination that notifies on sync
	mockDest := &agentMasterTestDestination{
		id: "test-target",
		onSync: func() {
			select {
			case syncCompleted <- true:
			default:
			}
		},
	}

	// Register destination
	if err := engine.RegisterDestination("test-target", mockDest); err != nil {
		t.Fatalf("Failed to register destination: %v", err)
	}

	// Start auto-sync
	config := AutoSyncConfig{
		Enabled:         true,
		TargetWhitelist: []string{"test-target"},
		WatchInterval:   100 * time.Millisecond,
		DebounceDelay:   200 * time.Millisecond,
	}

	if err := engine.StartAutoSync(config); err != nil {
		t.Fatalf("Failed to start auto-sync: %v", err)
	}
	defer engine.StopAutoSync()

	// Wait a bit for file watching to initialize
	time.Sleep(500 * time.Millisecond)

	// Verify auto-sync is running
	status, err := engine.GetAutoSyncStatus()
	if err != nil {
		t.Fatalf("Failed to get auto-sync status: %v", err)
	}
	if !status.Running {
		t.Fatal("Auto-sync is not running")
	}
	t.Logf("Auto-sync status: %+v", status)

	// Modify the config file
	t.Log("Modifying config file...")
	modifiedConfig := &Config{
		Version: "1.0.2",
		Servers: map[string]ServerWithMetadata{
			"test-server": {
				ServerConfig: ServerConfig{
					Command: "echo",
					Args:    []string{"hello", "world"}, // Changed
				},
				Internal: InternalMetadata{
					Enabled: true,
				},
			},
			"new-server": { // Added
				ServerConfig: ServerConfig{
					Command: "ls",
					Args:    []string{"-la"},
				},
				Internal: InternalMetadata{
					Enabled: true,
				},
			},
		},
	}

	data, err = json.MarshalIndent(modifiedConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal modified config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write modified config: %v", err)
	}

	// Wait for sync to complete
	select {
	case <-syncCompleted:
		t.Log("Sync completed after file change")
	case <-time.After(3 * time.Second):
		t.Fatal("Sync did not complete within timeout after file change")
	}

	// Verify the destination received the update
	if len(mockDest.data) == 0 {
		t.Fatal("Destination was not updated")
	}

	t.Logf("Destination data: %s", string(mockDest.data))

	var destConfig Config
	if err := json.Unmarshal(mockDest.data, &destConfig); err != nil {
		t.Fatalf("Failed to unmarshal destination data: %v", err)
	}

	// Verify the changes were synced
	if len(destConfig.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(destConfig.Servers))
	}

	if server, ok := destConfig.Servers["test-server"]; ok {
		if len(server.ServerConfig.Args) != 2 || server.ServerConfig.Args[1] != "world" {
			t.Errorf("test-server args not updated correctly: %v", server.ServerConfig.Args)
		}
	} else {
		t.Error("test-server not found in destination")
	}

	if _, ok := destConfig.Servers["new-server"]; !ok {
		t.Error("new-server not found in destination")
	}
}

// agentMasterTestDestination is a simple mock implementation
type agentMasterTestDestination struct {
	id     string
	data   []byte
	onSync func()
}

func (d *agentMasterTestDestination) GetID() string { return d.id }
func (d *agentMasterTestDestination) GetDescription() string { return "Test destination" }
func (d *agentMasterTestDestination) Transform(config *Config) (interface{}, error) {
	return config, nil
}
func (d *agentMasterTestDestination) Read() ([]byte, error) { return d.data, nil }
func (d *agentMasterTestDestination) Write(data []byte) error {
	d.data = make([]byte, len(data))
	copy(d.data, data)
	if d.onSync != nil {
		d.onSync()
	}
	return nil
}
func (d *agentMasterTestDestination) Exists() bool { return true }
func (d *agentMasterTestDestination) SupportsBackup() bool { return false }
func (d *agentMasterTestDestination) Backup() (string, error) { return "", nil }