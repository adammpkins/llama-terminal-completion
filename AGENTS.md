# LlamaTerm - Go Rewrite Project Plan

## Overview

This document outlines the plan to rewrite **LlamaTerm** from Python to Go, transforming it from a llama.cpp wrapper into a flexible OpenAI-compatible API client that can work with any LLM endpoint (local or remote).

## Current State Analysis

### Old Project Summary
The original project (`old-project/`) is a Python CLI tool that:
- Calls `llama.cpp` directly via subprocess
- Writes prompts to files and parses stdout output
- Supports three modes: Questions (`-q`), Commands (`-c`), and Wiki lookups (`-w`)
- Uses `.env` files for configuration (model paths, tokens, prompts, etc.)
- Has hardcoded prompt templates for command generation and Q&A

### Pain Points & Improvement Opportunities

| Issue | Impact | Improvement |
|-------|--------|-------------|
| Tight coupling to llama.cpp | Cannot use remote APIs, other local servers | Support any OpenAI-compatible API |
| Subprocess stdout parsing | Fragile, timing-dependent, error-prone | Use proper HTTP/streaming API calls |
| Python with many dependencies | Slower startup, requires Python runtime | Single static Go binary |
| Hardcoded prompt templates | Inflexible, model-specific | Configurable system prompts |
| No streaming output | User waits for full response | Real-time streaming to terminal |
| Basic error handling | Crashes, unclear error messages | Robust error handling, helpful messages |
| No conversation history | Each query is standalone | Optional conversation context |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI Layer                           │
│  (cobra/viper for commands, flags, env vars, config file)  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Core Service                          │
│  - Mode handlers (ask, cmd, chat, wiki)                    │
│  - Prompt templating                                        │
│  - Response processing                                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     OpenAI API Client                       │
│  - HTTP client with streaming support                       │
│  - Compatible with: OpenAI, Ollama, LM Studio,             │
│    llama.cpp server, vLLM, LocalAI, etc.                   │
└─────────────────────────────────────────────────────────────┘
```

---

## Proposed Features

### Core Functionality

1. **OpenAI-Compatible API Client**
   - Works with any OpenAI-compatible endpoint
   - Supports `/v1/chat/completions` and `/v1/completions`
   - Streaming responses by default
   - Configurable base URL, API key, model name

2. **Command Modes**
   - `lt ask "question"` - General Q&A
   - `lt cmd "what to do"` - Generate shell commands (with confirmation)
   - `lt chat` - Interactive conversation mode
   - `lt explain <file>` - Explain code/file contents (NEW)
   - `lt fix "error message"` - Suggest fixes for errors (NEW)

3. **Configuration**
   - Config file (`~/.config/lt/config.yaml` or `~/.ltrc`)
   - Environment variables (OPENAI_API_KEY, LLM_BASE_URL, etc.)
   - CLI flags for overrides
   - Multiple profiles for different endpoints/models

### Quality of Life

4. **Shell Integration**
   - Pipe support: `cat file.txt | lt ask "summarize this"`
   - Output to clipboard option
   - Shell completion scripts (bash, zsh, fish)

5. **Safety & Confirmation**
   - Command mode always asks before execution
   - Dry-run mode for commands
   - Dangerous command detection/warnings

6. **Response Handling**
   - Real-time streaming to terminal
   - Markdown rendering option
   - Code block syntax highlighting
   - Copy code blocks to clipboard

---

## Technical Decisions

### Dependencies

| Purpose | Library | Rationale |
|---------|---------|-----------|
| CLI framework | `cobra` | Industry standard, subcommands, completions |
| Config | `viper` | Env vars, config files, flags in one |
| HTTP client | `net/http` (stdlib) | No external deps for core functionality |
| Streaming | SSE parsing | Custom, lightweight implementation |
| Terminal UI | `charmbracelet/lipgloss`, `bubbletea` (optional) | Beautiful terminal output |
| Markdown | `glamour` | Terminal markdown rendering |

### Project Structure

```
llama-terminal-completion/
├── cmd/
│   └── lt/
│       └── main.go              # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go              # Root command
│   │   ├── ask.go               # Ask command
│   │   ├── cmd.go               # Command generation
│   │   ├── chat.go              # Interactive chat
│   │   └── config.go            # Config command
│   ├── client/
│   │   ├── client.go            # OpenAI API client
│   │   ├── streaming.go         # SSE stream handling
│   │   └── types.go             # API types
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── prompt/
│   │   └── templates.go         # Prompt templates
│   └── shell/
│       └── executor.go          # Shell command execution
├── config.example.yaml          # Example configuration
├── go.mod
├── go.sum
├── Makefile                     # Build, install, release
└── README.md
```

---

## Configuration Design

### Config File Example (`~/.config/lt/config.yaml`)

```yaml
# Default endpoint (can be overridden per command)
default_profile: local

