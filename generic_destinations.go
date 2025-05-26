package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileDestination is a generic file-based destination
type FileDestination struct {
	ID          string
	Path        string
	Format      ExportFormat
	Transformer ConfigTransformer
}

// NewFileDestination creates a new file destination
func NewFileDestination(id, path string, format ExportFormat) *FileDestination {
	return &FileDestination{
		ID:     id,
		Path:   path,
		Format: format,
	}
}

// GetID returns the destination identifier
func (f *FileDestination) GetID() string {
	return f.ID
}

// GetDescription returns a human-readable description
func (f *FileDestination) GetDescription() string {
	return fmt.Sprintf("File destination at %s", f.Path)
}

// Transform converts the config to the appropriate format
func (f *FileDestination) Transform(config *Config) (interface{}, error) {
	if f.Transformer != nil {
		return f.Transformer.Transform(config)
	}

	// Default: just return servers
	servers := make(map[string]ServerConfig)
	for name, server := range config.Servers {
		servers[name] = server.ServerConfig
	}

	return map[string]interface{}{
		"mcpServers": servers,
	}, nil
}

// Read reads the current configuration
func (f *FileDestination) Read() ([]byte, error) {
	path := expandPath(f.Path)
	return os.ReadFile(path)
}

// Write writes the configuration
func (f *FileDestination) Write(data []byte) error {
	path := expandPath(f.Path)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	return os.Rename(tmpPath, path)
}

// Exists checks if the destination exists
func (f *FileDestination) Exists() bool {
	path := expandPath(f.Path)
	_, err := os.Stat(path)
	return err == nil
}

// SupportsBackup returns true
func (f *FileDestination) SupportsBackup() bool {
	return true
}

// Backup creates a backup of the current file
func (f *FileDestination) Backup() (string, error) {
	path := expandPath(f.Path)

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil // No backup needed
	}

	// Read existing content
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Create backup path
	backupDir := filepath.Join(filepath.Dir(path), ".backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s.%s.backup",
		filepath.Base(path), timestamp))

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}

	return backupPath, nil
}

// ConfigTransformers for common formats

// FlatTransformer creates a flat structure
type FlatTransformer struct {
	WrapperKey string // e.g., "mcpServers"
}

func (f *FlatTransformer) Transform(config *Config) (interface{}, error) {
	servers := make(map[string]ServerConfig)
	for name, server := range config.Servers {
		servers[name] = server.ServerConfig
	}

	if f.WrapperKey != "" {
		return map[string]interface{}{
			f.WrapperKey: servers,
		}, nil
	}

	return servers, nil
}

func (f *FlatTransformer) Format() string {
	return "flat"
}

// NestedTransformer creates a nested structure
type NestedTransformer struct {
	RootKey    string // e.g., "mcp"
	ServersKey string // e.g., "servers"
}

func (n *NestedTransformer) Transform(config *Config) (interface{}, error) {
	servers := make(map[string]ServerConfig)
	for name, server := range config.Servers {
		servers[name] = server.ServerConfig
	}

	result := make(map[string]interface{})

	if n.RootKey != "" {
		root := make(map[string]interface{})
		key := n.ServersKey
		if key == "" {
			key = "servers"
		}
		root[key] = servers
		result[n.RootKey] = root
	} else {
		key := n.ServersKey
		if key == "" {
			key = "servers"
		}
		result[key] = servers
	}

	return result, nil
}

func (n *NestedTransformer) Format() string {
	return "nested"
}

// DirectTransformer returns config as-is
type DirectTransformer struct{}

func (d *DirectTransformer) Transform(config *Config) (interface{}, error) {
	return config, nil
}

func (d *DirectTransformer) Format() string {
	return "direct"
}

// expandPath is now in utils.go
