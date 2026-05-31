# LlamaTerm

![LlamaTerm Logo](assets/llama-md.png)

**AI assistant in your terminal** — works with any OpenAI-compatible API.

```bash
lt ask "How do I find large files in Linux?"
lt cmd "compress all images in this folder"
```

## Features

- 🚀 **Fast** — Single Go binary, <100ms startup, pure-Go (no CGO)
- 🔌 **Universal** — Works with Ollama, LM Studio, OpenAI, and more
- 💬 **Streaming** — Real-time response display
- 🤖 **Agentic** — `lt agent` (and `lt chat --agent`) run multi-step tool-use loops with live streaming
- 🧰 **Tools & MCP** — Built-in toolset plus any Model Context Protocol server
- 🎭 **Roles** — Named system-prompt presets (`-r code-reviewer`)
- 🧵 **Sessions** — Named, persistent context across invocations (`--session`)
- 📚 **RAG** — Index files/dirs/URLs and retrieve cited context for answers
- 🖼️ **Vision** — Attach images for vision-capable models
- 🧱 **Structured output** — JSON-schema-constrained responses
- 🔗 **Inline context** — `@file` and `@url` references expanded into prompts
- ⌨️ **Shell widget** — Ctrl-G command-line completion (`lt widget`)
- 🛡️ **Safe** — Command confirmation, dangerous-command detection, audit log
- 🔁 **Reliable** — Automatic retry with backoff on transient API errors
- ⚙️ **Configurable** — Config files, env vars, or CLI flags

## Quick Start

### Install

```bash
# Quick install (requires Go)
curl -sSL https://raw.githubusercontent.com/adammpkins/llama-terminal-completion/main/install.sh | bash

# Or build from source
git clone https://github.com/adammpkins/llama-terminal-completion.git
cd llamaterm
make install
```

### Shell Completion

```bash
# Bash
lt completion bash > /usr/local/etc/bash_completion.d/lt

# Zsh (add to ~/.zshrc)
source <(lt completion zsh)

# Fish
lt completion fish > ~/.config/fish/completions/lt.fish
```

### Usage

```bash
# Ask questions
lt ask "What is the difference between TCP and UDP?"

# Generate shell commands
lt cmd "find all .go files modified in the last week"

# Pipe content
cat error.log | lt ask "What's wrong here?"
```

### Configuration

