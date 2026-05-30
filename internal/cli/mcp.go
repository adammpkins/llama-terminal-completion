package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/adammpkins/llama-terminal-completion/internal/mcp"
	"github.com/adammpkins/llama-terminal-completion/internal/tools"
	"github.com/spf13/cobra"
)

// mcpServers is the value of the --mcp flag on agent/chat (repeatable).
var mcpServers []string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage Model Context Protocol (MCP) servers",
	Long: fmt.Sprintf(`Connect to MCP servers to expose their tools to the agent.

Configure servers in %s, for example:

  servers:
    filesystem:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path"]
    remote:
      transport: http
      url: https://example.com/mcp

Then: lt agent --mcp filesystem "do something with my files"`, mcp.ConfigPath()),
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := mcp.LoadConfig()
		if err != nil {
			return err
		}
		if len(cfg.Servers) == 0 {
			fmt.Printf("No MCP servers configured. Edit %s to add one.\n", mcp.ConfigPath())
			return nil
		}
		names := make([]string, 0, len(cfg.Servers))
		for n := range cfg.Servers {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			s := cfg.Servers[n]
			transport := s.Transport
			if transport == "" {
				transport = "stdio"
			}
			target := s.Command
			if transport != "stdio" {
				target = s.URL
			}
			fmt.Printf("%-16s %-8s %s\n", n, transport, target)
		}
		return nil
	},
}

var mcpToolsCmd = &cobra.Command{
	Use:   "tools <server>",
	Short: "List the tools a configured MCP server exposes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := mcp.LoadConfig()
		if err != nil {
			return err
		}
		sc, ok := cfg.Servers[args[0]]
		if !ok {
			return fmt.Errorf("MCP server %q not found in %s", args[0], mcp.ConfigPath())
		}
		ctx := context.Background()
		session, err := mcp.Connect(ctx, args[0], sc)
		if err != nil {
			return err
		}
		defer func() { _ = session.Close() }()

		ts, err := session.Tools(ctx)
		if err != nil {
			return err
		}
		if len(ts) == 0 {
			fmt.Println("(server exposes no tools)")
			return nil
		}
		for _, t := range ts {
			fmt.Printf("%-24s %s\n", t.Name(), t.Description())
		}
		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpListCmd, mcpToolsCmd)
	rootCmd.AddCommand(mcpCmd)

	agentCmd.Flags().StringSliceVar(&mcpServers, "mcp", nil, "Mount tools from these configured MCP servers")
}

// mountMCP connects to the named MCP servers and registers their tools into
// reg. It returns a cleanup function that closes all opened sessions.
func mountMCP(ctx context.Context, names []string, reg *tools.Registry) (func(), error) {
	if len(names) == 0 {
		return func() {}, nil
	}
	cfg, err := mcp.LoadConfig()
	if err != nil {
		return func() {}, err
	}
	var sessions []*mcp.Session
	cleanup := func() {
		for _, s := range sessions {
			_ = s.Close()
		}
	}
	for _, name := range names {
		sc, ok := cfg.Servers[name]
		if !ok {
			cleanup()
			return func() {}, fmt.Errorf("MCP server %q not found in %s", name, mcp.ConfigPath())
		}
		session, err := mcp.Connect(ctx, name, sc)
		if err != nil {
			cleanup()
			return func() {}, err
		}
		sessions = append(sessions, session)
		mcpTools, err := session.Tools(ctx)
		if err != nil {
			cleanup()
			return func() {}, err
		}
		for _, t := range mcpTools {
			reg.Register(t)
		}
	}
	return cleanup, nil
}
