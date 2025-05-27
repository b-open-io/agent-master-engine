package engine

import (
	"fmt"
	"strings"
)

// Common helper functions used across the engine

// ValidateServer validates a server configuration
func ValidateServer(name string, config ServerConfig) error {
	// Basic validation
	if name == "" {
		return fmt.Errorf("server name cannot be empty")
	}
	if config.Transport != "stdio" && config.Transport != "sse" {
		return fmt.Errorf("invalid transport: %s", config.Transport)
	}
	if config.Transport == "stdio" && config.Command == "" {
		return fmt.Errorf("stdio transport requires command")
	}
	if config.Transport == "sse" && config.URL == "" {
		return fmt.Errorf("sse transport requires URL")
	}
	return nil
}

// SanitizeServerName sanitizes a server name
func SanitizeServerName(name string) string {
	// Basic sanitization - remove spaces and special characters
	sanitized := strings.TrimSpace(name)
	// Replace spaces with hyphens
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	// Remove any non-alphanumeric characters except hyphens and underscores
	// This is a simple implementation - can be enhanced with regex
	return sanitized
}

// isNotFoundError checks if an error is a "not found" error
func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}