# siba-lsp

LSP + MCP server for [SIBA](https://github.com/greyfolk99/siba). Provides language intelligence for structured markdown documents.

## Architecture

siba-lsp is fully decoupled from the SIBA core engine. It calls the `siba` CLI as a subprocess and translates JSON output into LSP or MCP protocol messages.

```
Editor / AI Agent
  ↕ LSP protocol (stdio)    OR    MCP protocol (stdio)
siba-lsp                          siba-lsp --mcp
  ↕ subprocess (JSON)
siba CLI (--json mode)
```

No shared Go packages. No import dependencies. Communication is purely through JSON over subprocess pipes.

## Install

Requires `siba` CLI to be installed and available in PATH.

```bash
go install github.com/hjseo/siba-lsp/cmd/siba-lsp@latest
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
| `siba_render` | Render a document. Returns clean markdown. |
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
  bridge/     Subprocess bridge to siba CLI (JSON parsing)
  lsp/        LSP server (protocol types, transport, server logic)
  mcp/        MCP server (tool definitions, help text, protocol)
```

## Related Projects

- [siba](https://github.com/greyfolk99/siba) — Core engine + CLI
- [siba-viewer](https://github.com/greyfolk99/siba-viewer) — VSCode extension

## License

MIT
