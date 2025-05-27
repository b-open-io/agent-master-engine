package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config holds daemon configuration
type Config struct {
	// Storage
	StoragePath string `json:"storage_path,omitempty"`
	
	// Network
	SocketPath string `json:"socket_path,omitempty"`
	Port       int    `json:"port,omitempty"`
	
	// Logging
	LogLevel string `json:"log_level,omitempty"`
	LogFile  string `json:"log_file,omitempty"`
	
	// Behavior
	IdleTimeout   time.Duration `json:"idle_timeout,omitempty"`
	EnableSystemd bool          `json:"enable_systemd,omitempty"`
	
	// Security
	AllowedClients []string `json:"allowed_clients,omitempty"`
}

// SetDefaults sets default values for config
func (c *Config) SetDefaults() {
	if c.StoragePath == "" {
		home, _ := os.UserHomeDir()
		c.StoragePath = filepath.Join(home, ".agent-master")
	}
	
	if c.SocketPath == "" {
		c.SocketPath = filepath.Join(os.TempDir(), "agent-master-daemon.sock")
	}
	
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	
	// Expand paths
	c.StoragePath = expandPath(c.StoragePath)
	if c.LogFile != "" {
		c.LogFile = expandPath(c.LogFile)
	}
}

// LoadFromFile loads config from JSON file
func (c *Config) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	
	c.SetDefaults()
	return nil
}

// SaveToFile saves config to JSON file
func (c *Config) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0644)
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}