LlamaTerm works out of the box with [Ollama](https://ollama.ai) running on localhost.

For other providers, configure via:

1. **Config file** (`~/.config/lt/config.yaml`):
```yaml
base_url: https://api.openai.com/v1
model: gpt-4o-mini
api_key: sk-...
```

2. **Environment variables**:
```bash
export LT_BASE_URL=https://api.openai.com/v1
export LT_MODEL=gpt-4o-mini
export LT_API_KEY=sk-...
# or
export OPENAI_API_KEY=sk-...
```

3. **CLI flags**:
```bash
lt --base-url https://api.openai.com/v1 --model gpt-4o ask "Hello"
```

## Supported Providers

| Provider | Base URL | Notes |
|----------|----------|-------|
| Ollama | `http://localhost:11434/v1` | Default, no API key needed |
| LM Studio | `http://localhost:1234/v1` | Local GUI-based |
| llama.cpp | `http://localhost:8080/v1` | llama.cpp server |
| OpenAI | `https://api.openai.com/v1` | Requires API key |
| Azure OpenAI | Custom | Requires configuration |

## Commands

| Command | Description |
|---------|-------------|
| `lt ask <question>` | Ask a question (`-c` to copy, `--image`, `--schema`, `--rag`) |
| `lt cmd <description>` | Generate a shell command |
| `lt quick <description>` | Generate and run immediately |
| `lt copy <question>` | Ask and copy to clipboard |
| `lt chat` | Interactive chat session (`--agent` for tools in chat) |
| `lt agent <task>` | Run a multi-step agentic task with tools (alias: `lt do`) |
| `lt explain <file>` | Explain code or file contents |
| `lt fix <error>` | Get help fixing an error |
| `lt role list\|show\|add` | Manage roles (system-prompt presets) |
| `lt session list\|show\|rm` | Manage named sessions (persistent context) |
| `lt rag add\|list\|search\|rm` | Build and query embeddings-backed indexes |
| `lt mcp list\|tools` | Manage Model Context Protocol servers |
| `lt complete <buffer>` | Complete a command line into a shell command |
| `lt widget bash\|zsh\|fish` | Print a Ctrl-G command-line completion keybinding |
| `lt config show` | Show current configuration |
| `lt config init` | Create config file |
| `lt history list` | View saved conversations |
| `lt version` | Show version info |

### Command Flags

```
Global:
  --base-url    API base URL
  --api-key     API key
  -m, --model   Model to use
  -r, --role    Use a named role (system-prompt preset)
  --no-stream   Disable streaming output
  --max-tokens  Maximum tokens to generate
  --temperature Temperature for generation

lt cmd:
  --dry-run     Show command without running
  -y, --yes     Run without confirmation

lt ask:
  --image       Attach image file(s) or URL(s) for vision models
  --schema      Path to a JSON Schema; returns structured JSON output
  --rag         Augment the prompt with context from a RAG index
  --session     Reuse and extend a named session for context

lt agent:
  -y, --yes         Skip confirmation prompts for write/exec tools
  --max-iterations  Maximum model round-trips (default 12)
  --allow-outside   Allow file access outside the working directory
  --tools           Restrict to a comma-separated subset of tools
  --mcp             Mount tools from configured MCP servers
  --rag             Expose a rag_search tool backed by an index
  --session         Reuse and extend a named session for context

lt chat:
  --agent           Enable tools in chat (agentic REPL)
  -R, --resume      Resume a previous conversation
```

### Roles

Roles are named system-prompt presets, selectable on any command with the global `-r`/`--role` flag:

```bash
lt -r code-reviewer ask "review this diff" < changes.diff
git diff | lt -r commit-message ask "write a commit message"
lt role add sql "You are a senior SQL expert. Answer with portable SQL."
```

Built-in roles: `shell`, `code-reviewer`, `commit-message`. User roles live in `~/.config/lt/roles/*.yaml` and override built-ins of the same name.

### Agent (tool use)

`lt agent` runs a bounded, confirmation-gated loop where the model can read/write files, run shell commands, and fetch URLs:

```bash
lt agent "create hello.txt with 'hi', then read it back"
lt agent --yes "list the Go files under internal and summarize them"
lt do --tools read_file,run_command "find and explain the failing test"
```

Write and command-execution tools prompt for confirmation (reusing the dangerous-command detection) unless `--yes` is given. File access is restricted to the working directory unless `--allow-outside` is set. Every tool invocation is recorded to an audit log at `~/.config/lt/agent-audit.jsonl`.

> **Tool calling requires a function-calling-capable endpoint** (e.g. local Ollama models like `qwen3.5`/`gpt-oss`, or OpenAI). If the endpoint ignores the `tools` parameter, `lt` warns that no tools were called rather than letting the model pretend to act.

### Tools in chat

`lt chat --agent` is an interactive REPL where each turn can use tools, with context preserved across turns:

```bash
lt chat --agent              # read/write files, run commands, fetch URLs
lt chat --agent --yes        # skip confirmations
lt chat --agent --mcp fs     # also mount an MCP server's tools
```

### Sessions

Sessions keep context across separate invocations — pass `--session <name>` to `ask` or `agent`:

```bash
lt ask --session debug "what does errno 13 mean?"
lt ask --session debug "and how do I fix it?"   # remembers the above
lt session list
```

### Shell completion widget

Bind a key (Ctrl-G) that rewrites your current command line into a shell command via AI:

```bash
# zsh (~/.zshrc)
source <(lt widget zsh)
# bash (~/.bashrc)
source <(lt widget bash)
# fish
lt widget fish | source
```

Then type a partial command or a description and press **Ctrl-G**:

```
$ list pdfs changed this week<Ctrl-G>
$ find . -name "*.pdf" -mtime -7
```

### Inline context with `@`

Reference files and URLs directly in a prompt; they're fetched and inlined:

```bash
lt ask "summarize @README.md and compare to @https://example.com/spec"
```

Use `\@` to write a literal `@`.

### Retrieval (RAG)

Index local files, directories, or URLs and use them as context:

```bash
lt rag add ./docs --index handbook
lt rag search "how do I configure the proxy" --index handbook
lt ask --rag handbook "what's the retry policy?"   # inlines top matches
lt agent --rag handbook "update the proxy docs"     # exposes a rag_search tool
```

Embeddings use the model set by `embedding_model` (default `nomic-embed-text`). The store is a pure-Go, CGO-free flat file.

### Structured output

Force JSON output matching a schema (with an automatic prompt-based fallback for models lacking native support):

```bash
lt ask --schema person.json "Jane Doe is 31 and lives in Berlin"
```

### Vision

Attach images to a prompt for vision-capable models:

```bash
lt ask --image diagram.png "explain this architecture"
lt ask --image https://example.com/chart.png "what trend does this show?"
```

### MCP servers

Connect to [Model Context Protocol](https://modelcontextprotocol.io) servers and expose their tools to the agent. Configure them in `~/.config/lt/mcp.yaml`:

```yaml
servers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/path"]
```

```bash
lt mcp list
lt mcp tools filesystem
lt agent --mcp filesystem "organize my notes directory"
```

## More Examples

```bash
# Interactive chat with memory
lt chat

# Analyze a file
lt explain main.go
lt explain config.yaml "What does this configure?"

# Debug errors
lt fix "Error: module not found"
npm run build 2>&1 | lt fix
```

## Development

```bash
# Download dependencies
make deps

# Build
make build

# Run tests
make test

# Run
./bin/lt ask "Hello"
```

## License

MIT License
