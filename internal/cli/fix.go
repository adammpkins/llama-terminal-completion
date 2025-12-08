package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/spf13/cobra"
)

// stdinForError can be overridden in tests
var stdinForError io.Reader = nil

var fixCmd = &cobra.Command{
	Use:   "fix [error message]",
	Short: "Get suggestions to fix an error",
	Long: `Analyze an error message and get suggestions for fixing it.

You can provide the error as an argument or pipe it in.

Examples:
  lt fix "Error: cannot find module 'express'"
  lt fix "undefined is not a function"
  cat error.log | lt fix
  npm run build 2>&1 | lt fix`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFix,
}

func init() {
	rootCmd.AddCommand(fixCmd)
}

func runFix(cmd *cobra.Command, args []string) error {
	// Get error from args or stdin
	errorMsg, err := getErrorMessage(args)
	if err != nil {
		return err
	}

	if errorMsg == "" {
		return fmt.Errorf("no error message provided")
	}

	// Create API client
	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	systemPrompt := `You are an expert debugger and problem solver. Analyze the error message and provide:

1) What went wrong - Brief explanation of the error
2) Likely cause - Most probable reason for this error  
3) How to fix it - Step-by-step instructions to resolve the issue
4) Prevention - How to avoid this error in the future (if applicable)

IMPORTANT: Do NOT use any markdown formatting. No **bold**, no ### headers, no backticks. Write everything as plain text.`

	messages := []client.ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Please help me fix this error:\n\n%s", errorMsg),
		},
	}

	// Get response
	fmt.Println()
	printInfo("🔧 Analyzing error...\n\n")

	if cfg.Stream {
		err = apiClient.ChatCompletionStream(messages, cfg.MaxTokens, cfg.Temperature, func(content string) {
			fmt.Print(content)
		})
		fmt.Println()
	} else {
		resp, err := apiClient.ChatCompletion(messages, cfg.MaxTokens, cfg.Temperature)
		if err != nil {
			return err
		}
		if len(resp.Choices) > 0 {
			fmt.Println(resp.Choices[0].Message.Content)
		}
	}

	return err
}

// getErrorMessage gets the error from args or stdin
func getErrorMessage(args []string) (string, error) {
	// If argument provided, use it
	if len(args) > 0 {
		return args[0], nil
	}

	// If a test reader is set, use it
	if stdinForError != nil {
		return readErrorFromStdin(stdinForError)
	}

	// Check if stdin has data (pipe)
	if stdinStatFunc() {
		// Data is being piped - read all of it
		return readErrorFromStdin(os.Stdin)
	}

	return "", nil
}

// readErrorFromStdin reads all content from a reader
func readErrorFromStdin(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		builder.WriteString(line)
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(builder.String()), nil
}
