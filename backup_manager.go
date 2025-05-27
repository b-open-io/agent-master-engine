package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

// Backup Management functionality for engineImpl

// CreateBackup creates a backup of the current configuration
func (e *engineImpl) CreateBackup(description string) (*BackupInfo, error) {
	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	// Generate backup ID with timestamp and nanoseconds for uniqueness
	timestamp := time.Now()
	backupID := fmt.Sprintf("backup-%s-%d", timestamp.Format("20060102-150405"), timestamp.UnixNano())
	
	// Determine backup path
	backupPath := filepath.Join("backups", backupID+".json")
	
	// Marshal config
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create backup info
	info := &BackupInfo{
		ID:          backupID,
		Path:        backupPath,
		Timestamp:   timestamp,
		Size:        int64(len(data)),
		Type:        "manual",
		Description: description,
	}

	// Store backup using storage layer
	backupKey := fmt.Sprintf("backups/%s", backupID)
	if err := e.storage.Write(backupKey, data); err != nil {
		return nil, fmt.Errorf("failed to write backup: %w", err)
	}

	// Store backup metadata
	metadataKey := fmt.Sprintf("backup-meta/%s", backupID)
	metaData, _ := json.Marshal(info)
	if err := e.storage.Write(metadataKey, metaData); err != nil {
		// Try to clean up the backup
		e.storage.Delete(backupKey)
		return nil, fmt.Errorf("failed to write backup metadata: %w", err)
	}

	// Clean up old backups if needed
	if e.config.Settings.Backup.MaxBackups > 0 {
		if err := e.cleanupOldBackups(e.config.Settings.Backup.MaxBackups); err != nil {
			// Log but don't fail the backup
			fmt.Printf("Warning: failed to cleanup old backups: %v\n", err)
		}
	}

	// Emit event
	e.eventBus.emit(EventBackupCreated, *info)

	return info, nil
}

// ListBackups returns a list of all backups
func (e *engineImpl) ListBackups() ([]*BackupInfo, error) {
	// List all backup metadata keys
	metaKeys, err := e.storage.List("backup-meta/")
	if err != nil {
		return nil, fmt.Errorf("failed to list backup metadata: %w", err)
	}

	backups := make([]*BackupInfo, 0, len(metaKeys))
	
	for _, key := range metaKeys {
		// Read metadata
		data, err := e.storage.Read(key)
		if err != nil {
			continue // Skip invalid metadata
		}

		var info BackupInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue // Skip invalid metadata
		}

		backups = append(backups, &info)
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// RestoreBackup restores configuration from a backup
func (e *engineImpl) RestoreBackup(backupID string) error {
	// Read backup data
	backupKey := fmt.Sprintf("backups/%s", backupID)
	data, err := e.storage.Read(backupKey)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Parse backup config
	var backupConfig Config
	if err := json.Unmarshal(data, &backupConfig); err != nil {
		return fmt.Errorf("failed to parse backup: %w", err)
	}

	// Create a backup of current config before restoring (safety)
	if _, err := e.CreateBackup("pre-restore-backup"); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to create pre-restore backup: %v\n", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Restore the config
	e.config = &backupConfig

	// Save to storage
	if err := e.saveConfigNoLock(); err != nil {
		return fmt.Errorf("failed to save restored config: %w", err)
	}

	// Emit event
	e.eventBus.emit(EventBackupRestored, BackupInfo{
		ID:        backupID,
		Timestamp: time.Now(),
		Type:      "restore",
	})

	return nil
}

// cleanupOldBackups removes old backups keeping only the most recent maxBackups
func (e *engineImpl) cleanupOldBackups(maxBackups int) error {
	backups, err := e.ListBackups()
	if err != nil {
		return err
	}

	// If we have more backups than allowed
	if len(backups) > maxBackups {
		// Backups are already sorted newest first
		toDelete := backups[maxBackups:]
		
		for _, backup := range toDelete {
			// Delete backup data
			backupKey := fmt.Sprintf("backups/%s", backup.ID)
			e.storage.Delete(backupKey)
			
			// Delete metadata
			metaKey := fmt.Sprintf("backup-meta/%s", backup.ID)
			e.storage.Delete(metaKey)
		}
	}
	
	return nil
}