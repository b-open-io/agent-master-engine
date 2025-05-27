package engine

import (
	"os"
	"testing"
)

func TestImportMCPFormats(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		expectCount  int
		expectServer string // Name of one server to check
	}{
		{
			name:         "Claude Desktop format",
			file:         "testdata/master_config.json",
			expectCount:  10,
			expectServer: "obsidian",
		},
		{
			name:         "GitHub MCP format with inputs",
			file:         "testdata/mcp_format_github.json",
			expectCount:  1,
			expectServer: "github",
		},
		{
			name:         "SSE servers",
			file:         "testdata/sse_servers.json",
			expectCount:  4,
			expectServer: "Zapier_MCP",
		},
		{
			name:         "Stdio servers",
			file:         "testdata/stdio_servers.json",
			expectCount:  10,
			expectServer: "filesystem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create engine with memory storage
			e, err := NewEngine(WithMemoryStorage())
			if err != nil {
				t.Fatalf("Failed to create engine: %v", err)
			}

			// Read test file
			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("Failed to read test file: %v", err)
			}

			// Import with default options
			err = e.Import(data, ImportFormat("mcp"), ImportOptions{
				OverwriteExisting: true,
				MergeMode:         "merge",
			})
			if err != nil {
				t.Fatalf("Import failed: %v", err)
			}

			// Get config and check server count
			config, err := e.GetConfig()
			if err != nil {
				t.Fatalf("Failed to get config: %v", err)
			}
			if len(config.Servers) != tt.expectCount {
				t.Errorf("Expected %d servers, got %d", tt.expectCount, len(config.Servers))
			}

			// Check specific server exists
			if _, exists := config.Servers[tt.expectServer]; !exists {
				t.Errorf("Expected server %q not found", tt.expectServer)
			}
		})
	}
}

func TestImportWithVariableSubstitution(t *testing.T) {
	// Create engine
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Read test file with variables
	data, err := os.ReadFile("testdata/test_input_variables.json")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Set environment variable for testing
	os.Setenv("GITHUB_TOKEN", "test-token-123")
	defer os.Unsetenv("GITHUB_TOKEN")

	// Import with env var substitution enabled
	err = e.Import(data, ImportFormat("mcp"), ImportOptions{
		OverwriteExisting: true,
		SubstituteEnvVars: true,
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Check that variables were substituted
	config, err := e.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	
	// Check environment variable substitution
	if server, exists := config.Servers["github-env"]; exists {
		if token, ok := server.Env["GITHUB_PERSONAL_ACCESS_TOKEN"]; ok {
			if token != "test-token-123" {
				t.Errorf("Expected env var to be 'test-token-123', got %q", token)
			}
		} else {
			t.Error("GITHUB_PERSONAL_ACCESS_TOKEN not found in env")
		}
	} else {
		t.Error("github-env server not found")
	}

	// Check that input variables are preserved (not substituted yet)
	if server, exists := config.Servers["github-input"]; exists {
		if token, ok := server.Env["GITHUB_TOKEN"]; ok {
			if token != "${input:github_token}" {
				t.Errorf("Expected input var to be preserved as '${input:github_token}', got %q", token)
			}
		} else {
			t.Error("GITHUB_TOKEN not found in env")
		}
	} else {
		t.Error("github-input server not found")
	}
}

func TestImportRealWorldExamples(t *testing.T) {
	// Create engine
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Read real world examples
	data, err := os.ReadFile("testdata/test_real_world_examples.json")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Import
	err = e.Import(data, ImportFormat("mcp"), ImportOptions{
		OverwriteExisting: true,
		SkipInvalid:       true, // Skip any invalid servers
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Check some known servers
	config, err := e.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	expectedServers := []string{
		"postgres",
		"sqlite",
		"filesystem",
		"git",
		"github",
		"gitlab",
		"puppeteer",
		"playwright",
		"slack",
		"discord",
	}

	for _, name := range expectedServers {
		if _, exists := config.Servers[name]; !exists {
			t.Errorf("Expected server %q not found", name)
		}
	}

	// Check Docker-based server
	if server, exists := config.Servers["github"]; exists {
		if server.Command != "docker" {
			t.Errorf("Expected github server to use docker command, got %q", server.Command)
		}
	}
}