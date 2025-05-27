package engine

import (
	"os"
	"testing"
)

// TestMCPVersionCompatibility tests that we can parse both MCP versions
func TestMCPVersionCompatibility(t *testing.T) {
	tests := []struct {
		name            string
		file            string
		expectedVersion string
		expectedServers []string
		checkFeatures   func(t *testing.T, config *Config)
	}{
		{
			name:            "MCP 2024-11-05",
			file:            "testdata/mcp_v2024_11_05.json",
			expectedVersion: "2024-11-05",
			expectedServers: []string{"filesystem", "github", "legacy-sse"},
			checkFeatures: func(t *testing.T, config *Config) {
				// Check SSE transport is supported
				if server, ok := config.Servers["legacy-sse"]; ok {
					if server.Transport != "sse" {
						t.Errorf("Expected SSE transport, got %s", server.Transport)
					}
					if server.URL == "" {
						t.Error("SSE server should have URL")
					}
				}
			},
		},
		{
			name:            "MCP 2025-03-26",
			file:            "testdata/mcp_v2025_03_26.json",
			expectedVersion: "2025-03-26",
			expectedServers: []string{"filesystem", "github", "audio-processor", "batch-processor"},
			checkFeatures: func(t *testing.T, config *Config) {
				// Check for new metadata/annotations
				if server, ok := config.Servers["filesystem"]; ok {
					if server.Metadata == nil {
						t.Error("Expected metadata for filesystem server")
					} else if annotations, ok := server.Metadata["annotations"]; ok {
						if annotMap, ok := annotations.(map[string]interface{}); ok {
							if readOnly, ok := annotMap["readOnly"].(bool); !ok || !readOnly {
								t.Error("Expected readOnly annotation to be true")
							}
						}
					}
				}
				
				// Check OAuth annotations
				if server, ok := config.Servers["github"]; ok {
					if server.Metadata != nil {
						if annotations, ok := server.Metadata["annotations"].(map[string]interface{}); ok {
							if oauth, ok := annotations["oauth"].(map[string]interface{}); ok {
								if required, ok := oauth["required"].(bool); !ok || !required {
									t.Error("Expected OAuth to be required")
								}
							}
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read test file
			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("Failed to read test file: %v", err)
			}

			// Parse config
			config, err := ParseMCPConfig(data)
			if err != nil {
				t.Fatalf("Failed to parse config: %v", err)
			}

			// Check server count
			if len(config.Servers) != len(tt.expectedServers) {
				t.Errorf("Expected %d servers, got %d", len(tt.expectedServers), len(config.Servers))
			}

			// Check expected servers exist
			for _, serverName := range tt.expectedServers {
				if _, ok := config.Servers[serverName]; !ok {
					t.Errorf("Expected server %q not found", serverName)
				}
			}

			// Run version-specific checks
			if tt.checkFeatures != nil {
				tt.checkFeatures(t, config)
			}
		})
	}
}

// TestMCPVersionBackwardCompatibility verifies that 2025-03-26 configs work with 2024-11-05 parsers
func TestMCPVersionBackwardCompatibility(t *testing.T) {
	// Load a 2025-03-26 config
	data, err := os.ReadFile("testdata/mcp_v2025_03_26.json")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Parse it (simulating a 2024-11-05 parser that ignores unknown fields)
	config, err := ParseMCPConfig(data)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Core functionality should still work
	basicServers := []string{"filesystem", "github"}
	for _, serverName := range basicServers {
		server, ok := config.Servers[serverName]
		if !ok {
			t.Errorf("Basic server %q not found", serverName)
			continue
		}

		// Check basic fields are preserved
		if server.Transport == "" {
			t.Errorf("Server %q missing transport", serverName)
		}
		if server.Command == "" {
			t.Errorf("Server %q missing command", serverName)
		}
	}

	// New servers with new features should still parse
	if _, ok := config.Servers["audio-processor"]; !ok {
		t.Error("Audio processor server should parse even without audio support")
	}
}

// TestMCPVersionSpecificFeatures tests version-specific features
func TestMCPVersionSpecificFeatures(t *testing.T) {
	t.Run("OAuth 2.1 Authorization", func(t *testing.T) {
		// This would test OAuth configuration in 2025-03-26
		// For now, we just verify the metadata is preserved
		data, err := os.ReadFile("testdata/mcp_v2025_03_26.json")
		if err != nil {
			t.Fatal(err)
		}

		config, err := ParseMCPConfig(data)
		if err != nil {
			t.Fatal(err)
		}

		github := config.Servers["github"]
		if github.Metadata == nil {
			t.Fatal("Expected metadata for OAuth configuration")
		}
	})

	t.Run("JSON-RPC Batching", func(t *testing.T) {
		// Verify batching capability is preserved in metadata
		data, err := os.ReadFile("testdata/mcp_v2025_03_26.json")
		if err != nil {
			t.Fatal(err)
		}

		config, err := ParseMCPConfig(data)
		if err != nil {
			t.Fatal(err)
		}

		batch := config.Servers["batch-processor"]
		if batch.Metadata == nil {
			t.Fatal("Expected metadata for batch processor")
		}
	})
}

// TestMCPTransportMigration tests the SSE to Streamable HTTP migration
func TestMCPTransportMigration(t *testing.T) {
	// In 2024-11-05, SSE was a valid transport
	oldData, err := os.ReadFile("testdata/mcp_v2024_11_05.json")
	if err != nil {
		t.Fatal(err)
	}

	oldConfig, err := ParseMCPConfig(oldData)
	if err != nil {
		t.Fatal(err)
	}

	// Check SSE transport exists in old version
	sseServer, ok := oldConfig.Servers["legacy-sse"]
	if !ok || sseServer.Transport != "sse" {
		t.Error("Expected SSE transport in 2024-11-05 version")
	}

	// In 2025-03-26, SSE might be deprecated (though still supported for compatibility)
	// This is where we'd add migration logic if needed
}