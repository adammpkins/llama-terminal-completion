package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adammpkins/llama-terminal-completion/internal/agent"
	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/shellrun"
	"github.com/spf13/cobra"
)

var (
	agentMaxIter      int
	agentYes          bool
	agentAllowOutside bool
	agentTools        []string
)

var agentCmd = &cobra.Command{
	Use:     "agent [task]",
	Aliases: []string{"do"},
	Short:   "Run a multi-step agentic task using tools",
	Long: `Run an agentic loop where the model can read and write files, run shell
commands, and fetch URLs to accomplish a task.

Write and command-execution tools ask for confirmation before running (unless
--yes is given or confirm_commands is disabled). File access is restricted to
the current working directory unless --allow-outside is set.

Examples:
  lt agent "create a hello.txt file containing 'hi', then read it back"
  lt agent --yes "list the Go files under internal and summarize them"
  lt do "fix the failing test in cmd.go"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAgent,
}

func init() {
	agentCmd.Flags().IntVar(&agentMaxIter, "max-iterations", agent.DefaultMaxIterations, "Maximum model round-trips")
	agentCmd.Flags().BoolVarP(&agentYes, "yes", "y", false, "Skip confirmation prompts for write/exec tools")
	agentCmd.Flags().BoolVar(&agentAllowOutside, "allow-outside", false, "Allow file access outside the working directory")
	agentCmd.Flags().StringSliceVar(&agentTools, "tools", nil, "Restrict to a comma-separated subset of tools")
	agentCmd.Flags().StringVar(&sessionName, "session", "", "Reuse and extend a named session for context")
	rootCmd.AddCommand(agentCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	task, err := getQuestion(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(task) == "" {
		return fmt.Errorf("no task provided")
	}
	if task, err = expandReferences(task); err != nil {
		return err
	}

	_, registry, cleanup, err := buildAgentRuntime()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(agentTools) > 0 && len(registry.List()) == 0 {
		return fmt.Errorf("no known tools matched --tools %v", agentTools)
	}
	ctx := context.Background()

	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	// Count tool invocations so we can warn if the endpoint silently ignored
	// the tools parameter (i.e. doesn't support function calling).
	logAudit := auditLogger()
	var toolCalls int
	audit := func(e agent.AuditEntry) {
		toolCalls++
		logAudit(e)
	}

	a := &agent.Agent{
		Client:        apiClient,
		Registry:      registry,
		MaxIterations: agentMaxIter,
		MaxTokens:     cfg.MaxTokens,
		Temperature:   cfg.Temperature,
		SystemPrompt:  agentSystemPrompt(),
		Emit:          plainAgentPrinter(os.Stdout),
		Confirm:       agentConfirmFunc(),
		Audit:         audit,
		Stream:        cfg.Stream,
	}

	var result string
	if sessionName != "" {
		// Carry prior session context and persist the full exchange.
		prior, lerr := loadSession(sessionName)
		if lerr != nil {
			return lerr
		}
		if len(prior) == 0 {
			prior = []client.ChatMessage{{Role: "system", Content: agentSystemPrompt()}}
		}
		prior = append(prior, client.ChatMessage{Role: "user", Content: task})
		updated, out, rerr := a.RunTurn(ctx, prior)
		if rerr != nil {
			return rerr
		}
		result = out
		if serr := saveSession(sessionName, cfg.Model, updated); serr != nil {
			printWarning("Could not save session: %v", serr)
		}
	} else {
		out, rerr := a.Run(ctx, task)
		if rerr != nil {
			return rerr
		}
		result = out
	}

	if toolCalls == 0 {
		printWarning("\n⚠️  The model did not call any tools. If your endpoint doesn't support function calling, the agent cannot act (and may describe actions it didn't take). Use a tool-capable model/endpoint.")
	} else if strings.TrimSpace(result) == "" {
		printSuccess("\n✓ Done")
	}
	return nil
}

// agentSystemPrompt returns the system prompt for the agent. Roles override it
// (wired in Phase 2 via activeRole).
func agentSystemPrompt() string {
	if s := activeRoleSystem(); s != "" {
		return s
	}
	return agent.DefaultSystemPrompt
}

// auditLogPath returns the path to the agent audit log.
func auditLogPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "lt", "agent-audit.jsonl")
	}
	return filepath.Join(".", ".lt-agent-audit.jsonl")
}

// auditLogger returns a hook that appends each tool invocation as a JSON line.
// Failures to write are silently ignored so logging never breaks a run.
func auditLogger() func(agent.AuditEntry) {
	path := auditLogPath()
	return func(e agent.AuditEntry) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		defer func() { _ = f.Close() }()
		rec := struct {
			Time string `json:"time"`
			agent.AuditEntry
		}{Time: time.Now().Format(time.RFC3339), AuditEntry: e}
		if data, err := json.Marshal(rec); err == nil {
			_, _ = f.Write(append(data, '\n'))
		}
	}
}

// agentConfirmFunc builds the confirmation callback honoring --yes and
// confirm_commands, and warning on dangerous shell commands.
func agentConfirmFunc() agent.ConfirmFunc {
	if agentYes || !cfg.ConfirmCommands {
		return nil // auto-approve
	}
	return func(action, detail string) (bool, error) {
		fmt.Println()
		printInfo(fmt.Sprintf("%s\n", action))
		if detail != "" {
			fmt.Println(detail)
		}
		if action == "run_command" && shellrun.IsDangerous(detail) {
			printWarning("⚠️  This command may be dangerous!")
		}
		return shellrun.Confirm("Proceed? [y/N]: ", stdinForConfirm, os.Stdout)
	}
}
