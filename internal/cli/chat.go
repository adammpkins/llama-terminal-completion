package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/adammpkins/llamaterm/internal/client"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long: `Start an interactive conversation with the AI assistant.

The conversation maintains context, so you can have a back-and-forth dialogue.
Type 'exit', 'quit', or press Ctrl+C to end the session.

Examples:
  lt chat
  lt chat --model gpt-4`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	// Conversation history
	messages := []client.ChatMessage{
		{
			Role:    "system",
			Content: "You are a helpful AI assistant. Be concise but thorough in your responses.",
		},
	}

	reader := bufio.NewReader(os.Stdin)

	// Print welcome message
	fmt.Println()
	printInfo("╭─────────────────────────────────────────╮\n")
	printInfo("│         LlamaTerm Chat Session          │\n")
	printInfo("│   Type 'exit' or 'quit' to end chat     │\n")
	printInfo("╰─────────────────────────────────────────╯\n")
	fmt.Printf("  Model: %s\n", cfg.Model)
	fmt.Printf("  API:   %s\n", cfg.BaseURL)
	fmt.Println()

	for {
		// Print prompt
		printInfo("You: ")

		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil // EOF or error, exit gracefully
		}

		input = strings.TrimSpace(input)

		// Check for exit commands
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" || input == "q" {
			fmt.Println()
			printSuccess("Goodbye! 👋")
			return nil
		}

		// Special commands
		if input == "/clear" {
			messages = messages[:1] // Keep system prompt
			printInfo("Chat history cleared.\n\n")
			continue
		}
		if input == "/help" {
			printChatHelp()
			continue
		}
		if input == "/save" {
			if err := saveHistory(messages, cfg.Model); err != nil {
				printError("Failed to save: %v", err)
			} else {
				printSuccess("✓ Conversation saved")
			}
			fmt.Println()
			continue
		}

		// Add user message to history
		messages = append(messages, client.ChatMessage{
			Role:    "user",
			Content: input,
		})

		// Get response
		fmt.Println()
		printInfo("Assistant: ")

		var responseBuilder strings.Builder

		if cfg.Stream {
			err = apiClient.ChatCompletionStream(messages, cfg.MaxTokens, cfg.Temperature, func(content string) {
				fmt.Print(content)
				responseBuilder.WriteString(content)
			})
		} else {
			resp, err := apiClient.ChatCompletion(messages, cfg.MaxTokens, cfg.Temperature)
			if err == nil && len(resp.Choices) > 0 {
				content := resp.Choices[0].Message.Content
				fmt.Print(content)
				responseBuilder.WriteString(content)
			}
		}

		if err != nil {
			printError("%v", err)
			// Remove failed user message
			messages = messages[:len(messages)-1]
		} else {
			// Add assistant response to history
			messages = append(messages, client.ChatMessage{
				Role:    "assistant",
				Content: responseBuilder.String(),
			})
		}

		fmt.Println()
		fmt.Println()
	}
}

func printChatHelp() {
	fmt.Println()
	printInfo("Chat Commands:\n")
	fmt.Println("  /clear  - Clear conversation history")
	fmt.Println("  /save   - Save conversation to history")
	fmt.Println("  /help   - Show this help")
	fmt.Println("  exit    - End the chat session")
	fmt.Println()
}
