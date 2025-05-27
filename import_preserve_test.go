package engine

import (
	"os"
	"testing"
)

func TestImportPreservesVariables(t *testing.T) {
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

	// Set environment variable
	os.Setenv("GITHUB_TOKEN", "test-token-123")
	defer os.Unsetenv("GITHUB_TOKEN")

	// Import WITHOUT env var substitution (default behavior)
	err = e.Import(data, ImportFormat("mcp"), ImportOptions{
		OverwriteExisting: true,
		SubstituteEnvVars: false, // Explicit: don't substitute
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Check that variables were NOT substituted
	config, err := e.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	
	// Check environment variable NOT substituted
	if server, exists := config.Servers["github-env"]; exists {
		if token, ok := server.Env["GITHUB_PERSONAL_ACCESS_TOKEN"]; ok {
			if token != "${GITHUB_TOKEN}" {
				t.Errorf("Expected env var to be preserved as '${GITHUB_TOKEN}', got %q", token)
			}
		} else {
			t.Error("GITHUB_PERSONAL_ACCESS_TOKEN not found in env")
		}
	} else {
		t.Error("github-env server not found")
	}

	// Check that input variables are still preserved
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

func TestImportDefaultBehavior(t *testing.T) {
	// Test that the default behavior preserves variables (no substitution)
	e, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	data, err := os.ReadFile("testdata/test_input_variables.json")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Set environment variable
	os.Setenv("GITHUB_TOKEN", "test-token-123")
	defer os.Unsetenv("GITHUB_TOKEN")

	// Import with minimal options (testing default behavior)
	err = e.Import(data, ImportFormat("mcp"), ImportOptions{
		OverwriteExisting: true,
		// SubstituteEnvVars not specified - should default to false
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	config, err := e.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// By default, variables should be preserved
	if server, exists := config.Servers["github-env"]; exists {
		if token, ok := server.Env["GITHUB_PERSONAL_ACCESS_TOKEN"]; ok {
			if token != "${GITHUB_TOKEN}" {
				t.Errorf("Default behavior should preserve variables, but got %q", token)
			}
		}
	}
}