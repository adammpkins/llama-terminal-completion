package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/adammpkins/llamaterm/internal/client"
	"github.com/spf13/cobra"
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a question to the AI assistant",
	Long: `Ask a question and get an AI-powered response.

Examples:
  lt ask "What is the capital of France?"
  lt ask "Explain how TCP/IP works"
  echo "What is 2+2?" | lt ask`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAsk,
}

func runAsk(cmd *cobra.Command, args []string) error {
	// Get the question from args or stdin
	question, err := getQuestion(args)
	if err != nil {
		return err
	}

	if question == "" {
		return fmt.Errorf("no question provided")
	}

	// Create API client
	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	// Build messages
	messages := []client.ChatMessage{
		{
			Role:    "system",
			Content: "You are a helpful AI assistant. Provide clear, concise, and accurate answers.",
		},
		{
			Role:    "user",
			Content: question,
		},
	}

	var response strings.Builder

	// Stream or non-stream response
	if cfg.Stream {
		fmt.Println() // Start on new line
		err = apiClient.ChatCompletionStream(messages, cfg.MaxTokens, cfg.Temperature, func(content string) {
			fmt.Print(content)
			response.WriteString(content)
		})
		fmt.Println() // End with newline
	} else {
		resp, err := apiClient.ChatCompletion(messages, cfg.MaxTokens, cfg.Temperature)
		if err != nil {
			return err
		}
		if len(resp.Choices) > 0 {
			content := resp.Choices[0].Message.Content
			fmt.Println()
			fmt.Println(content)
			response.WriteString(content)
		}
	}

	// Handle --copy flag
	if copyOutput && response.Len() > 0 {
		fmt.Println()
		if err := copyToClipboard(response.String()); err != nil {
			printWarning("Could not copy to clipboard: %v", err)
		} else {
			printSuccess("✓ Copied to clipboard")
		}
	}

	return err
}

// getQuestion gets the question from args or stdin
func getQuestion(args []string) (string, error) {
	// If argument provided, use it
	if len(args) > 0 {
		return args[0], nil
	}

	// Check if stdin has data (pipe)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped
		reader := bufio.NewReader(os.Stdin)
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

	return "", nil
}
