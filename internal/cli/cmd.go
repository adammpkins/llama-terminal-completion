package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/shellrun"
	"github.com/spf13/cobra"
)

var (
	dryRun  bool
	autoRun bool
)

var cmdCmd = &cobra.Command{
	Use:   "cmd [description]",
	Short: "Generate a shell command from natural language",
	Long: `Generate a shell command based on your description.

By default, the command will ask for confirmation before running.

Examples:
  lt cmd "list all files larger than 100MB"
  lt cmd "find all Go files modified in the last week"
  lt cmd "compress all images in current directory"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCmd,
}

func init() {
	cmdCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show command without running")
	cmdCmd.Flags().BoolVarP(&autoRun, "yes", "y", false, "Run command without confirmation")
}

// stdinForConfirm can be overridden in tests
var stdinForConfirm io.Reader = nil

func runCmd(cmd *cobra.Command, args []string) error {
	// Get the description from args or stdin
	description, err := getQuestion(args)
	if err != nil {
		return err
	}

	if description == "" {
		return fmt.Errorf("no command description provided")
	}

	// Create API client
	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	// Build the prompt for command generation
	systemPrompt := `You are a command-line expert. Generate a single shell command that accomplishes the user's task.

Rules:
- Output ONLY the command, nothing else
- No explanations, no markdown, no code blocks
- Use common Unix/Linux commands
- Prefer simple, safe commands
- If multiple commands are needed, chain them with && or pipes`

	messages := []client.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: description},
	}

	// Get the command (non-streaming for commands)
	resp, err := apiClient.ChatCompletion(messages, 256, 0.3)
	if err != nil {
		return fmt.Errorf("failed to generate command: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no command generated")
	}

	// Clean up the command
	command := cleanCommand(resp.Choices[0].Message.Content)

	if command == "" {
		return fmt.Errorf("failed to extract command from response")
	}

	// Display the command
	fmt.Println()
	printInfo("Command: ")
	fmt.Println(command)
	fmt.Println()

	// Check for dangerous patterns
	if isDangerous(command) {
		printWarning("⚠️  This command may be dangerous!")
	}

	// Dry run - just show the command
	if dryRun {
		printInfo("(dry-run mode - command not executed)\n")
		return nil
	}

	// Auto-run or ask for confirmation
	if !autoRun && cfg.ConfirmCommands {
		ok, _ := shellrun.Confirm("Run this command? [y/N]: ", stdinForConfirm, os.Stdout)
		if !ok {
			printInfo("Command not executed.\n")
			return nil
		}
	}

	// Execute the command
	return executeCommand(command)
}

// cleanCommand removes markdown formatting and extra whitespace from a command
func cleanCommand(raw string) string {
	cmd := strings.TrimSpace(raw)

	// Remove markdown code blocks
	cmd = strings.TrimPrefix(cmd, "```bash")
	cmd = strings.TrimPrefix(cmd, "```sh")
	cmd = strings.TrimPrefix(cmd, "```")
	cmd = strings.TrimSuffix(cmd, "```")
	cmd = strings.TrimSpace(cmd)

	// Remove backticks
	cmd = strings.Trim(cmd, "`")

	// Take only the first line if multiple lines
	if idx := strings.Index(cmd, "\n"); idx != -1 {
		cmd = cmd[:idx]
	}

	return strings.TrimSpace(cmd)
}

// isDangerous checks if a command contains potentially dangerous patterns.
// It delegates to shellrun so the heuristics are single-sourced.
func isDangerous(cmd string) bool {
	return shellrun.IsDangerous(cmd)
}

// executeCommand runs a shell command interactively, wrapping shellrun with the
// CLI's status messages.
func executeCommand(command string) error {
	printInfo("Running...\n\n")

	if err := shellrun.RunInteractive(command, cfg.Shell); err != nil {
		return err
	}

	fmt.Println()
	printSuccess("✓ Command completed successfully")
	return nil
}
