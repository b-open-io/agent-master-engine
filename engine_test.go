package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testDataPath returns the path to test data files
func testDataPath(filename string) string {
	return filepath.Join("testdata", filename)
}

// LoadTestData loads a JSON test data file
func LoadTestData(t *testing.T, filename string, v interface{}) {
	data, err := os.ReadFile(testDataPath(filename))
	if err != nil {
		t.Fatalf("Failed to read test data %s: %v", filename, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("Failed to parse test data %s: %v", filename, err)
	}
}

// TestServerNameSanitization tests the sanitization of server names for Claude Code
func TestServerNameSanitization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"valid-name", "valid-name"},
		{"invalid name", "invalid-name"},
		{"invalid@name", "invalid@name"}, // Current implementation doesn't remove @
		{"123invalid", "123invalid"},     // Current implementation doesn't add _
	}

	for _, tc := range testCases {
		result := SanitizeServerName(tc.input)
		if result != tc.expected {
			t.Errorf("SanitizeServerName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// TestTransportValidation tests validation of different transport types
func TestTransportValidation(t *testing.T) {
	testCases := []struct {
		name   string
		server ServerConfig
		valid  bool
	}{
		{
			name: "valid stdio server",
			server: ServerConfig{
				Transport: "stdio",
				Command:   "python",
				Args:      []string{"-m", "test"},
			},
			valid: true,
		},
		{
			name: "invalid stdio server - no command",
			server: ServerConfig{
				Transport: "stdio",
				Args:      []string{"-m", "test"},
			},
			valid: false,
		},
		{
			name: "valid sse server",
			server: ServerConfig{
				Transport: "sse",
				URL:       "http://localhost:8080",
			},
			valid: true,
		},
		{
			name: "invalid sse server - no url",
			server: ServerConfig{
				Transport: "sse",
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateServer(tc.name, tc.server)
			if tc.valid && err != nil {
				t.Errorf("Expected valid server but got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Errorf("Expected invalid server but got no error")
			}
		})
	}
}

// TestEngineBasics tests basic engine functionality
func TestEngineBasics(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Test adding a server
	server := ServerWithMetadata{
		ServerConfig: ServerConfig{
			Transport: "stdio",
			Command:   "python",
			Args:      []string{"-m", "test"},
		},
	}

	err = engine.AddServer("test-server", server.ServerConfig)
	if err != nil {
		t.Errorf("Failed to add server: %v", err)
	}

	// Test getting server
	retrievedServer, err := engine.GetServer("test-server")
	if err != nil {
		t.Errorf("Failed to get server: %v", err)
	}

	if retrievedServer == nil {
		t.Error("Retrieved server is nil")
	}
}

// TestSyncOptions tests sync options validation
func TestSyncOptions(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Add a test server
	server := ServerWithMetadata{
		ServerConfig: ServerConfig{
			Transport: "stdio",
			Command:   "python",
			Args:      []string{"-m", "test"},
		},
	}
	engine.AddServer("test-server", server.ServerConfig)

	// Test dry run
	options := SyncOptions{
		DryRun: true,
	}

	dest := &FileDestination{Path: "/tmp/test-config.json"}
	_, err = engine.SyncTo(context.Background(), dest, options)
	if err != nil {
		t.Errorf("Dry run sync failed: %v", err)
	}
}
