// Package presets provides common MCP destination configurations
// This is separate from the core engine and completely optional
package presets

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	engine "github.com/b-open-io/agent-master-engine"
)

// Preset defines a common MCP destination configuration
type Preset struct {
	Name                 string
	Description          string
	DefaultPath          string
	ConfigFormat         string // "flat", "nested", "project-nested", etc.
	FileFormat           string // "json", "yaml", "toml"
	NamePattern          string // Regex pattern for name validation
	NameSanitizer        func(string) string
	RequiresSanitization bool
	SupportsProjects     bool
	CustomTransform      func(*engine.Config) (interface{}, error)
}

// Common presets - users can define their own
var CommonPresets = map[string]Preset{
	"claude": {
		Name:                 "claude",
		Description:          "Claude Desktop configuration",
		DefaultPath:          getClaudeDesktopConfigPath(),
		ConfigFormat:         "project-nested",
		FileFormat:           "json",
		NamePattern:          "^[a-zA-Z0-9_-]{1,64}$",
		RequiresSanitization: true,
		NameSanitizer:        sanitizeForClaude,
		SupportsProjects:     true,
		CustomTransform:      transformForClaude,
	},
	"vscode-mcp": {
		Name:         "vscode-mcp",
		Description:  "VS Code MCP extension",
		DefaultPath:  "~/.vscode/extensions/mcp/settings.json",
		ConfigFormat: "flat",
		FileFormat:   "json",
	},
	"cursor": {
		Name:         "cursor",
		Description:  "Cursor IDE",
		DefaultPath:  "~/Library/Application Support/Cursor/User/globalStorage/settings.json",
		ConfigFormat: "flat",
		FileFormat:   "json",
	},
	"generic-json": {
		Name:         "generic-json",
		Description:  "Generic JSON format",
		DefaultPath:  "./mcp-config.json",
		ConfigFormat: "flat",
		FileFormat:   "json",
	},
}

// NewDestination creates a destination from a preset
func NewDestination(presetName string, customPath ...string) (engine.Destination, error) {
	preset, ok := CommonPresets[presetName]
	if !ok {
		return nil, fmt.Errorf("unknown preset: %s", presetName)
	}

	path := preset.DefaultPath
	if len(customPath) > 0 && customPath[0] != "" {
		path = customPath[0]
	}

	return &PresetDestination{
		preset: preset,
		path:   path,
	}, nil
}

// PresetDestination implements engine.Destination
type PresetDestination struct {
	preset Preset
	path   string
}

func (pd *PresetDestination) GetID() string {
	return pd.preset.Name
}

func (pd *PresetDestination) GetDescription() string {
	return pd.preset.Description
}

func (pd *PresetDestination) GetPath() string {
	return pd.path
}

func (pd *PresetDestination) Transform(config *engine.Config) (interface{}, error) {
	// Use custom transform if provided
	if pd.preset.CustomTransform != nil {
		return pd.preset.CustomTransform(config)
	}

	// Otherwise use standard transforms based on format
	switch pd.preset.ConfigFormat {
	case "flat":
		return pd.transformFlat(config), nil
	case "nested":
		return pd.transformNested(config), nil
	case "project-nested":
		return pd.transformProjectNested(config), nil
	default:
		return config.Servers, nil
	}
}

func (pd *PresetDestination) transformFlat(config *engine.Config) map[string]interface{} {
	servers := make(map[string]engine.ServerConfig)
	for name, server := range config.Servers {
		if pd.preset.RequiresSanitization && pd.preset.NameSanitizer != nil {
			name = pd.preset.NameSanitizer(name)
		}
		servers[name] = server.ServerConfig
	}
	return map[string]interface{}{
		"mcpServers": servers,
	}
}

func (pd *PresetDestination) transformNested(config *engine.Config) map[string]interface{} {
	flat := pd.transformFlat(config)
	return map[string]interface{}{
		"mcp": flat,
	}
}

func (pd *PresetDestination) transformProjectNested(config *engine.Config) map[string]interface{} {
	// This is more complex - would need to read existing config
	// and merge appropriately
	return pd.transformFlat(config)
}

