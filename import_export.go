package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Import/Export functionality for engineImpl

// Export exports the configuration in the specified format
func (e *engineImpl) Export(format ExportFormat) ([]byte, error) {
	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	// Create export structure with only enabled servers
	exportConfig := &Config{
		Version:  config.Version,
		Servers:  make(map[string]ServerWithMetadata),
		Settings: config.Settings,
		Metadata: config.Metadata,
	}

	// Only export enabled servers
	for name, server := range config.Servers {
		if server.Internal.Enabled {
			exportConfig.Servers[name] = server
		}
	}

	switch format {
	case ExportFormatJSON:
		data, err := json.MarshalIndent(exportConfig, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %w", err)
		}
		return data, nil
		
	case ExportFormatYAML:
		// For now, we'll use JSON as YAML requires additional dependency
		// In production, you'd use gopkg.in/yaml.v3
		return nil, fmt.Errorf("YAML export not yet implemented")
		
	case ExportFormatTOML:
		// For now, we'll use JSON as TOML requires additional dependency
		// In production, you'd use github.com/BurntSushi/toml
		return nil, fmt.Errorf("TOML export not yet implemented")
		
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// ExportToFile exports the configuration to a file
func (e *engineImpl) ExportToFile(path string, format ExportFormat) error {
	// Export to bytes first
	data, err := e.Export(format)
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	// Expand path
	expandedPath := expandPath(path)

	// Create directory if needed
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(expandedPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Emit event
	e.eventBus.emit(EventConfigSaved, ConfigChange{
		Type:      "config-exported",
		Timestamp: time.Now(),
		Source:    "export",
		Details:   map[string]interface{}{"path": expandedPath, "format": string(format)},
	})

	return nil
}

// Import imports configuration from data
func (e *engineImpl) Import(data []byte, format ImportFormat, options ImportOptions) error {
	// Parse the data using our MCP parser
	config, err := ParseMCPConfigWithOptions(data, options.SubstituteEnvVars)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Handle merge options
	if options.MergeMode == "replace" {
		// Replace all servers
		e.config.Servers = make(map[string]ServerWithMetadata)
	}

	// Import servers
	importedCount := 0
	for name, server := range config.Servers {
		// Skip if server already exists and not overwriting
		if _, exists := e.config.Servers[name]; exists && !options.OverwriteExisting {
			continue
		}

		// Validate server if validator is set
		if e.validator != nil {
			if err := e.validator.ValidateServerConfig(name, server.ServerConfig); err != nil {
				if options.SkipInvalid {
					continue
				}
				return fmt.Errorf("invalid server %q: %w", name, err)
			}
		}

		// Add server with metadata
		e.config.Servers[name] = server
		importedCount++
	}

	// Save configuration
	if err := e.saveConfigNoLock(); err != nil {
		return fmt.Errorf("failed to save imported config: %w", err)
	}

	// Emit event
	e.eventBus.emit(EventConfigLoaded, ConfigChange{
		Type:      "config-imported",
		Timestamp: time.Now(),
		Source:    "import",
		Details:   map[string]interface{}{"imported_servers": importedCount},
	})

	return nil
}

// ImportFromTarget imports configuration from a specific target
func (e *engineImpl) ImportFromTarget(targetName string, options ImportOptions) (*ImportResult, error) {
	// TODO: Implement
	return nil, fmt.Errorf("not implemented")
}

// MergeConfigs merges multiple configurations into one
func (e *engineImpl) MergeConfigs(configs ...*Config) (*Config, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("no configs provided to merge")
	}

	// Start with an empty merged config
	merged := &Config{
		Version:  DefaultConfigVersion,
		Servers:  make(map[string]ServerWithMetadata),
		Settings: Settings{},
		Metadata: make(map[string]interface{}),
	}

	// Use the first non-nil config as base for settings
	for _, cfg := range configs {
		if cfg != nil && cfg.Version != "" {
			merged.Version = cfg.Version
			merged.Settings = cfg.Settings
			break
		}
	}

	// Merge servers from all configs
	serverSources := make(map[string]string) // Track which config each server came from
	
	for idx, cfg := range configs {
		if cfg == nil {
			continue
		}

		sourceName := fmt.Sprintf("config-%d", idx+1)
		
		// Merge servers
		for name, server := range cfg.Servers {
			if _, exists := merged.Servers[name]; exists {
				// Server exists - decide how to handle conflict
				// Option 1: Last one wins (current implementation)
				// Option 2: Could compare timestamps and keep newer
				// Option 3: Could merge server properties
				
				// For now: last one wins, but track the conflict
				if merged.Metadata["conflicts"] == nil {
					merged.Metadata["conflicts"] = make([]map[string]interface{}, 0)
				}
				
				conflicts := merged.Metadata["conflicts"].([]map[string]interface{})
				conflicts = append(conflicts, map[string]interface{}{
					"server":       name,
					"from":         serverSources[name],
					"overriddenBy": sourceName,
				})
				merged.Metadata["conflicts"] = conflicts
			}
			
			// Copy the server
			merged.Servers[name] = server
			serverSources[name] = sourceName
		}

		// Merge metadata
		for k, v := range cfg.Metadata {
			if k != "conflicts" { // Don't overwrite our conflict tracking
				merged.Metadata[k] = v
			}
		}
	}

	// Add merge info
	merged.Metadata["mergedAt"] = time.Now()
	merged.Metadata["sourceCount"] = len(configs)
	merged.Metadata["totalServers"] = len(merged.Servers)

	return merged, nil
}