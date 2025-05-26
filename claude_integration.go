package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ClaudeCodeAdapter provides integration with Claude Code
type ClaudeCodeAdapter struct {
	configPath string
	sdkPath    string // Path to claude executable
}

// NewClaudeCodeAdapter creates a new Claude Code adapter
func NewClaudeCodeAdapter() (*ClaudeCodeAdapter, error) {
	// Find claude executable
	sdkPath, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude executable not found: %w", err)
	}

	// Determine config path
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return &ClaudeCodeAdapter{
		configPath: filepath.Join(home, ".claude.json"),
		sdkPath:    sdkPath,
	}, nil
}

// ValidateServerConfig validates a server config using Claude Code SDK
func (c *ClaudeCodeAdapter) ValidateServerConfig(name string, config ServerConfig) error {
	// Create temporary config file
	tmpConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			name: config,
		},
	}

	tmpFile, err := os.CreateTemp("", "mcp-test-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if err := json.NewEncoder(tmpFile).Encode(tmpConfig); err != nil {
		return err
	}
	tmpFile.Close()

	// Test with Claude Code SDK
	cmd := exec.Command(c.sdkPath,
		"--mcp-config", tmpFile.Name(),
		"--system-prompt", "Test MCP configuration",
		"-p", "exit", // Non-interactive mode
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("validation failed: %s", output)
	}

	return nil
}

// TestMCPServer tests if an MCP server starts correctly
func (c *ClaudeCodeAdapter) TestMCPServer(name string, config ServerConfig) error {
	// Use Claude Code SDK to test server startup
	tmpConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			name: config,
		},
	}

	tmpFile, err := os.CreateTemp("", "mcp-test-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if err := json.NewEncoder(tmpFile).Encode(tmpConfig); err != nil {
		return err
	}
	tmpFile.Close()

	// Run with --mcp-debug for detailed output
	cmd := exec.Command(c.sdkPath,
		"--mcp-config", tmpFile.Name(),
		"--mcp-debug",
		"-p", "List available MCP tools",
		"--output-format", "json",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("server test failed: %s", output)
	}

	// Parse output to check if server started
	var result struct {
		Tools []struct {
			Name   string `json:"name"`
			Server string `json:"server"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		// Not JSON, check for error messages
		if strings.Contains(string(output), "error") ||
			strings.Contains(string(output), "failed") {
			return fmt.Errorf("server startup failed: %s", output)
		}
	}

	return nil
}

// GetAllowedTools extracts allowed tools from Claude Code configuration
func (c *ClaudeCodeAdapter) GetAllowedTools(projectPath string) ([]string, error) {
	// Read Claude Code config
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return nil, err
	}

	var config struct {
		Projects map[string]struct {
			AllowedTools []string `json:"allowedTools"`
		} `json:"projects"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if project, ok := config.Projects[projectPath]; ok {
		return project.AllowedTools, nil
	}

	return []string{}, nil
}

// FormatToolName formats a tool name according to Claude Code conventions
// MCP tools follow pattern: mcp__<serverName>__<toolName>
func FormatToolName(serverName, toolName string) string {
	return fmt.Sprintf("mcp__%s__%s", serverName, toolName)
}

// ParseToolName parses a Claude Code tool name
func ParseToolName(toolName string) (serverName, tool string, isMCP bool) {
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.Split(toolName, "__")
		if len(parts) >= 3 {
			return parts[1], strings.Join(parts[2:], "__"), true
		}
	}
	return "", toolName, false
}

// ClaudeCodeConfig represents the full Claude Code configuration
type ClaudeCodeConfig struct {
	MCPServers map[string]ServerConfig    `json:"mcpServers"`
	Projects   map[string]ProjectSettings `json:"projects"`
	Theme      string                     `json:"theme,omitempty"`
	// ... other Claude Code specific fields
}

// ProjectSettings represents Claude Code project-specific settings
type ProjectSettings struct {
	MCPServers                 map[string]ServerConfig `json:"mcpServers,omitempty"`
	AllowedTools               []string                `json:"allowedTools,omitempty"`
	DisabledMCPJSONServers     []string                `json:"disabledMcpjsonServers,omitempty"`
	EnabledMCPJSONServers      []string                `json:"enabledMcpjsonServers,omitempty"`
	EnableAllProjectMCPServers bool                    `json:"enableAllProjectMcpServers,omitempty"`
	DontCrawlDirectory         bool                    `json:"dontCrawlDirectory,omitempty"`
}

// ReadClaudeCodeConfig reads the Claude Code configuration
func (c *ClaudeCodeAdapter) ReadClaudeCodeConfig() (*ClaudeCodeConfig, error) {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config
			return &ClaudeCodeConfig{
				MCPServers: make(map[string]ServerConfig),
				Projects:   make(map[string]ProjectSettings),
			}, nil
		}
		return nil, err
	}

	var config ClaudeCodeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Initialize maps if nil
	if config.MCPServers == nil {
		config.MCPServers = make(map[string]ServerConfig)
	}
	if config.Projects == nil {
		config.Projects = make(map[string]ProjectSettings)
	}

	return &config, nil
}

// WriteClaudeCodeConfig writes the Claude Code configuration
func (c *ClaudeCodeAdapter) WriteClaudeCodeConfig(config *ClaudeCodeConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write atomically
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := c.configPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, c.configPath)
}
