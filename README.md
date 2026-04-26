# siba-lsp

LSP + MCP server for [SIBA](https://github.com/greyfolk99/siba). Provides language intelligence for structured markdown documents.

## Architecture

siba-lsp directly imports the siba core engine (`siba/pkg/*`). In-process parsing and validation — no subprocess, no JSON bridge.

```
Editor / AI Agent
  ↕ LSP protocol (stdio)    OR    MCP protocol (stdio)
siba-lsp                          siba-lsp --mcp
  ↓ direct import
siba/pkg (parser, scope, render, validate)
```

## Install

Requires `siba` CLI to be installed and available in PATH.

```bash
go install github.com/greyfolk99/siba-lsp/cmd/siba-lsp@latest
```

Or build from source:

```bash
git clone https://github.com/greyfolk99/siba-lsp.git
cd siba-lsp
go build -o siba-lsp ./cmd/siba-lsp
```

## Usage

```bash
# Start LSP server (default, stdio)
siba-lsp

# Start MCP server (stdio)
siba-lsp --mcp

# With logging
siba-lsp --log /tmp/siba-lsp.log
siba-lsp --mcp --log /tmp/siba-mcp.log

# Specify working directory
siba-lsp --mcp --workdir /path/to/project

# Version
siba-lsp version
```

## LSP Features

### Diagnostics

Real-time validation on file open, change, and save. Calls `siba check --json` and converts results to LSP diagnostics.

### Custom Requests

| Method | Description |
|--------|-------------|
| `siba/render` | Render a document, returns clean markdown |

### Workspace Support

On initialization, runs `siba check --json` on the entire workspace and publishes diagnostics for all files.

## MCP Features

### Tools

| Tool | Description |
|------|-------------|
| `siba_check` | Check a file or workspace for errors. Returns diagnostics as JSON. |
| `siba_cat` | Render a document (streaming). Use `file.md#section` for specific section. |
| `siba_ls` | List all documents/templates in workspace, or symbols in a file. |
| `siba_tree` | Show heading tree for a file. |
| `siba_find` | Search workspace for a keyword. |
| `siba_help` | Show SIBA syntax reference. Topics: directives, variables, templates, references, control, packages, types. |

### MCP Configuration Example

```json
{
  "mcpServers": {
    "siba": {
      "command": "siba-lsp",
      "args": ["--mcp", "--workdir", "/path/to/project"]
    }
  }
}
```

## Project Structure

```
internal/
  bridge/     In-process bridge to siba/pkg (direct import, no subprocess)
  lsp/        LSP server (protocol types, transport, server logic)
  mcp/        MCP server (tool definitions, help text, protocol)
```

## Related Projects

- [siba](https://github.com/greyfolk99/siba) — Core engine + CLI
- [siba-viewer](https://github.com/greyfolk99/siba-viewer) — VSCode extension

## License

MIT
