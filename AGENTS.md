# Agent Instructions for LlamaTerm

This file provides context for AI agents working on this codebase.

## Quick Start

```bash
make build          # Build to bin/lt
make test           # Run all tests
go test ./... -cover # Coverage report
```

## Architecture Overview

- **`cmd/lt/main.go`** - Entry point, calls `cli.Execute()`
- **`internal/cli/`** - All CLI commands and TUI
- **`internal/client/`** - OpenAI-compatible API client
- **`internal/config/`** - Viper-based configuration

## Key Patterns

### API Client (`internal/client/client.go`)

The client handles model-specific quirks:
- **Newer models** (gpt-4o, gpt-5, o1): Use `max_completion_tokens` not `max_tokens`
- **o1 and gpt-5+**: Don't support `temperature` parameter
- **gpt-5+**: Skip token limits entirely, let API use defaults

See `usesMaxCompletionTokens()` and `buildRequest()` for this logic.

### Chat TUI (`internal/cli/chat_tui.go`)

Uses Charm.sh's Bubble Tea framework. Key concepts:
- `chatModel` struct holds all state
- `Update()` handles messages, returns new model
- `View()` renders the UI string
- Commands return `tea.Cmd` for async operations

The TUI only runs in TTY mode. Non-TTY falls back to simple chat mode.

### System Prompts

Non-TUI commands (ask, fix, explain, copy) explicitly request **plain text output** (no markdown) since streaming can't render markdown in real-time. The chat TUI uses glamour for markdown rendering after the full response is received.

### Testability Patterns

Many functions use injectable dependencies for testing:
- `stdinStatFunc` - Override `os.Stdin.Stat()` checks
- `userConfigDirFunc`, `userHomeDirFunc` - Override path lookups
- `osGOOSFunc`, `execLookPathFunc` - Override clipboard detection

## Testing

Coverage targets:
- `internal/client`: >90% (currently 93.9%)
- `internal/config`: >85% (currently 88.0%)
- `internal/cli`: Lower due to TUI complexity (currently 61.7%)

Most TUI code is difficult to unit test. Focus tests on:
- Command execution logic
- API client behavior
- Configuration loading
- History management

## Common Tasks

### Adding a New Command

1. Create `internal/cli/newcmd.go`
2. Define `var newCmd = &cobra.Command{...}`
3. Add `rootCmd.AddCommand(newCmd)` in `init()` or `root.go`
4. Add tests in `internal/cli/commands_test.go`

### Adding API Features

1. Add types to `internal/client/types.go`
2. Add client method to `internal/client/client.go`
3. Add tests to `internal/client/client_test.go`

### Modifying Model Compatibility

Update these functions in `client.go`:
- `usesMaxCompletionTokens()` - Which models need new token param
- `buildRequest()` - How requests are constructed

## Gotchas

1. **Streaming vs Markdown**: Streaming output can't render markdown. Choose one.
2. **TTY Detection**: `runChat()` checks for TTY before launching TUI.
3. **Model List**: `/models` endpoint not available on all providers.
4. **Temperature**: Some models (o1, gpt-5) don't accept temperature.
5. **Token Params**: Newer models reject `max_tokens`, need `max_completion_tokens`.

## Dependencies

Key external packages:
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/glamour` - Markdown rendering
- `github.com/charmbracelet/lipgloss` - Terminal styling