func (pd *PresetDestination) Read() ([]byte, error) {
	path := expandPath(pd.path)
	return os.ReadFile(path)
}

func (pd *PresetDestination) Write(data []byte) error {
	path := expandPath(pd.path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func (pd *PresetDestination) Exists() bool {
	path := expandPath(pd.path)
	_, err := os.Stat(path)
	return err == nil
}

func (pd *PresetDestination) SupportsBackup() bool {
	return true
}

func (pd *PresetDestination) Backup() (string, error) {
	// Simple timestamp-based backup
	path := expandPath(pd.path)
	backupPath := fmt.Sprintf("%s.backup.%d", path, time.Now().Unix())

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}

	return backupPath, nil
}

// CreateValidator creates a validator from preset
func CreateValidator(presetName string) engine.ServerValidator {
	preset, ok := CommonPresets[presetName]
	if !ok || preset.NamePattern == "" {
		return nil
	}

	return &PatternValidator{
		pattern: regexp.MustCompile(preset.NamePattern),
		preset:  preset,
	}
}

// CreateSanitizer creates a sanitizer from preset
func CreateSanitizer(presetName string) engine.NameSanitizer {
	preset, ok := CommonPresets[presetName]
	if !ok || preset.NameSanitizer == nil {
		return nil
	}

	return &PresetSanitizer{
		sanitize: preset.NameSanitizer,
		pattern:  preset.NamePattern,
	}
}

// PatternValidator validates using regex pattern
type PatternValidator struct {
	pattern *regexp.Regexp
	preset  Preset
}

func (pv *PatternValidator) ValidateName(name string) error {
	if !pv.pattern.MatchString(name) {
		return fmt.Errorf("name must match pattern %s", pv.preset.NamePattern)
	}
	return nil
}

func (pv *PatternValidator) ValidateConfig(config engine.ServerConfig) error {
	// Basic validation - can be extended
	if config.Transport != "stdio" && config.Transport != "sse" {
		return fmt.Errorf("invalid transport: %s", config.Transport)
	}
	return nil
}

func (pv *PatternValidator) ValidateServerConfig(name string, config engine.ServerConfig) error {
	// Validate name
	if err := pv.ValidateName(name); err != nil {
		return err
	}
	// Validate config
	return pv.ValidateConfig(config)
}

// PresetSanitizer sanitizes names
type PresetSanitizer struct {
	sanitize func(string) string
	pattern  string
}

func (ps *PresetSanitizer) Sanitize(name string) string {
	return ps.sanitize(name)
}

func (ps *PresetSanitizer) NeedsSanitization(name string) bool {
	return name != ps.sanitize(name)
}

// Helper functions
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// Example sanitizer for Claude (not in core engine!)
func sanitizeForClaude(name string) string {
	// Remove @ prefix
	name = strings.TrimPrefix(name, "@")

	// Replace problematic characters
	replacements := map[string]string{
		"/": "-",
		" ": "-",
		".": "-",
	}

	for old, new := range replacements {
		name = strings.ReplaceAll(name, old, new)
	}

	// Remove other special characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	name = reg.ReplaceAllString(name, "")

	// Cleanup
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-_")

	if name == "" {
		name = "unnamed-server"
	}

	if len(name) > 64 {
		name = name[:64]
	}

	return name
}

// Example transformer for Claude's specific format
func transformForClaude(config *engine.Config) (interface{}, error) {
	// This would handle Claude's project-nested format
	// Reading existing config, preserving non-MCP fields, etc.
	// For now, simplified
	return map[string]interface{}{
		"mcpServers": config.Servers,
	}, nil
}

// getClaudeDesktopConfigPath returns the platform-specific Claude Desktop config path
func getClaudeDesktopConfigPath() string {
	switch runtime.GOOS {
	case "darwin": // macOS
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
		}
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Claude", "claude_desktop_config.json")
		}
	case "linux":
		if home, err := os.UserHomeDir(); err == nil {
			// Try XDG_CONFIG_HOME first, fallback to ~/.config
			if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
				return filepath.Join(configHome, "Claude", "claude_desktop_config.json")
			}
			return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
		}
	}
	
	// Fallback to the old default
	return "~/.claude.json"
}
