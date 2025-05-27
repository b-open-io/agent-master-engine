# MCP Configuration Formats

This document describes the different MCP configuration formats encountered in the wild and how Agent Master Engine handles them.

## Standard MCP Elements

Based on official implementations (GitHub MCP Server, VS Code), the core MCP configuration consists of:

1. **Servers object** - The actual server configurations
2. **Inputs array** (optional) - Secure input prompts for sensitive values
3. **Variable substitution** - Template patterns for environment and input variables

## Configuration Formats

### 1. VS Code / GitHub Format
Used by VS Code and official MCP servers like GitHub's.

```json
{
  "mcp": {
    "inputs": [
      {
        "type": "promptString",
        "id": "github_token",
        "description": "GitHub Personal Access Token",
        "password": true
      }
    ],
    "servers": {
      "github": {
        "command": "docker",
        "args": ["run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghcr.io/github/github-mcp-server"],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
        }
      }
    }
  }
}
```

### 2. Claude Desktop Format
Used by Anthropic's Claude Desktop application.

```json
{
  "mcpServers": {
    "filesystem": {
      "transport": "stdio",
      "command": "bunx",
      "args": ["@modelcontextprotocol/server-filesystem", "/tmp"]
    }
  }
}
```

### 3. Flat Format
Simplified format with direct servers object.

```json
{
  "servers": {
    "sqlite": {
      "transport": "stdio",
      "command": "npx",
      "args": ["@modelcontextprotocol/server-sqlite", "db.sqlite"]
    }
  }
}
```

## Variable Substitution

MCP supports two types of variable substitution:

### 1. Environment Variables
- Pattern: `${ENV_VAR_NAME}`
- Example: `${GITHUB_TOKEN}`, `${HOME}`
- Resolved from system environment

### 2. Input Variables
- Pattern: `${input:variable_id}`
- Example: `${input:github_token}`
- Resolved from the inputs array
- Prompts user on first use (in supporting tools)

## Transport Types

### stdio (Standard I/O)
- Local executable communication
- Most common for local MCP servers

### sse (Server-Sent Events)
- Remote server communication
- Requires `url` and optional `headers`

### http (Streamable HTTP)
- New in MCP 2025-03-26
- Enhanced streaming capabilities

## VS Code Specific Features

1. **Predefined Variables**
   - `${workspaceFolder}` - Current workspace path
   - `${env:VARIABLE}` - Alternative env var syntax

2. **Secure Storage**
   - Input values are encrypted and stored
   - Passwords marked with `password: true` are hidden

3. **Server Naming Conventions**
   - Use camelCase
   - Avoid whitespace and special characters
   - Be descriptive and unique

## Implementation Notes

### What Agent Master Engine Does

1. **Preserves All Formats** - Can parse all three main formats
2. **Variable Substitution** - Substitutes environment variables by default
3. **Input Preservation** - Stores inputs in metadata for future use
4. **Format Conversion** - Can transform between formats when syncing

### Future Enhancements

1. **Input System Integration** - Actually prompt for input values
2. **Format Preference** - Let destinations specify their preferred format
3. **VS Code Variables** - Support `${workspaceFolder}` and similar
4. **Validation** - Validate inputs match their usage in server configs