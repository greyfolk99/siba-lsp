# siba-lsp

LSP server for [SIBA](https://github.com/greyfolk99/siba). Provides language intelligence for structured markdown documents.

## Architecture

siba-lsp is fully decoupled from the SIBA core engine. It calls the `siba` CLI as a subprocess and translates JSON output into LSP protocol messages.

```
Editor (VSCode, Neovim, etc.)
  ↕ LSP protocol (stdio)
siba-lsp
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
# Start LSP server (stdio)
siba-lsp

# With logging
siba-lsp --log /tmp/siba-lsp.log

# Version
siba-lsp version
```

## Features

### Diagnostics

Real-time validation on file open, change, and save. Calls `siba check --json` and converts results to LSP diagnostics.

### Custom Requests

| Method | Description |
|--------|-------------|
| `siba/render` | Render a document, returns clean markdown |

### Workspace Support

On initialization, runs `siba check --json` on the entire workspace and publishes diagnostics for all files.

## Project Structure

```
internal/
  bridge/     Subprocess bridge to siba CLI (JSON parsing)
  lsp/        LSP server (protocol types, transport, server logic)
```

## Related Projects

- [siba](https://github.com/greyfolk99/siba) — Core engine + CLI
- [siba-preview](https://github.com/greyfolk99/siba-preview) — VSCode extension

## License

MIT
