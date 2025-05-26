package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SyncManager handles synchronization operations
type SyncManager struct {
	engine *engineImpl
}

// NewSyncManager creates a new sync manager
func NewSyncManager(engine *engineImpl) *SyncManager {
	return &SyncManager{engine: engine}
}

// SyncToTarget synchronizes configuration to a specific target
func (sm *SyncManager) SyncToTarget(ctx context.Context, targetName string, options SyncOptions) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{
		Target:     targetName,
		Success:    false,
		Changes:    []Change{},
		Errors:     []SyncError{},
		ConfigPath: "",
		Duration:   0,
	}

	// Get target configuration
	target, err := sm.engine.GetTarget(targetName)
	if err != nil {
		return result, fmt.Errorf("failed to get target %s: %w", targetName, err)
	}

	if !target.Enabled && !options.Force {
		return result, fmt.Errorf("target %s is disabled", targetName)
	}

	// Generate target-specific configuration
	config, err := sm.generateTargetConfig(targetName, target)
	if err != nil {
		return result, fmt.Errorf("failed to generate config: %w", err)
	}

	// Expand config path
	configPath := expandPath(target.ConfigPath)
	result.ConfigPath = configPath

	// Create backup if requested
	if options.BackupFirst && !options.DryRun {
		backupPath, err := sm.createBackup(configPath)
		if err != nil {
			// Log but don't fail
			result.Errors = append(result.Errors, SyncError{
				Error:       fmt.Sprintf("backup failed: %v", err),
				Recoverable: true,
			})
		} else {
			result.BackupPath = backupPath
		}
	}

	// Get existing config for comparison
	var existingConfig interface{}
	existingData, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(existingData, &existingConfig)
	}

	// Calculate changes
	changes := sm.calculateChanges(existingConfig, config, targetName)
	result.Changes = changes

	// Apply changes if not dry run
	if !options.DryRun {
		if err := sm.writeTargetConfig(configPath, config); err != nil {
			result.Errors = append(result.Errors, SyncError{
				Error:       fmt.Sprintf("write failed: %v", err),
				Recoverable: false,
			})
			return result, err
		}
	}

	result.Success = true
	result.Duration = time.Since(start)

	return result, nil
}

// SyncToAllTargets synchronizes to all enabled targets
func (sm *SyncManager) SyncToAllTargets(ctx context.Context, options SyncOptions) (*MultiSyncResult, error) {
	start := time.Now()
	targets, err := sm.engine.ListTargets()
	if err != nil {
		return nil, err
	}

	result := &MultiSyncResult{
		Results:       []SyncResult{},
		TotalDuration: 0,
		SuccessCount:  0,
		FailureCount:  0,
	}

	// Use worker pool for parallel sync
	var wg sync.WaitGroup
	resultChan := make(chan SyncResult, len(targets))
	semaphore := make(chan struct{}, 5) // Max 5 concurrent syncs

	for _, target := range targets {
		if !target.Enabled && !options.Force {
			continue
		}

		wg.Add(1)
		go func(t *TargetInfo) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check context cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Sync to target
			syncResult, err := sm.SyncToTarget(ctx, t.Name, options)
			if err != nil {
				syncResult = &SyncResult{
					Target:  t.Name,
					Success: false,
					Errors: []SyncError{{
						Error:       err.Error(),
						Recoverable: false,
					}},
				}
			}

			resultChan <- *syncResult
		}(target)
	}

	// Wait for all syncs to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for syncResult := range resultChan {
		result.Results = append(result.Results, syncResult)
		if syncResult.Success {
			result.SuccessCount++
		} else {
			result.FailureCount++
		}
	}

	result.TotalDuration = time.Since(start)
	return result, nil
}

// generateTargetConfig generates configuration for a specific target
func (sm *SyncManager) generateTargetConfig(targetName string, target *TargetConfig) (interface{}, error) {
	switch target.ConfigFormat {
	case "flat":
		return sm.generateFlatConfig(targetName, target)
	case "nested":
		return sm.generateNestedConfig(targetName, target)
	case "project-nested":
		return sm.generateProjectNestedConfig(targetName, target)
	default:
		return nil, fmt.Errorf("unknown config format: %s", target.ConfigFormat)
	}
}

