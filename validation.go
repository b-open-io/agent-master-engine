package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultValidator provides basic MCP validation rules
type DefaultValidator struct {
	namePattern   *regexp.Regexp
	maxNameLength int
}

// NewDefaultValidator creates a validator with standard MCP rules
func NewDefaultValidator() *DefaultValidator {
	return &DefaultValidator{
		namePattern:   regexp.MustCompile(`^[^\x00-\x1F\x7F]+$`), // No control characters
		maxNameLength: 256,
	}
}

// ValidateName validates a server name
func (v *DefaultValidator) ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	if len(name) > v.maxNameLength {
		return fmt.Errorf("server name too long (max %d characters)", v.maxNameLength)
	}

	if !v.namePattern.MatchString(name) {
		return fmt.Errorf("server name contains invalid characters")
	}

	return nil
}

// ValidateConfig validates a server configuration
func (v *DefaultValidator) ValidateConfig(config ServerConfig) error {
	// Validate transport
	if config.Transport != "stdio" && config.Transport != "sse" {
		return fmt.Errorf("invalid transport %q: must be 'stdio' or 'sse'", config.Transport)
	}

	// Transport-specific validation
	switch config.Transport {
	case "stdio":
		if err := v.validateStdioConfig(config); err != nil {
			return fmt.Errorf("stdio validation failed: %w", err)
		}
	case "sse":
		if err := v.validateSSEConfig(config); err != nil {
			return fmt.Errorf("sse validation failed: %w", err)
		}
	}

	// Validate environment variables format
	for key, value := range config.Env {
		if err := v.validateEnvVar(key, value); err != nil {
			return fmt.Errorf("invalid environment variable %q: %w", key, err)
		}
	}

	return nil
}

// validateStdioConfig validates stdio transport specific fields
func (v *DefaultValidator) validateStdioConfig(config ServerConfig) error {
	// Command is required
	if config.Command == "" {
		return fmt.Errorf("command is required for stdio transport")
	}

	// SSE fields should not be present
	if config.URL != "" {
		return fmt.Errorf("url field is not allowed for stdio transport")
	}

	// Headers validation would go here if ServerConfig had headers

	return nil
}

// validateSSEConfig validates SSE transport specific fields
func (v *DefaultValidator) validateSSEConfig(config ServerConfig) error {
	// URL is required
	if config.URL == "" {
		return fmt.Errorf("url is required for sse transport")
	}

	// Validate URL format
	if !strings.HasPrefix(config.URL, "http://") && !strings.HasPrefix(config.URL, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}

	// stdio fields should not be present
	if config.Command != "" {
		return fmt.Errorf("command field is not allowed for sse transport")
	}

	if len(config.Args) > 0 {
		return fmt.Errorf("args field is not allowed for sse transport")
	}

	if len(config.Env) > 0 {
		return fmt.Errorf("env field is not allowed for sse transport")
	}

	return nil
}

// validateEnvVar validates environment variable format
func (v *DefaultValidator) validateEnvVar(key, value string) error {
	// Key must be valid identifier
	if !isValidEnvKey(key) {
		return fmt.Errorf("invalid key format")
	}

	// Value can be literal or reference
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		// Validate reference format
		ref := value[2 : len(value)-1]
		if !isValidEnvKey(ref) {
			return fmt.Errorf("invalid reference format")
		}
	}

	return nil
}

// NoOpSanitizer doesn't modify names
type NoOpSanitizer struct{}

// Sanitize returns the name unchanged
func (n *NoOpSanitizer) Sanitize(name string) string {
	return name
}

// NeedsSanitization always returns false
func (n *NoOpSanitizer) NeedsSanitization(name string) bool {
	return false
}

// PatternValidator validates names against a regex pattern
type PatternValidator struct {
	pattern      *regexp.Regexp
	maxLength    int
	errorMessage string
}

// NewPatternValidator creates a validator with a custom pattern
func NewPatternValidator(pattern string, maxLength int) (*PatternValidator, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	return &PatternValidator{
		pattern:      re,
		maxLength:    maxLength,
		errorMessage: fmt.Sprintf("name must match pattern %s", pattern),
	}, nil
}

// ValidateName validates against the pattern
func (p *PatternValidator) ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if p.maxLength > 0 && len(name) > p.maxLength {
		return fmt.Errorf("name too long (max %d characters)", p.maxLength)
	}

	if !p.pattern.MatchString(name) {
		return fmt.Errorf(p.errorMessage)
	}

	return nil
}

// ValidateConfig delegates to default validator
func (p *PatternValidator) ValidateConfig(config ServerConfig) error {
	return NewDefaultValidator().ValidateConfig(config)
}

// ReplacementSanitizer sanitizes names by replacing characters
type ReplacementSanitizer struct {
	replacements map[string]string
	removeChars  string
	maxLength    int
}

// NewReplacementSanitizer creates a sanitizer with custom replacements
func NewReplacementSanitizer(replacements map[string]string, removeChars string, maxLength int) *ReplacementSanitizer {
	return &ReplacementSanitizer{
		replacements: replacements,
		removeChars:  removeChars,
		maxLength:    maxLength,
	}
}

// Sanitize applies replacements and removals
func (r *ReplacementSanitizer) Sanitize(name string) string {
	// Apply replacements
	for old, new := range r.replacements {
		name = strings.ReplaceAll(name, old, new)
	}

	// Remove characters
	if r.removeChars != "" {
		for _, char := range r.removeChars {
			name = strings.ReplaceAll(name, string(char), "")
		}
	}

	// Cleanup multiple consecutive hyphens/underscores
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	name = regexp.MustCompile(`_+`).ReplaceAllString(name, "_")

	// Trim
	name = strings.Trim(name, "-_")

	// Ensure not empty
	if name == "" {
		name = "unnamed"
	}

	// Truncate if needed
	if r.maxLength > 0 && len(name) > r.maxLength {
		name = name[:r.maxLength]
	}

	return name
}

// NeedsSanitization checks if sanitization would change the name
func (r *ReplacementSanitizer) NeedsSanitization(name string) bool {
	return name != r.Sanitize(name)
}

// Helper function
func isValidEnvKey(s string) bool {
	if s == "" {
		return false
	}

	// Must start with letter or underscore
	if !isLetter(rune(s[0])) && s[0] != '_' {
		return false
	}

	// Rest must be alphanumeric or underscore
	for i := 1; i < len(s); i++ {
		if !isLetter(rune(s[i])) && !isDigit(rune(s[i])) && s[i] != '_' {
			return false
		}
	}

	return true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// HandleDuplicateName generates unique name by appending number
func HandleDuplicateName(baseName string, existingNames map[string]bool, maxLength int) string {
	name := baseName
	counter := 2

	for existingNames[name] {
		name = fmt.Sprintf("%s-%d", baseName, counter)
		counter++

		// Ensure we don't exceed length limit
		if maxLength > 0 && len(name) > maxLength {
			// Truncate base name to make room for suffix
			maxBase := maxLength - len(fmt.Sprintf("-%d", counter))
			if maxBase < 1 {
				maxBase = 1
			}
			name = fmt.Sprintf("%s-%d", baseName[:maxBase], counter)
		}
	}

	return name
}
