package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectScanning(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "project-scan-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test project structure
	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create a package.json file to make it a detectable project
	packageJSON := `{
		"name": "test-project",
		"version": "1.0.0"
	}`
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create an MCP config file
	mcpConfig := `{
		"mcpServers": {
			"filesystem": {
				"transport": "stdio",
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(projectDir, "mcp.json"), []byte(mcpConfig), 0644); err != nil {
		t.Fatalf("Failed to create mcp.json: %v", err)
	}

	// Create engine with memory storage
	engine, err := NewEngine(WithMemoryStorage())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Create detector
	detector := NewDefaultProjectDetector()

	// Test ScanForProjects
	projects, err := engine.ScanForProjects([]string{tempDir}, detector)
	if err != nil {
		t.Fatalf("ScanForProjects failed: %v", err)
	}

	// Verify results
	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}

	project := projects[0]
	if project.Name != "test-project" {
		t.Errorf("Expected project name 'test-project', got '%s'", project.Name)
	}

	if project.Path != projectDir {
		t.Errorf("Expected project path '%s', got '%s'", projectDir, project.Path)
	}

	// Check that MCP servers were detected
	if len(project.Servers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(project.Servers))
	}

	if _, exists := project.Servers["filesystem"]; !exists {
		t.Error("Expected 'filesystem' server to be detected")
	}

	// Test RegisterProject
	err = engine.RegisterProject(projectDir, *project)
	if err != nil {
		t.Fatalf("RegisterProject failed: %v", err)
	}

	// Test GetProjectConfig
	retrievedProject, err := engine.GetProjectConfig(projectDir)
	if err != nil {
		t.Fatalf("GetProjectConfig failed: %v", err)
	}

	if retrievedProject.Name != project.Name {
		t.Errorf("Retrieved project name mismatch: expected '%s', got '%s'", project.Name, retrievedProject.Name)
	}

	// Test ListProjects
	projectInfos, err := engine.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(projectInfos) != 1 {
		t.Fatalf("Expected 1 project info, got %d", len(projectInfos))
	}

	projectInfo := projectInfos[0]
	if projectInfo.Name != "test-project" {
		t.Errorf("Expected project info name 'test-project', got '%s'", projectInfo.Name)
	}

	if projectInfo.ServerCount != 1 {
		t.Errorf("Expected server count 1, got %d", projectInfo.ServerCount)
	}
}

func TestDefaultProjectDetector(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "detector-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	detector := NewDefaultProjectDetector()

	// Test with no project files
	if detector.IsProjectRoot(tempDir) {
		t.Error("Empty directory should not be detected as project root")
	}

	// Create a go.mod file
	goMod := `module test-project

go 1.21
`
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Test with go.mod
	if !detector.IsProjectRoot(tempDir) {
		t.Error("Directory with go.mod should be detected as project root")
	}

	// Test DetectProject
	project, err := detector.DetectProject(tempDir)
	if err != nil {
		t.Fatalf("DetectProject failed: %v", err)
	}

	expectedName := filepath.Base(tempDir)
	if project.Name != expectedName {
		t.Errorf("Expected project name '%s', got '%s'", expectedName, project.Name)
	}

	if project.Path != tempDir {
		t.Errorf("Expected project path '%s', got '%s'", tempDir, project.Path)
	}
}
