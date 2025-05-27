package engine

import (
	"fmt"
)

// Destination and Target Management functionality for engineImpl

// RegisterDestination registers a custom destination
func (e *engineImpl) RegisterDestination(name string, dest Destination) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.destinations[name] = dest
	return nil
}

// RemoveDestination removes a registered destination
func (e *engineImpl) RemoveDestination(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.destinations[name]; !exists {
		return fmt.Errorf("destination %q not found", name)
	}

	delete(e.destinations, name)
	return nil
}

// GetDestination retrieves a destination by name
func (e *engineImpl) GetDestination(name string) (Destination, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dest, exists := e.destinations[name]
	if !exists {
		return nil, fmt.Errorf("destination %q not found", name)
	}

	return dest, nil
}

// ListDestinations returns all registered destinations
func (e *engineImpl) ListDestinations() map[string]Destination {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return copy to prevent external modification
	result := make(map[string]Destination)
	for name, dest := range e.destinations {
		result[name] = dest
	}

	return result
}

// Target Management (Legacy - use Destinations instead)

// RegisterTarget registers a legacy target configuration
func (e *engineImpl) RegisterTarget(target TargetConfig) error {
	// Legacy method - targets are now handled as destinations
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.Targets == nil {
		e.config.Targets = make(map[string]TargetConfig)
	}
	e.config.Targets[target.Name] = target

	return e.saveConfigNoLock()
}

// RemoveTarget removes a legacy target
func (e *engineImpl) RemoveTarget(name string) error {
	// Legacy method - targets are now handled as destinations
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.Targets == nil {
		return fmt.Errorf("target %q not found", name)
	}

	if _, exists := e.config.Targets[name]; !exists {
		return fmt.Errorf("target %q not found", name)
	}

	delete(e.config.Targets, name)

	return e.saveConfigNoLock()
}

// GetTarget retrieves a legacy target configuration
func (e *engineImpl) GetTarget(name string) (*TargetConfig, error) {
	// Legacy method
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config.Targets == nil {
		return nil, fmt.Errorf("target %q not found", name)
	}

	target, exists := e.config.Targets[name]
	if !exists {
		return nil, fmt.Errorf("target %q not found", name)
	}

	targetCopy := target
	return &targetCopy, nil
}

// ListTargets returns all legacy targets
func (e *engineImpl) ListTargets() ([]*TargetInfo, error) {
	// Legacy method
	e.mu.RLock()
	defer e.mu.RUnlock()

	var targets []*TargetInfo

	if e.config.Targets != nil {
		for name, target := range e.config.Targets {
			info := &TargetInfo{
				Name:       name,
				Type:       target.Type,
				Enabled:    target.Enabled,
				ConfigPath: target.ConfigPath,
			}

			// Count servers for this target
			count := 0
			for _, server := range e.config.Servers {
				if server.Internal.Enabled && e.shouldSyncToTarget(server, name) {
					count++
				}
			}
			info.ServerCount = count

			targets = append(targets, info)
		}
	}

	return targets, nil
}

// shouldSyncToTarget checks if a server should sync to a specific target
func (e *engineImpl) shouldSyncToTarget(server ServerWithMetadata, target string) bool {
	// Check exclusions first
	for _, excluded := range server.Internal.ExcludeFromTargets {
		if excluded == target {
			return false
		}
	}

	// Check inclusions
	for _, included := range server.Internal.SyncTargets {
		if included == "all" || included == target {
			return true
		}
	}

	return false
}