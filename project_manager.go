package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScanForProjects scans the given paths for MCP projects using the provided detector
func (e *engineImpl) ScanForProjects(paths []string, detector ProjectDetector) ([]*ProjectConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var projects []*ProjectConfig
	visited := make(map[string]bool)

	for _, path := range paths {
		// Expand path
		expandedPath := expandPath(path)

		// Skip if already visited
		if visited[expandedPath] {
			continue
		}
		visited[expandedPath] = true

		// Check if path exists
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			continue
		}

		// Scan this path
		foundProjects, err := e.scanPath(expandedPath, detector)
		if err != nil {
			continue // Skip paths with errors
		}

		projects = append(projects, foundProjects...)
	}

	return projects, nil
}

// scanPath recursively scans a single path for projects
func (e *engineImpl) scanPath(path string, detector ProjectDetector) ([]*ProjectConfig, error) {
	var projects []*ProjectConfig

	// Check if this path is a project root
	if detector.IsProjectRoot(path) {
		project, err := detector.DetectProject(path)
		if err == nil && project != nil {
			projects = append(projects, project)
		}
	}

	// Get scan settings
	maxDepth := 3 // Default max depth
	excludePaths := []string{".git", "node_modules", ".vscode", ".idea", "target", "build", "dist"}

	if e.config != nil && e.config.Settings.ProjectScanning.MaxDepth > 0 {
		maxDepth = e.config.Settings.ProjectScanning.MaxDepth
	}
	if e.config != nil && len(e.config.Settings.ProjectScanning.ExcludePaths) > 0 {
		excludePaths = e.config.Settings.ProjectScanning.ExcludePaths
	}

	// Recursively scan subdirectories
	err := e.scanDirectory(path, detector, &projects, 0, maxDepth, excludePaths)
	return projects, err
}

// scanDirectory recursively scans directories up to maxDepth
func (e *engineImpl) scanDirectory(path string, detector ProjectDetector, projects *[]*ProjectConfig, currentDepth, maxDepth int, excludePaths []string) error {
	if currentDepth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()

		// Skip excluded directories
		if e.shouldExcludePath(dirName, excludePaths) {
			continue
		}

		dirPath := filepath.Join(path, dirName)

		// Check if this is a project root
		if detector.IsProjectRoot(dirPath) {
			project, err := detector.DetectProject(dirPath)
			if err == nil && project != nil {
				*projects = append(*projects, project)
			}
		}

		// Recursively scan subdirectory
		e.scanDirectory(dirPath, detector, projects, currentDepth+1, maxDepth, excludePaths)
	}

	return nil
}

// shouldExcludePath checks if a path should be excluded from scanning
func (e *engineImpl) shouldExcludePath(path string, excludePaths []string) bool {
	for _, exclude := range excludePaths {
		if strings.Contains(path, exclude) {
			return true
		}
	}
	return false
}

// RegisterProject registers a project configuration
func (e *engineImpl) RegisterProject(path string, config ProjectConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Expand path
	expandedPath := expandPath(path)

	// Ensure config has the correct path
	config.Path = expandedPath

	// Initialize projects map if needed
	if e.config.Settings.Projects == nil {
		e.config.Settings.Projects = make(map[string]ProjectConfig)
	}

	// Store project
	e.config.Settings.Projects[expandedPath] = config

	// Save configuration
	return e.saveConfigNoLock()
}

// GetProjectConfig retrieves a project configuration by path
func (e *engineImpl) GetProjectConfig(path string) (*ProjectConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Expand path
	expandedPath := expandPath(path)

	// Check if projects map exists
	if e.config.Settings.Projects == nil {
		return nil, fmt.Errorf("project not found: %s", expandedPath)
	}

	// Look up project
	project, exists := e.config.Settings.Projects[expandedPath]
	if !exists {
		return nil, fmt.Errorf("project not found: %s", expandedPath)
	}

	return &project, nil
}

// ListProjects returns information about all registered projects
func (e *engineImpl) ListProjects() ([]*ProjectInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var projects []*ProjectInfo

	// Check if projects map exists
	if e.config.Settings.Projects == nil {
		return projects, nil
	}

	// Convert projects to ProjectInfo
	for path, config := range e.config.Settings.Projects {
		serverNames := make([]string, 0, len(config.Servers))
		for name := range config.Servers {
			serverNames = append(serverNames, name)
		}

		projects = append(projects, &ProjectInfo{
			Name:        config.Name,
			Path:        path,
			ServerCount: len(config.Servers),
			Servers:     serverNames,
		})
	}

	return projects, nil
}

// DefaultProjectDetector provides a basic implementation of ProjectDetector
type DefaultProjectDetector struct {
	// ConfigFiles are the files that indicate a project root
	ConfigFiles []string
}

// NewDefaultProjectDetector creates a new default project detector
func NewDefaultProjectDetector() *DefaultProjectDetector {
	return &DefaultProjectDetector{
		ConfigFiles: []string{
			"package.json",
			"go.mod",
			"Cargo.toml",
			"pyproject.toml",
			"requirements.txt",
			"pom.xml",
			"build.gradle",
			".project",
			"mcp.json",
			"mcp-config.json",
			".mcp",
		},
	}
}

// IsProjectRoot checks if the given path is a project root
func (d *DefaultProjectDetector) IsProjectRoot(path string) bool {
	for _, configFile := range d.ConfigFiles {
		configPath := filepath.Join(path, configFile)
		if _, err := os.Stat(configPath); err == nil {
			return true
		}
	}
	return false
}

// DetectProject detects and creates a project configuration for the given path
func (d *DefaultProjectDetector) DetectProject(path string) (*ProjectConfig, error) {
	// Get project name from directory
	projectName := filepath.Base(path)

	// Create basic project config
	project := &ProjectConfig{
		Name:     projectName,
		Path:     path,
		Servers:  make(map[string]ServerWithMetadata),
		Metadata: make(map[string]interface{}),
	}

	// Try to detect MCP configuration files
	mcpConfigs := []string{"mcp.json", "mcp-config.json", ".mcp/config.json"}

	for _, configFile := range mcpConfigs {
		configPath := filepath.Join(path, configFile)
		if _, err := os.Stat(configPath); err == nil {
			// Found MCP config, try to parse it
			if err := d.parseMCPConfig(configPath, project); err == nil {
				break
			}
		}
	}

	// Add metadata about detection
	project.Metadata["detectedAt"] = time.Now()
	project.Metadata["detector"] = "DefaultProjectDetector"

	return project, nil
}

// parseMCPConfig parses an MCP configuration file and adds servers to the project
func (d *DefaultProjectDetector) parseMCPConfig(configPath string, project *ProjectConfig) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Use the existing MCP parser instead of custom parsing
	config, err := ParseMCPConfig(data)
	if err != nil {
		return err
	}

	// Add servers from parsed config to project
	for name, server := range config.Servers {
		project.Servers[name] = ServerWithMetadata{
			ServerConfig: server.ServerConfig,
			Internal: InternalMetadata{
				Enabled:         true,
				Source:          "project-scan",
				ProjectPath:     project.Path,
				ProjectSpecific: true,
				LastModified:    time.Now(),
			},
		}
	}

	// Preserve any inputs metadata
	if config.Metadata != nil {
		if inputs, ok := config.Metadata["inputs"]; ok {
			project.Metadata["inputs"] = inputs
		}
	}

	return nil
}
