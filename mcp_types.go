package engine

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"time"
)

// MCPConfig represents the newer MCP configuration format with inputs support
type MCPConfig struct {
	MCP *MCPWrapper `json:"mcp,omitempty"`
	// Also support direct mcpServers for Claude/Cursor format
	MCPServers map[string]ServerConfig `json:"mcpServers,omitempty"`
	// And direct servers for simpler formats
	Servers map[string]ServerConfig `json:"servers,omitempty"`
}

// MCPWrapper contains the MCP-specific configuration
type MCPWrapper struct {
	Inputs  []MCPInput              `json:"inputs,omitempty"`
	Servers map[string]ServerConfig `json:"servers"`
}

// MCPInput defines an input variable for MCP configurations
type MCPInput struct {
	Type        string `json:"type"`               // "promptString", "promptNumber", etc.
	ID          string `json:"id"`                 // Variable identifier
	Description string `json:"description"`        // Human-readable description
	Default     string `json:"default,omitempty"`  // Default value
	Password    bool   `json:"password,omitempty"` // Hide input (for secrets)
	Required    bool   `json:"required,omitempty"` // Is this input required?
}

// MCPVersion represents different MCP protocol versions
type MCPVersion string

const (
	MCPVersion20241105 MCPVersion = "2024-11-05"
	MCPVersion20250326 MCPVersion = "2025-03-26" 
	MCPVersionDraft    MCPVersion = "draft"
)

// ParseMCPConfig attempts to parse various MCP configuration formats
func ParseMCPConfig(data []byte) (*Config, error) {
	return ParseMCPConfigWithOptions(data, true) // Default: substitute env vars
}

// ParseMCPConfigWithOptions parses MCP config with options
func ParseMCPConfigWithOptions(data []byte, substituteEnvVars bool) (*Config, error) {
	// Try parsing as MCPConfig first (supports all formats)
	var mcpConfig MCPConfig
	if err := json.Unmarshal(data, &mcpConfig); err == nil {
		return mcpConfig.ToConfigWithOptions(substituteEnvVars)
	}

	// Fall back to direct Config parsing
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// ToConfig converts MCPConfig to the standard Config format (with env var substitution)
func (m *MCPConfig) ToConfig() (*Config, error) {
	return m.ToConfigWithOptions(true) // Default: substitute env vars
}

// ToConfigWithOptions converts MCPConfig with options
func (m *MCPConfig) ToConfigWithOptions(substituteEnvVars bool) (*Config, error) {
	config := &Config{
		Version: DefaultConfigVersion,
		Servers: make(map[string]ServerWithMetadata),
	}

	// Handle different formats
	servers := make(map[string]ServerConfig)

	// Priority: mcp.servers > mcpServers > servers
	if m.MCP != nil && m.MCP.Servers != nil {
		servers = m.MCP.Servers
		// TODO: Handle inputs - store in metadata or settings
		if len(m.MCP.Inputs) > 0 {
			if config.Metadata == nil {
				config.Metadata = make(map[string]interface{})
			}
			config.Metadata["inputs"] = m.MCP.Inputs
		}
	} else if m.MCPServers != nil {
		servers = m.MCPServers
	} else if m.Servers != nil {
		servers = m.Servers
	}

	// Convert to ServerWithMetadata
	for name, server := range servers {
		// Optionally apply environment variable substitution
		// Input variables are always preserved as-is for runtime substitution
		finalServer := server
		if substituteEnvVars {
			finalServer = SubstituteVariables(server, nil, nil)
		}
		
		config.Servers[name] = ServerWithMetadata{
			ServerConfig: finalServer,
			Internal: InternalMetadata{
				Enabled:      true,
				Source:       "mcp-config",
				LastModified: time.Now(),
			},
		}
	}

	return config, nil
}

// SubstituteVariables replaces ${input:xxx} and ${ENV_VAR} in server configs
func SubstituteVariables(config ServerConfig, inputs map[string]string, env map[string]string) ServerConfig {
	result := config

	// Substitute in env
	if result.Env != nil {
		newEnv := make(map[string]string)
		for k, v := range result.Env {
			newEnv[k] = substituteString(v, inputs, env)
		}
		result.Env = newEnv
	}

	// Substitute in args
	if result.Args != nil {
		newArgs := make([]string, len(result.Args))
		for i, arg := range result.Args {
			newArgs[i] = substituteString(arg, inputs, env)
		}
		result.Args = newArgs
	}

	// Substitute in headers (for SSE)
	if result.Headers != nil {
		newHeaders := make(map[string]string)
		for k, v := range result.Headers {
			newHeaders[k] = substituteString(v, inputs, env)
		}
		result.Headers = newHeaders
	}

	// Substitute in URL (for SSE)
	result.URL = substituteString(result.URL, inputs, env)
	
	// Substitute in command
	result.Command = substituteString(result.Command, inputs, env)

	return result
}

// substituteString replaces variables in a string
func substituteString(s string, inputs map[string]string, env map[string]string) string {
	// TODO: Implement proper variable substitution
	// This is a simplified version - in production, use a proper template engine
	result := s

	// Replace ${input:xxx}
	for key, value := range inputs {
		result = strings.ReplaceAll(result, "${input:"+key+"}", value)
	}

	// Replace ${ENV_VAR}
	for key, value := range env {
		result = strings.ReplaceAll(result, "${"+key+"}", value)
	}

	// Also check os.Getenv for unresolved env vars
	// This is a simple regex-based approach
	envPattern := regexp.MustCompile(`\$\{([^}]+)\}`)
	result = envPattern.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		if strings.HasPrefix(varName, "input:") {
			// Already handled above
			return match
		}
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match // Keep original if not found
	})

	return result
}