// generateFlatConfig generates flat configuration (VS Code, Cursor style)
func (sm *SyncManager) generateFlatConfig(targetName string, target *TargetConfig) (map[string]interface{}, error) {
	sm.engine.mu.RLock()
	defer sm.engine.mu.RUnlock()

	config := make(map[string]interface{})
	mcpServers := make(map[string]ServerConfig)

	for name, server := range sm.engine.config.Servers {
		// Skip disabled servers unless including them
		if !server.Internal.Enabled {
			continue
		}

		// Check if should sync to this target
		if !sm.engine.shouldSyncToTarget(server, targetName) {
			continue
		}

		// Apply name sanitization if needed
		finalName := name
		if target.RequiresSanitization {
			finalName = SanitizeServerName(name)
		}

		// Add server config (without internal metadata)
		mcpServers[finalName] = server.ServerConfig
	}

	config["mcpServers"] = mcpServers
	return config, nil
}

// generateNestedConfig generates nested configuration
func (sm *SyncManager) generateNestedConfig(targetName string, target *TargetConfig) (map[string]interface{}, error) {
	// Similar to flat but with different structure
	return sm.generateFlatConfig(targetName, target)
}

// generateProjectNestedConfig generates Claude Code style configuration
func (sm *SyncManager) generateProjectNestedConfig(targetName string, target *TargetConfig) (map[string]interface{}, error) {
	sm.engine.mu.RLock()
	defer sm.engine.mu.RUnlock()

	// Read existing config to preserve non-MCP settings
	configPath := expandPath(target.ConfigPath)
	var existingConfig map[string]interface{}

	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &existingConfig)
	} else {
		existingConfig = make(map[string]interface{})
	}

	// Generate global MCP servers
	mcpServers := make(map[string]ServerConfig)

	for name, server := range sm.engine.config.Servers {
		// Skip project-specific servers in global section
		if server.Internal.ProjectSpecific {
			continue
		}

		// Skip disabled servers
		if !server.Internal.Enabled {
			continue
		}

		// Check if should sync to this target
		if !sm.engine.shouldSyncToTarget(server, targetName) {
			continue
		}

		// Apply name sanitization
		finalName := name
		if target.RequiresSanitization {
			finalName = SanitizeServerName(name)
		}

		mcpServers[finalName] = server.ServerConfig
	}

	existingConfig["mcpServers"] = mcpServers

	// Handle project-specific servers
	if projects, ok := existingConfig["projects"].(map[string]interface{}); ok {
		// Update existing projects
		for projectPath, projectData := range projects {
			if project, ok := projectData.(map[string]interface{}); ok {
				// Add project-specific servers
				projectServers := make(map[string]ServerConfig)

				for name, server := range sm.engine.config.Servers {
					if !server.Internal.ProjectSpecific {
						continue
					}

					// Check if this server belongs to this project
					if server.Internal.ProjectPath != projectPath {
						continue
					}

					// Apply sanitization
					finalName := name
					if target.RequiresSanitization {
						finalName = SanitizeServerName(name)
					}

					projectServers[finalName] = server.ServerConfig
				}

				if len(projectServers) > 0 {
					project["mcpServers"] = projectServers
				}
			}
		}
	}

	return existingConfig, nil
}

// calculateChanges calculates what changes will be made
func (sm *SyncManager) calculateChanges(existing, new interface{}, targetName string) []Change {
	changes := []Change{}

	// Simple comparison for now
	// TODO: Implement deep comparison

	existingJSON, _ := json.Marshal(existing)
	newJSON, _ := json.Marshal(new)

	if string(existingJSON) != string(newJSON) {
		changes = append(changes, Change{
			Type:   "update",
			Server: "all", // TODO: Be more specific
		})
	}

	return changes
}

// writeTargetConfig writes configuration to target file
func (sm *SyncManager) writeTargetConfig(path string, config interface{}) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal config
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// createBackup creates a backup of the target config
func (sm *SyncManager) createBackup(configPath string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil // No backup needed
	}

	// Read existing content
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	// Create backup path
	backupDir := filepath.Join(filepath.Dir(configPath), ".backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s.%s.backup",
		filepath.Base(configPath), timestamp))

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}

	return backupPath, nil
}

// PreviewSync previews what changes would be made
func (sm *SyncManager) PreviewSync(targetName string) (*SyncPreview, error) {
	// Get target
	target, err := sm.engine.GetTarget(targetName)
	if err != nil {
		return nil, err
	}

	// Generate config
	config, err := sm.generateTargetConfig(targetName, target)
	if err != nil {
		return nil, err
	}

	// Get existing config
	configPath := expandPath(target.ConfigPath)
	var existingConfig interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &existingConfig)
	}

	// Calculate changes
	changes := sm.calculateChanges(existingConfig, config, targetName)

	preview := &SyncPreview{
		Destination:    targetName,
		Changes:        changes,
		EstimatedTime:  100 * time.Millisecond, // Rough estimate
		RequiresBackup: len(changes) > 0,
	}

	return preview, nil
}

// expandPath is defined in generic_destinations.go