profiles:
  local:
    base_url: http://localhost:11434/v1    # Ollama
    model: llama3.2
    api_key: ""                             # Optional for local

  openai:
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
    api_key: ${OPENAI_API_KEY}              # Environment variable

  anthropic:
    base_url: https://api.anthropic.com/v1
    model: claude-3-sonnet
    api_key: ${ANTHROPIC_API_KEY}

# Behavior settings
settings:
  stream: true
  confirm_commands: true
  shell: /bin/zsh
  max_tokens: 1024
  temperature: 0.7
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LLM_BASE_URL` | API base URL | `http://localhost:11434/v1` |
| `LLM_API_KEY` | API key | (empty) |
| `LLM_MODEL` | Model name | `llama3.2` |
| `LLM_PROFILE` | Config profile to use | `default` |
| `OPENAI_API_KEY` | OpenAI API key | - |

---

## Implementation Phases

### Phase 1: Core Foundation ✅
- [x] Project setup with Go modules
- [x] Basic CLI structure with Cobra
- [x] Configuration loading (Viper)
- [x] OpenAI-compatible HTTP client
- [x] Streaming response handling
- [x] Basic `ask` command

### Phase 2: Feature Parity ✅
- [x] `cmd` command with confirmation flow
- [x] Shell command execution
- [x] Multiple API endpoint support
- [x] Environment variable configuration
- [x] Colored terminal output

### Phase 3: Enhancements ✅
- [x] `chat` interactive mode
- [x] Pipe/stdin support
- [x] Shell completion scripts
- [ ] Markdown rendering
- [x] Config management (`lt config`)

### Phase 4: Polish ✅
- [x] `explain` and `fix` commands
- [x] Dangerous command detection
- [ ] Clipboard integration
- [x] Installation scripts
- [ ] Homebrew formula
- [x] Comprehensive documentation
- [x] Chat history (`lt history`)

---

## Compatibility Matrix

The rewritten tool will work with:

| Provider | Endpoint | Notes |
|----------|----------|-------|
| **Ollama** | `http://localhost:11434/v1` | Popular local option |
| **LM Studio** | `http://localhost:1234/v1` | GUI-based local |
| **llama.cpp server** | `http://localhost:8080/v1` | Original llama.cpp HTTP server |
| **vLLM** | Custom | High-performance serving |
| **LocalAI** | Custom | Drop-in OpenAI replacement |
| **OpenAI** | `https://api.openai.com/v1` | Requires API key |
| **Azure OpenAI** | Custom | Requires configuration |
| **Anthropic** | Via proxy | Needs OpenAI-compatible proxy |

---

## Success Criteria

1. **Single binary** - No runtime dependencies, just `lt`
2. **< 100ms startup** - Fast CLI experience  
3. **Works offline** - With local LLM servers
4. **Zero config for Ollama** - Just works with defaults
5. **Drop-in replacement** - Same core functionality as original LlamaTerm
6. **Streaming first** - Real-time response display
7. **Cross-platform** - macOS, Linux, Windows builds

---

## Migration Path

Users can migrate from the original Python version of LlamaTerm:

```bash
# Old way
python ask_llama.py -q "How does photosynthesis work?"
python ask_llama.py -c "list files in current directory"

# New way  
lt ask "How does photosynthesis work?"
lt cmd "list files in current directory"
```

The new LlamaTerm will be backwards-compatible in spirit but not require llama.cpp installation or Python.
