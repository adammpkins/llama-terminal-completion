package cli

import (
	"fmt"
	"strings"

	"github.com/adammpkins/llamaterm/internal/client"
	"github.com/spf13/cobra"
)

var (
	copyOutput bool
)

var quickCmd = &cobra.Command{
	Use:   "quick [prompt]",
	Short: "Generate and run a command without confirmation",
	Long: `Generate a command from your description and run it immediately.

Unlike 'cmd', this skips confirmation (use with caution).
Outputs the command before running for transparency.

Examples:
  lt quick "show disk usage"
  lt quick "list running docker containers"`,
	Args: cobra.MinimumNArgs(1),
	RunE: executeQuickCmd,
}

var copyCmd = &cobra.Command{
	Use:   "copy [question]",
	Short: "Ask a question and copy the response to clipboard",
	Long: `Like 'ask', but copies the response to your clipboard.

Examples:
  lt copy "Write a bash function to check if a port is open"
  lt copy "Generate a .gitignore for a Go project"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCopy,
}

func init() {
	rootCmd.AddCommand(quickCmd)
	rootCmd.AddCommand(copyCmd)

	// Add --copy flag to ask command
	askCmd.Flags().BoolVarP(&copyOutput, "copy", "c", false, "Copy response to clipboard")
}

func executeQuickCmd(cmd *cobra.Command, args []string) error {
	description := strings.Join(args, " ")

	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	systemPrompt := `You are a command-line expert. Generate a single shell command that accomplishes the user's task.
Output ONLY the command, nothing else. No explanations, no markdown, no code blocks.`

	messages := []client.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: description},
	}

	resp, err := apiClient.ChatCompletion(messages, 256, 0.3)
	if err != nil {
		return fmt.Errorf("failed to generate command: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no command generated")
	}

	command := cleanCommand(resp.Choices[0].Message.Content)

	// Show the command
	fmt.Println()
	printInfo("$ ")
	fmt.Println(command)
	fmt.Println()

	// Check for dangerous patterns
	if isDangerous(command) {
		printWarning("⚠️  Dangerous command detected - aborting")
		return fmt.Errorf("command blocked for safety")
	}

	// Execute
	return executeCommand(command)
}

func runCopy(cmd *cobra.Command, args []string) error {
	question := strings.Join(args, " ")

	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	messages := []client.ChatMessage{
		{
			Role:    "system",
			Content: "You are a helpful AI assistant. Provide clear, concise, and accurate answers. IMPORTANT: Do NOT use any markdown formatting. No **bold**, no ### headers, no ``` code blocks. Write everything as plain text.",
		},
		{
			Role:    "user",
			Content: question,
		},
	}

	fmt.Println()

	var response strings.Builder

	if cfg.Stream {
		err := apiClient.ChatCompletionStream(messages, cfg.MaxTokens, cfg.Temperature, func(content string) {
			fmt.Print(content)
			response.WriteString(content)
		})
		if err != nil {
			return err
		}
	} else {
		resp, err := apiClient.ChatCompletion(messages, cfg.MaxTokens, cfg.Temperature)
		if err != nil {
			return err
		}
		if len(resp.Choices) > 0 {
			content := resp.Choices[0].Message.Content
			fmt.Println(content)
			response.WriteString(content)
		}
	}

	fmt.Println()
	fmt.Println()

	// Copy to clipboard
	if err := copyToClipboard(response.String()); err != nil {
		printWarning("Could not copy to clipboard: %v", err)
	} else {
		printSuccess("✓ Copied to clipboard")
	}

	return nil
}

// handleCopyFlag is called after ask command to optionally copy output
func handleCopyFlag(response string) {
	if copyOutput && response != "" {
		if err := copyToClipboard(response); err != nil {
			printWarning("Could not copy to clipboard: %v", err)
		} else {
			printSuccess("✓ Copied to clipboard")
		}
	}
}
