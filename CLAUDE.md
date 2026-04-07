# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

agcm is a terminal user interface (TUI) for the Red Hat Customer Portal API. It allows users to browse support cases, search solutions/articles, and export case data to markdown.

## Build Commands

```bash
make build          # Build to ./build/agcm
make install        # Install to $GOPATH/bin
make test           # Run all tests (go test -v ./...)
make lint           # Run golangci-lint
make fmt            # Format code (go fmt ./...)
make run            # Run the TUI directly (go run ./cmd/agcm)
```

To run a single test:
```bash
go test -v -run TestName ./path/to/package
```

## Architecture

### Package Structure

- **cmd/agcm/** - CLI entry point using Cobra
  - `cmd/root.go` - Root command, TUI launch, initializes API client and auth
  - `cmd/auth.go` - `auth login|logout|status` subcommands
  - `cmd/list.go`, `cmd/show.go`, `cmd/search.go`, `cmd/export.go` - Non-TUI CLI commands
  - `cmd/mcp.go` - `mcp` subcommand, starts MCP server over stdio
  - `cmd/completion.go` - `completion` subcommand, generates shell completions

- **internal/api/** - Red Hat Customer Portal API client
  - `client.go` - HTTP client with token refresh handling
  - `cases.go` - Case listing, details, comments, attachments
  - `products.go` - Product listing for filters
  - `types.go` - API data structures

- **internal/auth/** - OAuth token management
  - `oauth.go` - TokenManager exchanges offline tokens for access tokens via Red Hat SSO
  - `storage.go` - Persists offline token to disk

- **internal/config/** - YAML config at `~/.config/agcm/config.yaml`

- **internal/export/** - Case export to markdown
  - `export.go` - Exporter with concurrent case fetching
  - `markdown.go` - Formatter using Go templates (exports `CleanHTML` for reuse)
  - `manifest.go` - Export manifest tracking

- **internal/mcp/** - MCP (Model Context Protocol) server
  - `server.go` - Server setup, stdio transport
  - `tools.go` - Tool definitions and handlers (list_cases, get_case, search, get_solution, get_article, export_case)

- **internal/tui/** - Bubble Tea TUI
  - `app.go` - Main model, Update/View loop, state management
  - `browser.go` - (if present) browser integration
  - `components/` - Reusable UI components (caselist, casedetail, statusbar, modal, filterbar, quicksearch, etc.)
  - `styles/` - Lipgloss styling and key bindings

### Key Patterns

**TUI (Bubble Tea)**: The `Model` in `internal/tui/app.go` owns all state. Components in `components/` are sub-models with their own Update/View methods. Messages are custom types (e.g., `casesLoadedMsg`, `caseDetailLoadedMsg`) that trigger state transitions.

**API Client**: Uses functional options pattern (`WithBaseURL`, `WithTokenRefresher`, etc.). Token refresh is transparent - on 401, `TokenRefresher` is called automatically.

**Authentication Flow**: User provides offline token obtained from https://access.redhat.com/management/api. The offline token is stored locally and exchanged for short-lived access tokens as needed.

## Configuration

Config stored at `~/.config/agcm/config.yaml` (or `$XDG_CONFIG_HOME/agcm/`):
- `api.base_url` - API endpoint (default: https://api.access.redhat.com)
- `defaults.account_number` - Default account filter
- `defaults.group_number` - Default group filter
- `debug.log_file` - Custom debug log file path

## Debug Mode

Run with `--debug` flag to write API requests/responses to `~/.cache/agcm/debug.log` (respects `$XDG_CACHE_HOME`). Override with `--debug-log <path>` or `debug.log_file` in config. Set `AGCM_DEBUG_LAYOUT=1` for TUI layout debugging.
