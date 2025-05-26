package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAutoSync(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "engine-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create engine with file storage
	engine, err := NewEngine(WithFileStorage(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	// Create a test config
	config := &Config{
		Version: "1.0.0",
		Servers: map[string]ServerWithMetadata{
			"test-server": {
				ServerConfig: ServerConfig{
					Transport: "stdio",
					Command:   "test-command",
				},
				Internal: InternalMetadata{
					Enabled:      true,
					LastModified: time.Now(),
				},
			},
		},
	}

	// Set config
	if err := engine.SetConfig(config); err != nil {
		t.Fatal(err)
	}

	// Register a test destination
	testDest := &testDestination{
		id:       "test-dest",
		path:     filepath.Join(tmpDir, "test-dest.json"),
		synced:   make(chan bool, 1),
	}
	if err := engine.RegisterDestination("test-dest", testDest); err != nil {
		t.Fatal(err)
	}

	// Start auto-sync
	autoSyncConfig := AutoSyncConfig{
		Enabled:         true,
		WatchInterval:   100 * time.Millisecond,
		DebounceDelay:   50 * time.Millisecond,
		TargetWhitelist: []string{"test-dest"},
	}

	if err := engine.StartAutoSync(autoSyncConfig); err != nil {
		t.Fatal(err)
	}
	defer engine.StopAutoSync()

	// Get status
	status, err := engine.GetAutoSyncStatus()
	if err != nil {
		t.Fatal(err)
	}

	if !status.Running {
		t.Error("Expected auto-sync to be running")
	}

	// Make a change to trigger sync
	config.Servers["test-server2"] = ServerWithMetadata{
		ServerConfig: ServerConfig{
			Transport: "sse",
			URL:       "http://test.com",
		},
		Internal: InternalMetadata{
			Enabled:      true,
			LastModified: time.Now(),
		},
	}

	if err := engine.SetConfig(config); err != nil {
		t.Fatal(err)
	}

	// Wait for sync to happen
	select {
	case <-testDest.synced:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for sync")
	}

	// Stop auto-sync
	if err := engine.StopAutoSync(); err != nil {
		t.Fatal(err)
	}

	// Verify stopped
	status, err = engine.GetAutoSyncStatus()
	if err != nil {
		t.Fatal(err)
	}

	if status.Running {
		t.Error("Expected auto-sync to be stopped")
	}
}

// testDestination is a mock destination for testing
type testDestination struct {
	mu     sync.RWMutex
	id     string
	path   string
	synced chan bool
	data   []byte
}

func (td *testDestination) GetID() string {
	td.mu.RLock()
	defer td.mu.RUnlock()
	return td.id
}

func (td *testDestination) GetDescription() string {
	return "Test destination"
}

func (td *testDestination) Transform(config *Config) (interface{}, error) {
	// Simple transform - just return servers
	return map[string]interface{}{
		"mcpServers": config.Servers,
	}, nil
}

func (td *testDestination) Read() ([]byte, error) {
	td.mu.RLock()
	defer td.mu.RUnlock()
	if td.data != nil {
		return td.data, nil
	}
	return os.ReadFile(td.path)
}

func (td *testDestination) Write(data []byte) error {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.data = data
	// Signal that sync happened
	select {
	case td.synced <- true:
	default:
	}
	return os.WriteFile(td.path, data, 0644)
}

func (td *testDestination) Exists() bool {
	td.mu.RLock()
	defer td.mu.RUnlock()
	_, err := os.Stat(td.path)
	return err == nil || td.data != nil
}

func (td *testDestination) SupportsBackup() bool {
	return true
}

func (td *testDestination) Backup() (string, error) {
	td.mu.RLock()
	backupPath := td.path + ".backup"
	td.mu.RUnlock()
	
	data, err := td.Read()
	if err != nil {
		return "", err
	}
	return backupPath, os.WriteFile(backupPath, data, 0644)
}

func TestAutoSyncWithFileChanges(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "engine-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config file
	configPath := filepath.Join(tmpDir, "config.json")
	configData := []byte(`{
		"version": "1.0.0",
		"servers": {
			"test-server": {
				"transport": "stdio",
				"command": "test-command",
				"internal": {
					"enabled": true
				}
			}
		}
	}`)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create engine
	engine, err := NewEngine(WithFileStorage(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	// Load config from file
	if err := engine.LoadConfig(configPath); err != nil {
		t.Fatal(err)
	}

	// Register destination
	testDest := &testDestination{
		id:       "test-dest",
		path:     filepath.Join(tmpDir, "test-dest.json"),
		synced:   make(chan bool, 1),
	}
	if err := engine.RegisterDestination("test-dest", testDest); err != nil {
		t.Fatal(err)
	}

	// Start auto-sync
	autoSyncConfig := AutoSyncConfig{
		Enabled:         true,
		WatchInterval:   100 * time.Millisecond,
		DebounceDelay:   50 * time.Millisecond,
		TargetWhitelist: []string{"test-dest"},
		IgnorePatterns:  []string{"*.backup"},
	}

	if err := engine.StartAutoSync(autoSyncConfig); err != nil {
		t.Fatal(err)
	}
	defer engine.StopAutoSync()

	// Wait a bit for watcher to be ready
	time.Sleep(200 * time.Millisecond)

	// Modify config file
	newConfigData := []byte(`{
		"version": "1.0.0",
		"servers": {
			"test-server": {
				"transport": "stdio",
				"command": "test-command",
				"internal": {
					"enabled": true
				}
			},
			"new-server": {
				"transport": "sse",
				"url": "http://example.com",
				"internal": {
					"enabled": true
				}
			}
		}
	}`)

	if err := os.WriteFile(configPath, newConfigData, 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for sync
	select {
	case <-testDest.synced:
		// Success - verify the synced data contains new server
		testDest.mu.RLock()
		hasData := testDest.data != nil
		testDest.mu.RUnlock()
		if !hasData {
			t.Error("No data synced")
		}
		// Could add more validation here
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for file change sync")
	}
}

func TestAutoSyncDebouncing(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "engine-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create engine
	engine, err := NewEngine(WithFileStorage(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	// Set initial config before starting auto-sync to avoid initial sync
	config := &Config{
		Version: "1.0.0",
		Servers: make(map[string]ServerWithMetadata),
	}
	if err := engine.SetConfig(config); err != nil {
		t.Fatal(err)
	}

	// Register destination that counts syncs
	syncCount := 0
	syncTimes := []time.Time{}
	testDest := &countingDestination{
		testDestination: testDestination{
			id:   "count-dest",
			path: filepath.Join(tmpDir, "count-dest.json"),
		},
		syncCount: &syncCount,
		syncTimes: &syncTimes,
	}
	if err := engine.RegisterDestination("count-dest", testDest); err != nil {
		t.Fatal(err)
	}

	// Start auto-sync with longer intervals to avoid periodic syncs during test
	autoSyncConfig := AutoSyncConfig{
		Enabled:         true,
		WatchInterval:   10 * time.Second, // Much longer than test duration
		DebounceDelay:   200 * time.Millisecond,
		TargetWhitelist: []string{"count-dest"},
	}

	if err := engine.StartAutoSync(autoSyncConfig); err != nil {
		t.Fatal(err)
	}
	defer engine.StopAutoSync()

	// Wait a bit for auto-sync to initialize
	time.Sleep(100 * time.Millisecond)

	// Reset sync count after any initialization syncs
	syncCount = 0
	syncTimes = []time.Time{}

	// Make multiple rapid changes
	startTime := time.Now()
	for i := 0; i < 5; i++ {
		serverName := fmt.Sprintf("server-%d", i)
		config.Servers[serverName] = ServerWithMetadata{
			ServerConfig: ServerConfig{
				Transport: "stdio",
				Command:   fmt.Sprintf("cmd-%d", i),
			},
		}
		if err := engine.SetConfig(config); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond) // Less than debounce delay
	}

	// Wait for debounced sync (debounce delay + some buffer)
	time.Sleep(300 * time.Millisecond)

	// Should only have synced once due to debouncing
	testDest.mu.Lock()
	finalCount := syncCount
	finalTimes := make([]time.Time, len(syncTimes))
	copy(finalTimes, syncTimes)
	testDest.mu.Unlock()
	
	if finalCount != 1 {
		t.Errorf("Expected 1 sync due to debouncing, got %d", finalCount)
		if len(finalTimes) > 0 {
			for i, st := range finalTimes {
				t.Logf("Sync %d occurred at: %v (%.2f ms after start)", 
					i+1, st, st.Sub(startTime).Seconds()*1000)
			}
		}
	}
}

// countingDestination counts how many times Write is called
type countingDestination struct {
	testDestination
	mu        sync.Mutex
	syncCount *int
	syncTimes *[]time.Time
}

func (cd *countingDestination) Write(data []byte) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	*cd.syncCount++
	if cd.syncTimes != nil {
		*cd.syncTimes = append(*cd.syncTimes, time.Now())
	}
	return cd.testDestination.Write(data)
}

func TestAutoSyncBlacklistWhitelist(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "engine-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create engine
	engine, err := NewEngine(WithFileStorage(tmpDir))
	if err != nil {
		t.Fatal(err)
	}

	// Create thread-safe sync flags
	type syncFlag struct {
		mu     sync.Mutex
		synced bool
	}
	
	dest1Flag := &syncFlag{}
	dest2Flag := &syncFlag{}
	dest3Flag := &syncFlag{}

	dest1 := &syncFlagDestination{
		testDestination: testDestination{
			id:   "dest1",
			path: filepath.Join(tmpDir, "dest1.json"),
		},
		synced: &dest1Flag.synced,
		flagMu: &dest1Flag.mu,
	}
	dest2 := &syncFlagDestination{
		testDestination: testDestination{
			id:   "dest2",
			path: filepath.Join(tmpDir, "dest2.json"),
		},
		synced: &dest2Flag.synced,
		flagMu: &dest2Flag.mu,
	}
	dest3 := &syncFlagDestination{
		testDestination: testDestination{
			id:   "dest3",
			path: filepath.Join(tmpDir, "dest3.json"),
		},
		synced: &dest3Flag.synced,
		flagMu: &dest3Flag.mu,
	}

	engine.RegisterDestination("dest1", dest1)
	engine.RegisterDestination("dest2", dest2)
	engine.RegisterDestination("dest3", dest3)

	// Test whitelist - only sync to dest1 and dest2
	autoSyncConfig := AutoSyncConfig{
		Enabled:         true,
		WatchInterval:   100 * time.Millisecond,
		DebounceDelay:   50 * time.Millisecond,
		TargetWhitelist: []string{"dest1", "dest2"},
	}

	if err := engine.StartAutoSync(autoSyncConfig); err != nil {
		t.Fatal(err)
	}

	// Trigger sync
	config := &Config{Version: "1.0.0", Servers: make(map[string]ServerWithMetadata)}
	engine.SetConfig(config)

	time.Sleep(300 * time.Millisecond)
	engine.StopAutoSync()

	// Check flags with proper synchronization
	dest1Flag.mu.Lock()
	dest1Synced := dest1Flag.synced
	dest1Flag.mu.Unlock()
	
	dest2Flag.mu.Lock()
	dest2Synced := dest2Flag.synced
	dest2Flag.mu.Unlock()
	
	dest3Flag.mu.Lock()
	dest3Synced := dest3Flag.synced
	dest3Flag.mu.Unlock()

	if !dest1Synced {
		t.Error("dest1 should have been synced (in whitelist)")
	}
	if !dest2Synced {
		t.Error("dest2 should have been synced (in whitelist)")
	}
	if dest3Synced {
		t.Error("dest3 should not have been synced (not in whitelist)")
	}

	// Reset flags with proper synchronization
	dest1Flag.mu.Lock()
	dest1Flag.synced = false
	dest1Flag.mu.Unlock()
	
	dest2Flag.mu.Lock()
	dest2Flag.synced = false
	dest2Flag.mu.Unlock()
	
	dest3Flag.mu.Lock()
	dest3Flag.synced = false
	dest3Flag.mu.Unlock()

	// Test blacklist - sync to all except dest2
	autoSyncConfig = AutoSyncConfig{
		Enabled:         true,
		WatchInterval:   100 * time.Millisecond,
		DebounceDelay:   50 * time.Millisecond,
		TargetBlacklist: []string{"dest2"},
	}

	if err := engine.StartAutoSync(autoSyncConfig); err != nil {
		t.Fatal(err)
	}

	// Trigger sync
	config.Servers["new"] = ServerWithMetadata{ServerConfig: ServerConfig{Transport: "stdio", Command: "cmd"}}
	engine.SetConfig(config)

	time.Sleep(300 * time.Millisecond)
	engine.StopAutoSync()

	// Check flags with proper synchronization
	dest1Flag.mu.Lock()
	dest1Synced = dest1Flag.synced
	dest1Flag.mu.Unlock()
	
	dest2Flag.mu.Lock()
	dest2Synced = dest2Flag.synced
	dest2Flag.mu.Unlock()
	
	dest3Flag.mu.Lock()
	dest3Synced = dest3Flag.synced
	dest3Flag.mu.Unlock()

	if !dest1Synced {
		t.Error("dest1 should have been synced (not in blacklist)")
	}
	if dest2Synced {
		t.Error("dest2 should not have been synced (in blacklist)")
	}
	if !dest3Synced {
		t.Error("dest3 should have been synced (not in blacklist)")
	}
}

// syncFlagDestination sets a flag when Write is called
type syncFlagDestination struct {
	testDestination
	flagMu *sync.Mutex
	synced *bool
}

func (sfd *syncFlagDestination) Write(data []byte) error {
	sfd.flagMu.Lock()
	*sfd.synced = true
	sfd.flagMu.Unlock()
	return sfd.testDestination.Write(data)
}