# TODO: MCP Inputs Support

## Overview

MCP configurations (especially VS Code and GitHub format) support an "inputs" system for securely collecting sensitive values like API tokens. This is a standard feature that we should support in the future.

## Current State

- We parse and preserve `mcp.inputs` in metadata
- We preserve `${input:variable_id}` patterns in configurations
- We do NOT actually prompt for or resolve input values

## Future Implementation

### 1. Input Collection Interface

```go
type InputCollector interface {
    CollectInputs(inputs []MCPInput) (map[string]string, error)
    StoreSecurely(id string, value string) error
    RetrieveSecurely(id string) (string, error)
}
```

### 2. Input Resolution

When syncing or importing, we should:
1. Detect `${input:xxx}` patterns
2. Check if we have the input definition
3. Prompt user if value not stored
4. Securely store for future use
5. Substitute value in configuration

### 3. Security Considerations

- Input values marked with `password: true` should be:
  - Hidden during input
  - Encrypted in storage
  - Never logged or displayed
  
### 4. VS Code Compatibility

VS Code has specific conventions:
- Prompts on first server start
- Stores encrypted in VS Code's secure storage
- Supports default values
- Validates required inputs

## Example Implementation

```go
// During import/sync
if hasInputPatterns(config) {
    inputs := extractInputDefinitions(config)
    values, err := inputCollector.CollectInputs(inputs)
    if err != nil {
        return err
    }
    
    // Substitute input values
    config = substituteInputValues(config, values)
}
```

## References

- [VS Code MCP Docs](https://code.visualstudio.com/docs/copilot/chat/mcp-servers)
- [GitHub MCP Server](https://github.com/github/github-mcp-server)
- MCP Specification (inputs section)