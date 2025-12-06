# LlamaTerm

![LlamaTerm Logo](old-project/llama-md.png)

**AI assistant in your terminal** — works with any OpenAI-compatible API.

```bash
lt ask "How do I find large files in Linux?"
lt cmd "compress all images in this folder"
```

## Features

- 🚀 **Fast** — Single Go binary, <100ms startup
- 🔌 **Universal** — Works with Ollama, LM Studio, OpenAI, and more
- 💬 **Streaming** — Real-time response display
- 🛡️ **Safe** — Command confirmation with dangerous command detection
- ⚙️ **Configurable** — Config files, env vars, or CLI flags

## Quick Start

### Install

```bash
# Quick install (requires Go)
curl -sSL https://raw.githubusercontent.com/adammpkins/llamaterm/main/install.sh | bash

# Or build from source
git clone https://github.com/adammpkins/llamaterm.git
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
| `lt ask <question>` | Ask a question (`-c` to copy) |
| `lt cmd <description>` | Generate a shell command |
| `lt quick <description>` | Generate and run immediately |
| `lt copy <question>` | Ask and copy to clipboard |
| `lt chat` | Interactive chat session |
| `lt explain <file>` | Explain code or file contents |
| `lt fix <error>` | Get help fixing an error |
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
  --no-stream   Disable streaming output
  --max-tokens  Maximum tokens to generate
  --temperature Temperature for generation

lt cmd:
  --dry-run     Show command without running
  -y, --yes     Run without confirmation
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
