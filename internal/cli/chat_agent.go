package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/agent"
	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/shellrun"
	"github.com/adammpkins/llama-terminal-completion/internal/tools"
)

// buildAgentRuntime assembles the workspace + tool registry shared by every
// agent frontend (one-shot `agent`, the plain REPL, and the Bubble Tea TUI):
// built-in tools, an optional rag_search tool, and any mounted MCP servers. The
// returned cleanup closes MCP sessions and must be called by the caller.
func buildAgentRuntime() (*tools.Workspace, *tools.Registry, func(), error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, func() {}, fmt.Errorf("cannot determine working directory: %w", err)
	}
	ws := &tools.Workspace{Root: cwd, AllowOutside: agentAllowOutside, Shell: cfg.Shell}
	registry := tools.DefaultRegistry(ws, nil)
	if len(agentTools) > 0 {
		registry = registry.Filter(agentTools)
	}
	if ragFlag != "" {
		registry.Register(&ragSearchTool{index: ragFlag})
	}
	cleanup, err := mountMCP(context.Background(), mcpServers, registry)
	if err != nil {
		return nil, nil, func() {}, err
	}
	return ws, registry, cleanup, nil
}

// runAgenticChat is an interactive REPL where each turn can use tools (read,
// write, run_command, web_fetch, plus any --mcp/--rag tools). It preserves
// conversation context across turns via agent.RunTurn. Tool confirmations are
// read from the same reader as the prompt to avoid double-buffering stdin.
func runAgenticChat(reader *bufio.Reader, output io.Writer) error {
	_, registry, cleanup, err := buildAgentRuntime()
	if err != nil {
		return err
	}
	defer cleanup()

	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	sys := agent.DefaultSystemPrompt
	if s := activeRoleSystem(); s != "" {
		sys = s
	}

	a := &agent.Agent{
		Client:        apiClient,
		Registry:      registry,
		MaxIterations: agentMaxIter,
		MaxTokens:     cfg.MaxTokens,
		Temperature:   cfg.Temperature,
		Emit:          plainAgentPrinter(output),
		Confirm:       replConfirm(reader, output),
		Audit:         auditLogger(),
		Stream:        cfg.Stream,
	}

	messages := []client.ChatMessage{{Role: "system", Content: sys}}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "╭─────────────────────────────────────────╮")
	fmt.Fprintln(output, "│      LlamaTerm Agentic Chat (tools)     │")
	fmt.Fprintln(output, "│   'exit' to quit · /clear · /tools      │")
	fmt.Fprintln(output, "╰─────────────────────────────────────────╯")
	fmt.Fprintf(output, "  Model: %s    Tools: %d\n\n", cfg.Model, len(registry.List()))

	for {
		fmt.Fprint(output, "You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil // EOF
		}
		input = strings.TrimSpace(input)
		switch {
		case input == "":
			continue
		case input == "exit" || input == "quit" || input == "q":
			fmt.Fprintln(output, "\nGoodbye! 👋")
			return nil
		case input == "/clear":
			messages = messages[:1]
			fmt.Fprintln(output, "Chat history cleared.")
			continue
		case input == "/tools":
			for _, t := range registry.List() {
				fmt.Fprintf(output, "  %-16s %s\n", t.Name(), t.Description())
			}
			continue
		case input == "/help":
			fmt.Fprintln(output, "Commands: exit · /clear · /tools · /help")
			continue
		}

		// Inline @file/@url references like the other commands.
		if expanded, eerr := expandReferences(input); eerr == nil {
			input = expanded
		}

		messages = append(messages, client.ChatMessage{Role: "user", Content: input})
		fmt.Fprint(output, "\nAssistant: ")
		updated, _, rerr := a.RunTurn(context.Background(), messages)
		if rerr != nil {
			fmt.Fprintf(output, "\nError: %v\n\n", rerr)
			// Drop the failed user turn so history stays consistent.
			messages = messages[:len(messages)-1]
			continue
		}
		messages = updated
		fmt.Fprintln(output) // blank line between turns (EventDone already added one)
	}
}

// plainAgentPrinter renders agent events to a plain writer (the line-based
// frontend). It is the text counterpart to the Bubble Tea frontend; both
// consume the same agent.Event stream. It tracks whether the turn produced any
// output so an empty turn (common with reasoning models that exhaust the token
// budget) is reported rather than leaving the user staring at silence.
func plainAgentPrinter(w io.Writer) func(agent.Event) {
	produced := false
	return func(e agent.Event) {
		switch e.Kind {
		case agent.EventStarted:
			produced = false
		case agent.EventToken:
			if strings.TrimSpace(e.Token) != "" {
				produced = true
			}
			fmt.Fprint(w, e.Token)
		case agent.EventToolRequested:
			produced = true
			fmt.Fprintf(w, "\033[96m→ %s %s\033[0m\n", e.Tool, e.Args)
		case agent.EventToolResult:
			if e.Declined {
				fmt.Fprint(w, "  (declined by user)\n")
			}
		case agent.EventError:
			fmt.Fprintf(w, "\nError: %v\n", e.Err)
		case agent.EventDone:
			if !produced {
				fmt.Fprint(w, "(no response — the model returned nothing. Try a higher --max-tokens or a non-reasoning model.)\n")
			}
			fmt.Fprintln(w)
		}
	}
}

// replConfirm returns a ConfirmFunc that reads y/n from the REPL's reader so it
// does not double-buffer stdin against the main prompt loop.
func replConfirm(reader *bufio.Reader, output io.Writer) agent.ConfirmFunc {
	if agentYes || !cfg.ConfirmCommands {
		return nil
	}
	return func(action, detail string) (bool, error) {
		fmt.Fprintln(output)
		printInfo(fmt.Sprintf("%s\n", action))
		if detail != "" {
			fmt.Fprintln(output, detail)
		}
		if action == "run_command" && shellrun.IsDangerous(detail) {
			printWarning("⚠️  This command may be dangerous!")
		}
		fmt.Fprint(output, "Proceed? [y/N]: ")
		resp, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}
		resp = strings.TrimSpace(strings.ToLower(resp))
		return resp == "y" || resp == "yes", nil
	}
}
