package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMCPFormatParsing(t *testing.T) {
	testCases := []struct {
		name     string
		jsonFile string
		wantErr  bool
	}{
		{
			name:     "Claude Desktop format",
			jsonFile: "testdata/master_config.json",
			wantErr:  false,
		},
		{
			name:     "GitHub MCP format with inputs",
			jsonFile: "testdata/mcp_format_github.json",
			wantErr:  false,
		},
		{
			name:     "SSE servers",
			jsonFile: "testdata/sse_servers.json",
			wantErr:  false,
		},
		{
			name:     "stdio servers",
			jsonFile: "testdata/stdio_servers.json",
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.jsonFile)
			if err != nil {
				t.Fatalf("Failed to read test file %s: %v", tc.jsonFile, err)
			}

			// Test parsing with MCPConfig
			var mcpConfig MCPConfig
			err = json.Unmarshal(data, &mcpConfig)
			if err != nil && !tc.wantErr {
				t.Errorf("Failed to parse as MCPConfig: %v", err)
			}

			// Try to convert to standard Config
			if err == nil {
				config, err := mcpConfig.ToConfig()
				if err != nil && !tc.wantErr {
					t.Errorf("Failed to convert to Config: %v", err)
				}
				if config != nil && len(config.Servers) == 0 && !tc.wantErr {
					t.Errorf("No servers found after conversion")
				}
			}

			// Also test direct parsing
			config, err := ParseMCPConfig(data)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseMCPConfig() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && config != nil {
				t.Logf("Successfully parsed %d servers from %s", len(config.Servers), tc.jsonFile)
			}
		})
	}
}

func TestVariableSubstitution(t *testing.T) {
	// Set up test environment
	os.Setenv("TEST_API_KEY", "secret123")
	os.Setenv("HOME", "/home/testuser")
	defer os.Unsetenv("TEST_API_KEY")
	defer os.Unsetenv("HOME")

	inputs := map[string]string{
		"github_token": "ghp_test123",
		"workspace":    "/workspace/test",
	}

	env := map[string]string{
		"CUSTOM_VAR": "custom_value",
	}

	testCases := []struct {
		name   string
		config ServerConfig
		want   ServerConfig
	}{
		{
			name: "Environment variable substitution",
			config: ServerConfig{
				Command: "test",
				Env: map[string]string{
					"API_KEY": "${TEST_API_KEY}",
					"HOME":    "${HOME}",
				},
			},
			want: ServerConfig{
				Command: "test",
				Env: map[string]string{
					"API_KEY": "secret123",
					"HOME":    "/home/testuser",
				},
			},
		},
		{
			name: "Input variable substitution",
			config: ServerConfig{
				Command: "docker",
				Args:    []string{"run", "-e", "TOKEN=${input:github_token}"},
				Env: map[string]string{
					"GITHUB_TOKEN": "${input:github_token}",
					"WORKSPACE":    "${input:workspace}",
				},
			},
			want: ServerConfig{
				Command: "docker",
				Args:    []string{"run", "-e", "TOKEN=ghp_test123"},
				Env: map[string]string{
					"GITHUB_TOKEN": "ghp_test123",
					"WORKSPACE":    "/workspace/test",
				},
			},
		},
		{
			name: "Mixed substitution",
			config: ServerConfig{
				Command: "test",
				Env: map[string]string{
					"COMPOSITE": "${input:github_token}-${CUSTOM_VAR}",
				},
			},
			want: ServerConfig{
				Command: "test",
				Env: map[string]string{
					"COMPOSITE": "ghp_test123-custom_value",
				},
			},
		},
		{
			name: "SSE headers substitution",
			config: ServerConfig{
				Transport: "sse",
				URL:       "https://api.example.com",
				Headers: map[string]string{
					"Authorization": "Bearer ${input:github_token}",
					"X-Custom":      "${CUSTOM_VAR}",
				},
			},
			want: ServerConfig{
				Transport: "sse",
				URL:       "https://api.example.com",
				Headers: map[string]string{
					"Authorization": "Bearer ghp_test123",
					"X-Custom":      "custom_value",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SubstituteVariables(tc.config, inputs, env)
			
			// Compare results
			if tc.want.Command != result.Command {
				t.Errorf("Command: want %s, got %s", tc.want.Command, result.Command)
			}

			// Compare env
			if len(tc.want.Env) > 0 {
				for k, v := range tc.want.Env {
					if result.Env[k] != v {
						t.Errorf("Env[%s]: want %s, got %s", k, v, result.Env[k])
					}
				}
			}

			// Compare args
			if len(tc.want.Args) > 0 {
				for i, arg := range tc.want.Args {
					if i < len(result.Args) && result.Args[i] != arg {
						t.Errorf("Args[%d]: want %s, got %s", i, arg, result.Args[i])
					}
				}
			}

			// Compare headers
			if len(tc.want.Headers) > 0 {
				for k, v := range tc.want.Headers {
					if result.Headers[k] != v {
						t.Errorf("Headers[%s]: want %s, got %s", k, v, result.Headers[k])
					}
				}
			}
		})
	}
}

func TestMCPFormatExamples(t *testing.T) {
	// Test parsing the format examples file
	data, err := os.ReadFile("testdata/mcp_formats.json")
	if err != nil {
		t.Skip("mcp_formats.json not found")
	}

	var formats struct {
		Formats []struct {
			Name    string          `json:"name"`
			Example json.RawMessage `json:"example"`
		} `json:"formats"`
	}

	if err := json.Unmarshal(data, &formats); err != nil {
		t.Fatalf("Failed to parse formats file: %v", err)
	}

	for _, format := range formats.Formats {
		t.Run(format.Name, func(t *testing.T) {
			config, err := ParseMCPConfig(format.Example)
			if err != nil {
				t.Errorf("Failed to parse %s: %v", format.Name, err)
				return
			}

			if config == nil || len(config.Servers) == 0 {
				t.Errorf("No servers found in %s format", format.Name)
			}
		})
	}
}

func TestRealWorldExamples(t *testing.T) {
	// Test examples from awesome-mcp-servers
	testFiles := []string{
		"testdata/docker_examples.json",
		"testdata/real_world_examples.json",
	}

	for _, file := range testFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Skip("Test file not found:", file)
			}

			// These files have a different structure, so we just verify they're valid JSON
			var examples interface{}
			if err := json.Unmarshal(data, &examples); err != nil {
				t.Errorf("Invalid JSON in %s: %v", file, err)
			}
		})
	}
}