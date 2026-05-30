// Package mcp is a thin client wrapper over the official Go MCP SDK. It
// connects to configured MCP servers (stdio or streamable HTTP), lists their
// tools, and adapts each one to the project's tools.Tool interface so they
// register into the same registry the agent loop uses.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	"github.com/adammpkins/llama-terminal-completion/internal/tools"
)

// ServerConfig describes how to reach a single MCP server.
type ServerConfig struct {
	Command   string            `yaml:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	Transport string            `yaml:"transport,omitempty"` // "stdio" (default) or "http"
	URL       string            `yaml:"url,omitempty"`
}

// Config is the top-level ~/.config/lt/mcp.yaml structure.
type Config struct {
	Servers map[string]ServerConfig `yaml:"servers"`
}

// Injectable for testing.
var (
	configDirFunc = os.UserConfigDir
	homeDirFunc   = os.UserHomeDir
	readFileFunc  = os.ReadFile
)

// ConfigPath returns the path to the MCP server config file.
func ConfigPath() string {
	if dir, err := configDirFunc(); err == nil {
		return filepath.Join(dir, "lt", "mcp.yaml")
	}
	if home, err := homeDirFunc(); err == nil {
		return filepath.Join(home, ".lt", "mcp.yaml")
	}
	return filepath.Join(".", ".lt", "mcp.yaml")
}

// LoadConfig reads the MCP config, returning an empty config if none exists.
func LoadConfig() (*Config, error) {
	data, err := readFileFunc(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Servers: map[string]ServerConfig{}}, nil
		}
		return nil, fmt.Errorf("read mcp config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse mcp config: %w", err)
	}
	if c.Servers == nil {
		c.Servers = map[string]ServerConfig{}
	}
	return &c, nil
}

// Session is a connected MCP client session for one server.
type Session struct {
	name    string
	session *sdk.ClientSession
}

// Connect establishes a session with the given server.
func Connect(ctx context.Context, name string, cfg ServerConfig) (*Session, error) {
	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, err
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "lt", Version: "dev"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server %q: %w", name, err)
	}
	return &Session{name: name, session: cs}, nil
}

func buildTransport(cfg ServerConfig) (sdk.Transport, error) {
	switch strings.ToLower(cfg.Transport) {
	case "", "stdio":
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio MCP server requires a command")
		}
		cmd := exec.Command(cfg.Command, cfg.Args...)
		if len(cfg.Env) > 0 {
			cmd.Env = os.Environ()
			for k, v := range cfg.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}
		return &sdk.CommandTransport{Command: cmd}, nil
	case "http", "streamable", "streamable-http":
		if cfg.URL == "" {
			return nil, fmt.Errorf("http MCP server requires a url")
		}
		return &sdk.StreamableClientTransport{Endpoint: cfg.URL}, nil
	default:
		return nil, fmt.Errorf("unknown MCP transport %q", cfg.Transport)
	}
}

// Close terminates the session (and the server subprocess for stdio).
func (s *Session) Close() error {
	if s.session == nil {
		return nil
	}
	return s.session.Close()
}

// Tools lists the server's tools and adapts them to tools.Tool.
func (s *Session) Tools(ctx context.Context) ([]tools.Tool, error) {
	res, err := s.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list tools from %q: %w", s.name, err)
	}
	out := make([]tools.Tool, 0, len(res.Tools))
	for _, t := range res.Tools {
		schema, _ := json.Marshal(t.InputSchema)
		out = append(out, &mcpTool{
			session:     s.session,
			name:        t.Name,
			description: t.Description,
			schema:      schema,
		})
	}
	return out, nil
}

// mcpTool adapts a remote MCP tool to the local tools.Tool interface.
type mcpTool struct {
	session     *sdk.ClientSession
	name        string
	description string
	schema      json.RawMessage
}

func (t *mcpTool) Name() string                { return t.name }
func (t *mcpTool) Description() string         { return t.description }
func (t *mcpTool) Parameters() json.RawMessage { return t.schema }

// Risk is RiskWrite for all MCP tools: their side effects are opaque to us, so
// the agent confirms them by default (override with --yes).
func (t *mcpTool) Risk() tools.RiskLevel { return tools.RiskWrite }

func (t *mcpTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var argMap map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &argMap); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	res, err := t.session.CallTool(ctx, &sdk.CallToolParams{Name: t.name, Arguments: argMap})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
			b.WriteString("\n")
		}
	}
	out := strings.TrimSpace(b.String())
	if res.IsError {
		return out, fmt.Errorf("tool %q reported an error", t.name)
	}
	if out == "" {
		out = "(tool returned no text content)"
	}
	return out, nil
}
