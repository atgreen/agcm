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


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
