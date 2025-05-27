package engine

import (
	"testing"
	"time"
)

// TestAutoSyncPersistence tests that auto-sync state is properly persisted and restored
func TestAutoSyncPersistence(t *testing.T) {
	// Create first engine instance with memory storage
	storage := NewMemoryStorage()
	engine1, err := NewEngine(WithStorage(storage))
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Start auto-sync
	config := AutoSyncConfig{
		Enabled:         true,
		WatchInterval:   2 * time.Second,
		DebounceDelay:   1 * time.Second,
		TargetWhitelist: []string{"claude", "cursor"},
	}

	err = engine1.StartAutoSync(config)
	if err != nil {
		t.Fatalf("Failed to start auto-sync: %v", err)
	}

	// Verify auto-sync is running
	status, err := engine1.GetAutoSyncStatus()
	if err != nil {
		t.Fatalf("Failed to get auto-sync status: %v", err)
	}

	if !status.Running {
		t.Error("Auto-sync should be running")
	}
	if !status.Enabled {
		t.Error("Auto-sync should be enabled")
	}
	if status.WatchInterval != 2*time.Second {
		t.Errorf("Expected watch interval of 2s, got %v", status.WatchInterval)
	}

	// Stop auto-sync to simulate shutdown
	err = engine1.StopAutoSync()
	if err != nil {
		t.Fatalf("Failed to stop auto-sync: %v", err)
	}

	// Create second engine instance with same storage (simulating restart)
	engine2, err := NewEngine(WithStorage(storage))
	if err != nil {
		t.Fatalf("Failed to create second engine: %v", err)
	}

	// Give auto-start time to execute
	time.Sleep(200 * time.Millisecond)

	// Verify auto-sync auto-started
	status2, err := engine2.GetAutoSyncStatus()
	if err != nil {
		t.Fatalf("Failed to get auto-sync status from second engine: %v", err)
	}

	if !status2.Running {
		t.Error("Auto-sync should have auto-started")
	}
	if !status2.Enabled {
		t.Error("Auto-sync should still be enabled")
	}
	if status2.WatchInterval != 2*time.Second {
		t.Errorf("Expected persisted watch interval of 2s, got %v", status2.WatchInterval)
	}

	// Clean up
	engine2.StopAutoSync()
}

// TestAutoSyncDisablePersistence tests that disabled state is persisted
func TestAutoSyncDisablePersistence(t *testing.T) {
	storage := NewMemoryStorage()
	
	// First, create engine with auto-sync enabled
	engine1, err := NewEngine(WithStorage(storage))
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Start and then stop auto-sync
	config := AutoSyncConfig{
		Enabled:       true,
		WatchInterval: 1 * time.Second,
		DebounceDelay: 500 * time.Millisecond,
	}

	err = engine1.StartAutoSync(config)
	if err != nil {
		t.Fatalf("Failed to start auto-sync: %v", err)
	}

	err = engine1.StopAutoSync()
	if err != nil {
		t.Fatalf("Failed to stop auto-sync: %v", err)
	}

	// Create new engine instance
	engine2, err := NewEngine(WithStorage(storage))
	if err != nil {
		t.Fatalf("Failed to create second engine: %v", err)
	}

	// Give enough time for auto-start to execute if it was going to
	time.Sleep(200 * time.Millisecond)

	// Verify auto-sync did not auto-start
	status, err := engine2.GetAutoSyncStatus()
	if err != nil {
		t.Fatalf("Failed to get auto-sync status: %v", err)
	}

	if status.Running {
		t.Error("Auto-sync should not have auto-started when disabled")
	}
	if status.Enabled {
		t.Error("Auto-sync should be disabled")
	}
}