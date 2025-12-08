# LlamaTerm Website Content

## About Section

### What is LlamaTerm?

Ever wish you could look up Linux commands or ask questions and receive responses from the terminal? You probably need a paid service, an API key with paid usage, or at least an internet connection, right? 

**Not with LlamaTerm.**

We run a Large Language Model (think ChatGPT) locally, on your personal machine, and generate responses from there. No cloud required. No API fees. Just you and your AI assistant.

---

## Documentation

### Installation

#### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/adammpkins/llama-terminal-completion/main/install.sh | bash
```

This downloads the latest release and installs it to `/usr/local/bin`.

#### From Source

```bash
git clone https://github.com/adammpkins/llama-terminal-completion.git
cd llamaterm
make install
```

Requires Go 1.21+.

---

### Getting Started

LlamaTerm works out of the box with [Ollama](https://ollama.ai) running locally.

```bash
# Install Ollama (if not installed)
curl -fsSL https://ollama.ai/install.sh | sh

# Pull a model
ollama pull llama3.2

# Start using LlamaTerm
lt ask "What is the difference between TCP and UDP?"
```

---

### Core Commands

#### `lt ask <question>`

Ask any question and get an AI response.

```bash
lt ask "How do I find large files in Linux?"
lt ask "Explain recursion in simple terms"
```

Options:
- `-c, --copy` — Copy response to clipboard

#### `lt cmd <description>`

Generate a shell command from a natural language description.

```bash
lt cmd "find all .go files modified in the last week"
lt cmd "compress all images in this folder"
```

Options:
- `--dry-run` — Show command without running
- `-y, --yes` — Run immediately without confirmation

LlamaTerm detects dangerous commands (`rm -rf`, `sudo`, etc.) and warns you before execution.

#### `lt quick <description>`

Generate and run a command immediately (no confirmation).

```bash
lt quick "show disk usage"
lt quick "list running docker containers"
```

#### `lt chat`

Start an interactive chat session with conversation memory.

```bash
lt chat
lt chat --resume  # Resume a previous conversation
```

In-chat commands:
- `/help` — Show available commands
- `/model` — Switch models
- `/history` — Browse saved conversations
- `/new` — Start a new conversation
- `/clear` — Clear current chat
- `Ctrl+H` — Open conversation history
- `Ctrl+O` — Open model selector
- `Ctrl+C` — Exit (auto-saves)

#### `lt explain <file> [question]`

Analyze and explain code or file contents.

```bash
lt explain main.go
lt explain config.yaml "What does this configure?"
lt explain error.log "What went wrong?"
```

#### `lt fix <error>`

Get help fixing errors.

```bash
lt fix "Error: module not found"
npm run build 2>&1 | lt fix
```

#### `lt copy <question>`

Ask a question and copy the response to clipboard.

```bash
lt copy "Write a git commit message for adding user auth"
```

---

### Piping Input

LlamaTerm reads from stdin, letting you pipe content:

```bash
cat error.log | lt ask "What's wrong here?"
git diff | lt ask "Summarize these changes"
docker logs myapp | lt fix
```

---

### Chat History

Conversations are automatically saved and can be resumed later.

```bash
# List saved conversations
lt history list

# Resume a conversation
lt chat --resume

# Delete a conversation
lt history delete <id>

# Clear all conversations
lt history clear
```

---

### Configuration

LlamaTerm can be configured via config file, environment variables, or CLI flags.

#### Config File

Location: `~/.config/lt/config.yaml`

```yaml
base_url: http://localhost:11434/v1
model: llama3.2
api_key: ""
stream: true
max_tokens: 2048
temperature: 0.7
```

Create a config file interactively:
```bash
lt config init
```

View current config:
```bash
lt config show
```

#### Environment Variables

```bash
export LT_BASE_URL=https://api.openai.com/v1
export LT_MODEL=gpt-4o-mini
export LT_API_KEY=sk-...
```

Or use standard OpenAI environment variable:
```bash
export OPENAI_API_KEY=sk-...
```

#### CLI Flags

```bash
lt --base-url https://api.openai.com/v1 --model gpt-4o-mini ask "Hello"
```

| Flag | Description |
|------|-------------|
| `--base-url` | API base URL |
| `--api-key` | API key |
| `-m, --model` | Model to use |
| `--no-stream` | Disable streaming output |
| `--max-tokens` | Maximum tokens to generate |
| `--temperature` | Temperature for generation |

---

### Supported Providers

LlamaTerm works with any OpenAI-compatible API:

| Provider | Base URL | API Key Required |
|----------|----------|------------------|
| **Ollama** | `http://localhost:11434/v1` | No |
| **LM Studio** | `http://localhost:1234/v1` | No |
| **llama.cpp** | `http://localhost:8080/v1` | No |
| **OpenAI** | `https://api.openai.com/v1` | Yes |
| **Azure OpenAI** | Your deployment URL | Yes |
| **Anthropic (via proxy)** | Proxy URL | Yes |
| **Together AI** | `https://api.together.xyz/v1` | Yes |
| **Groq** | `https://api.groq.com/openai/v1` | Yes |

---

### Shell Completion

Enable tab completion for commands and flags:

```bash
# Bash
lt completion bash > /usr/local/etc/bash_completion.d/lt

# Zsh (add to ~/.zshrc)
source <(lt completion zsh)

# Fish
lt completion fish > ~/.config/fish/completions/lt.fish
```

---

### All Commands Reference

| Command | Description |
|---------|-------------|
| `lt ask <question>` | Ask a question |
| `lt cmd <description>` | Generate a shell command |
| `lt quick <description>` | Generate and run immediately |
| `lt copy <question>` | Ask and copy to clipboard |
| `lt chat` | Interactive chat session |
| `lt explain <file>` | Explain code or file |
| `lt fix <error>` | Help fix an error |
| `lt config show` | Show configuration |
| `lt config init` | Create config file |
| `lt history list` | List saved conversations |
| `lt history delete <id>` | Delete a conversation |
| `lt history clear` | Clear all conversations |
| `lt completion <shell>` | Generate shell completion |
| `lt version` | Show version info |
| `lt help` | Show help |
