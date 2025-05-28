package engine

import (
	"fmt"
	"time"
)

// Server Management functionality for engineImpl

// AddServer adds a new server configuration
func (e *engineImpl) AddServer(name string, server ServerConfig) error {
	// Validate server
	if err := ValidateServer(name, server); err != nil {
		return err
	}

	// Optional: Test server if validator supports it
	// TODO: Add server testing capability to validator interface

	e.mu.Lock()
	defer e.mu.Unlock()

	// Check for duplicates
	if _, exists := e.config.Servers[name]; exists {
		return fmt.Errorf("server %q already exists", name)
	}

	// Add with default metadata
	e.config.Servers[name] = ServerWithMetadata{
		ServerConfig: server,
		Internal: InternalMetadata{
			Enabled:      true,
			SyncTargets:  []string{"all"},
			Source:       "user",
			LastModified: time.Now(),
		},
	}

	// Save config (without lock since we already hold it)
	if err := e.saveConfigNoLock(); err != nil {
		return err
	}

	e.eventBus.emit(EventServerAdded, ConfigChange{
		Type:      "server-added",
		Name:      name,
		Timestamp: time.Now(),
		Source:    "user",
	})

	return nil
}

// UpdateServer updates an existing server configuration
func (e *engineImpl) UpdateServer(name string, server ServerConfig) error {
	// Validate server
	if err := ValidateServer(name, server); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.config.Servers[name]
	if !exists {
		return fmt.Errorf("server %q not found", name)
	}

	// Update config, preserve metadata
	existing.ServerConfig = server
	existing.Internal.LastModified = time.Now()
	e.config.Servers[name] = existing

	// Save config (without lock since we already hold it)
	if err := e.saveConfigNoLock(); err != nil {
		return err
	}

	e.eventBus.emit(EventServerUpdated, ConfigChange{
		Type:      "server-updated",
		Name:      name,
		Timestamp: time.Now(),
		Source:    "user",
	})

	return nil
}

// RemoveServer removes a server configuration
func (e *engineImpl) RemoveServer(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.config.Servers[name]; !exists {
		return fmt.Errorf("server %q not found", name)
	}

	delete(e.config.Servers, name)

	// Save config (without lock since we already hold it)
	if err := e.saveConfigNoLock(); err != nil {
		return err
	}

	e.eventBus.emit(EventServerRemoved, ConfigChange{
		Type:      "server-removed",
		Name:      name,
		Timestamp: time.Now(),
		Source:    "user",
	})

	return nil
}

// GetServer retrieves a server configuration by name
func (e *engineImpl) GetServer(name string) (*ServerWithMetadata, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	server, exists := e.config.Servers[name]
	if !exists {
		return nil, fmt.Errorf("server %q not found", name)
	}

	// Return copy
	serverCopy := server
	return &serverCopy, nil
}

// ListServers returns a list of servers matching the filter
func (e *engineImpl) ListServers(filter ServerFilter) ([]*ServerInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var servers []*ServerInfo

	for name, server := range e.config.Servers {
		// Apply filters
		if filter.Enabled != nil && server.Internal.Enabled != *filter.Enabled {
			continue
		}

		if filter.Transport != "" && server.Transport != filter.Transport {
			continue
		}

		if filter.Source != "" && server.Internal.Source != filter.Source {
			continue
		}

		// TODO: Implement other filters

		info := &ServerInfo{
			Name:            name,
			Config:          server.ServerConfig,
			Transport:       server.Transport,
			Enabled:         server.Internal.Enabled,
			SyncTargetCount: len(server.Internal.SyncTargets),
			LastModified:    server.Internal.LastModified,
			HasErrors:       server.Internal.ErrorCount > 0,
		}

		servers = append(servers, info)
	}

	return servers, nil
}

// ValidateServer validates a server configuration using the configured validator
func (e *engineImpl) ValidateServer(name string, server ServerConfig) error {
	return ValidateServer(name, server)
}

// SanitizeServerName sanitizes a server name using the configured sanitizer
func (e *engineImpl) SanitizeServerName(name string) string {
	return SanitizeServerName(name)
}

// SanitizeName sanitizes a name using the configured sanitizer
func (e *engineImpl) SanitizeName(name string) string {
	// Use sanitizer if set
	if e.sanitizer != nil {
		return e.sanitizer.Sanitize(name)
	}
	// Fall back to basic sanitization
	return SanitizeServerName(name)
}

// SetSanitizer sets the name sanitizer
func (e *engineImpl) SetSanitizer(sanitizer NameSanitizer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sanitizer = sanitizer
}

// SetValidator sets the server validator
func (e *engineImpl) SetValidator(validator ServerValidator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.validator = validator
}