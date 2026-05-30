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
- **`internal/client/`** - OpenAI-compatible API client (chat, tools, embeddings, structured output)
- **`internal/config/`** - Viper-based configuration
- **`internal/shellrun/`** - Shared command execution, dangerous-command detection, confirmation (used by `cmd` and the agent's `run_command` tool — single-sourced to avoid an import cycle)
- **`internal/tools/`** - `Tool` interface, `Registry`, and built-in tools (read_file, write_file, edit_file, list_dir, run_command, web_fetch). MCP tools implement the same interface and register here.
- **`internal/agent/`** - Bounded, confirmation-gated tool-use loop (`lt agent`). Non-streamed; `Risk() >= RiskWrite` tools require confirmation.
- **`internal/roles/`** - Named system-prompt presets + `text/template` rendering. Built-ins overridable by `~/.config/lt/roles/*.yaml`.
- **`internal/rag/`** - Pure-Go (no CGO) retrieval: chunk → embed → JSON store → in-memory cosine search.
- **`internal/mcp/`** - MCP client via the official Go SDK (`github.com/modelcontextprotocol/go-sdk`) behind a small interface; adapts remote tools to `tools.Tool`.
- **`internal/fetch/`** - URL fetch + HTML→text, shared by `web_fetch` and `@url` expansion.

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

### Tool calling & the agent loop

- The client exposes `ChatCompletionWithOptions(messages, CompletionOptions{...})` (non-streamed) and `ChatCompletionStreamWithOptions(...)` which reassembles fragmented streaming `tool_calls` deltas by index. `Agent.Stream` selects which is used.
- The agent loop (`Agent.loop`, exposed via `Run` for one-shot and `RunTurn` for conversational use): send messages + tool defs → if `finish_reason == "tool_calls"`, execute each call (confirming `RiskWrite`/`RiskExec`), append `role:"tool"` results, repeat until no tool calls or `MaxIterations`. An empty final turn triggers one nudge (reasoning models sometimes stop early).
- `Agent.Audit` records every tool call; the CLI writes it to `~/.config/lt/agent-audit.jsonl` and warns if a run invokes zero tools (endpoint may not support function calling).
- New built-in tools: implement `tools.Tool` and register in `tools.DefaultRegistry`. They get a workspace boundary via `Workspace.resolve`. `edit_file` requires a unique match.

### Evented agent core + frontends

The agent **never writes to stdout**. `Agent.Emit func(Event)` emits structured `agent.Event`s (`EventStarted/Token/ToolRequested/ToolResult/Error/Done`); `Agent.Confirm` is a **blocking** callback. This lets one engine back multiple frontends:

- **Plain REPL** (`chat_agent.go`, `plainAgentPrinter`): renders events to a writer; `Confirm` reads the REPL's stdin reader.
- **Bubble Tea TUI** (`chat_agent_tui.go`): runs the agent in a worker goroutine; `Emit` sends events to the program over a buffered `events chan tea.Msg`; `Confirm` sends an `agentConfirmMsg` and blocks on `<-decision`. A confirmation blocks the **worker**, not the UI — the event loop stays live and a confirm modal answers the channel. Exactly one `waitForAgentMsg` command is outstanding at a time (single channel reader).
- **One-shot `agent`** command uses the same core with `plainAgentPrinter`.

All three build their workspace/registry via `buildAgentRuntime()`. `lt chat --agent` routes to the TUI in a real terminal, the REPL otherwise (non-TTY/tests). When adding a frontend, consume `agent.Event` — do not make the core print.

### Reliability & sessions

- `client.doRequest` retries transient failures (network errors, 429/5xx) with exponential backoff. Tests neutralize the sleep via `client.DisableRetryBackoffForTest()` (called in the cli/client `TestMain`).
- Sessions (`internal/cli/session.go`) persist named conversations under `~/.config/lt/sessions/`; `--session` on `ask`/`agent` loads prior context and saves the exchange.
- The shell widget: `lt complete` is the engine (buffer → one command, silent on error), `lt widget {bash,zsh,fish}` prints the Ctrl-G keybinding.

### Multimodal content

`client.ChatMessage` keeps `Content string` for all existing call sites, plus a `Parts []ContentPart` field excluded from default JSON (`json:"-"`). Custom `MarshalJSON`/`UnmarshalJSON` emit `content` as a string when `Parts` is empty (backward compatible with saved history) and as an array when set. **Never** change `Content` to `any` — it would break every `.Content` caller and the conversation store.

### Testability Patterns

Many functions use injectable dependencies for testing:
- `stdinStatFunc` - Override `os.Stdin.Stat()` checks
- `userConfigDirFunc`, `userHomeDirFunc` - Override path lookups
- `osGOOSFunc`, `execLookPathFunc` - Override clipboard detection
- New packages follow the same seam: package-level `var xFunc = realImpl` for FS/network (e.g. `roles.configDirFunc`, `rag.readFileFunc`, `mcp.readFileFunc`, `referenceFetchFunc`). Tests use `t.TempDir()` + `httptest` + the MCP SDK's `NewInMemoryTransports`.

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
