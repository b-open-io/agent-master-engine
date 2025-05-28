package daemon

import (
	engine "github.com/b-open-io/agent-master-engine"
	"github.com/b-open-io/agent-master-engine/presets"
)

// registerPresetDestinations registers all available preset destinations with the engine
func (d *Daemon) registerPresetDestinations() error {
	d.logger.Info("Registering preset destinations")
	
	// Get all available presets
	presetNames := []string{"claude", "vscode-mcp", "cursor", "generic-json"}
	
	// Also register common aliases
	aliases := map[string]string{
		"vscode": "vscode-mcp",  // Alias for backward compatibility
	}
	
	// Register each preset
	for _, name := range presetNames {
		dest, err := presets.NewDestination(name)
		if err != nil {
			d.logger.Warn("Failed to create preset destination", "preset", name, "error", err)
			continue
		}
		
		if err := d.engine.RegisterDestination(name, dest); err != nil {
			d.logger.Warn("Failed to register preset destination", "preset", name, "error", err)
			continue
		}
		
		d.logger.Debug("Registered preset destination", "name", name)
	}
	
	// Register aliases
	for alias, target := range aliases {
		dest, err := presets.NewDestination(target)
		if err != nil {
			d.logger.Warn("Failed to create preset destination for alias", "alias", alias, "target", target, "error", err)
			continue
		}
		
		if err := d.engine.RegisterDestination(alias, dest); err != nil {
			d.logger.Warn("Failed to register preset alias", "alias", alias, "error", err)
			continue
		}
		
		d.logger.Debug("Registered preset alias", "alias", alias, "target", target)
	}
	
	// Register additional presets that might not be in the CommonPresets map
	// but are referenced in the CLI
	additionalPresets := map[string]struct {
		path   string
		format string
	}{
		"windsurf": {
			path:   "~/.codeium/windsurf/mcp_config.json",
			format: "flat", // Windsurf uses flat format like Claude
		},
		"zed": {
			path:   "~/.config/zed/settings.json",
			format: "nested", // Zed uses nested format under context_servers
		},
	}
	
	// Register windsurf and zed as custom destinations
	if err := d.registerCustomDestinations(additionalPresets); err != nil {
		d.logger.Warn("Failed to register some custom destinations", "error", err)
	}
	
	// List all registered destinations
	destinations := d.engine.ListDestinations()
	d.logger.Info("Preset destinations registered", "count", len(destinations), "names", destinations)
	
	return nil
}

// registerCustomDestinations registers destinations that aren't in the presets package
func (d *Daemon) registerCustomDestinations(destinations map[string]struct {
	path   string
	format string
}) error {
	for name, info := range destinations {
		// Create appropriate destination based on format
		var dest engine.Destination
		
		switch info.format {
		case "flat":
			// Windsurf uses flat format similar to Claude
			dest = engine.NewFileDestination(name, info.path, engine.ExportFormatJSON)
			dest.(*engine.FileDestination).Transformer = &FlatFormatTransformer{}
		case "nested":
			// Zed uses nested format under context_servers
			dest = engine.NewFileDestination(name, info.path, engine.ExportFormatJSON)
			dest.(*engine.FileDestination).Transformer = &ZedFormatTransformer{}
		default:
			d.logger.Warn("Unknown format for custom destination", "name", name, "format", info.format)
			continue
		}
		
		if err := d.engine.RegisterDestination(name, dest); err != nil {
			d.logger.Warn("Failed to register custom destination", "name", name, "error", err)
			continue
		}
		
		d.logger.Debug("Registered custom destination", "name", name, "path", info.path, "format", info.format)
	}
	
	return nil
}

// FlatFormatTransformer transforms config to flat format (mcpServers at root)
type FlatFormatTransformer struct{}

func (f *FlatFormatTransformer) Transform(config *engine.Config) (interface{}, error) {
	servers := make(map[string]engine.ServerConfig)
	for name, serverWithMeta := range config.Servers {
		// Only include enabled servers
		if enabled, ok := serverWithMeta.Metadata["enabled"].(bool); !ok || enabled {
			servers[name] = serverWithMeta.ServerConfig
		}
	}
	
	return map[string]interface{}{
		"mcpServers": servers,
	}, nil
}

func (f *FlatFormatTransformer) Format() string {
	return "json"
}

// ZedFormatTransformer transforms config to Zed's format (under context_servers)
type ZedFormatTransformer struct{}

func (z *ZedFormatTransformer) Transform(config *engine.Config) (interface{}, error) {
	servers := make(map[string]engine.ServerConfig)
	for name, serverWithMeta := range config.Servers {
		// Only include enabled servers
		if enabled, ok := serverWithMeta.Metadata["enabled"].(bool); !ok || enabled {
			servers[name] = serverWithMeta.ServerConfig
		}
	}
	
	return map[string]interface{}{
		"context_servers": servers,
	}, nil
}

func (z *ZedFormatTransformer) Format() string {
	return "json"
}