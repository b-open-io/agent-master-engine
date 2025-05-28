# Release Notes - agent-master-engine v1.0.3

## Bug Fixes

### Fixed: Nil Map Panic on Config Load
- **Issue**: When loading configuration from file, the Servers map was not initialized, causing panic on server operations
- **Fix**: Added initialization check after loading config to ensure Servers map exists
- **File**: `config_manager.go:33`

```go
// Ensure servers map is initialized
if e.config.Servers == nil {
    e.config.Servers = make(map[string]ServerConfig)
}
```

## Testing
The fix has been tested with:
- Loading config files without servers field
- Loading config files with null servers field  
- Loading config files with empty servers object
- All scenarios now work without panic

## Compatibility
This is a backward-compatible bug fix release. No API changes.

## Upgrade Instructions
```bash
go get -u github.com/b-open-io/agent-master-engine@v1.0.3
```

## Commit
```
fix: initialize Servers map after loading config from file

Previously, loading a config file without a servers field or with
servers set to null would cause a nil map panic on any server
operation. This fix ensures the Servers map is always initialized
after loading configuration.
```