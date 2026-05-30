package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/agent"
	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/spf13/cobra"
)

// stdinForChat can be overridden in tests
var stdinForChat io.Reader = nil

// chatOutputWriter can be overridden in tests (defaults to os.Stdout)
var chatOutputWriter io.Writer = nil

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long: `Start an interactive conversation with the AI assistant.

The conversation maintains context, so you can have a back-and-forth dialogue.
Type 'exit', 'quit', or press Ctrl+C to end the session.

Examples:
  lt chat              Start a new conversation
  lt chat --resume     Resume a previous conversation`,
	RunE: runChat,
}

var resumeFlag bool
var agentChatFlag bool

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVarP(&resumeFlag, "resume", "R", false, "Resume a previous conversation")
	chatCmd.Flags().BoolVar(&agentChatFlag, "agent", false, "Enable tools in chat (agentic REPL with file/command/web tools)")
	// Agentic-chat flags (shared package vars with the agent command).
	chatCmd.Flags().BoolVarP(&agentYes, "yes", "y", false, "Skip confirmation prompts for write/exec tools (agent mode)")
	chatCmd.Flags().IntVar(&agentMaxIter, "max-iterations", agent.DefaultMaxIterations, "Maximum model round-trips per turn (agent mode)")
	chatCmd.Flags().BoolVar(&agentAllowOutside, "allow-outside", false, "Allow file access outside the working directory (agent mode)")
	chatCmd.Flags().StringVar(&ragFlag, "rag", "", "Expose a rag_search tool backed by this index (agent mode)")
	chatCmd.Flags().StringSliceVar(&mcpServers, "mcp", nil, "Mount tools from configured MCP servers (agent mode)")
}

func runChat(cmd *cobra.Command, args []string) error {
	// Agentic chat: rich Bubble Tea frontend in a real terminal, plain line
	// REPL otherwise (non-TTY, piped input, tests). Both drive the same evented
	// agent core.
	if agentChatFlag {
		if stdinForChat == nil && chatOutputWriter == nil && isTerminal() {
			return runAgenticChatTUI()
		}
		var reader *bufio.Reader
		if stdinForChat != nil {
			reader = bufio.NewReader(stdinForChat)
		} else {
			reader = bufio.NewReader(os.Stdin)
		}
		output := chatOutputWriter
		if output == nil {
			output = os.Stdout
		}
		return runAgenticChat(reader, output)
	}

	// Check if we're in a TTY for the beautiful TUI
	// If stdin is being injected (for tests) or we're not in a TTY, use simple mode
	if stdinForChat == nil && isTerminal() {
		return runChatTUI(resumeFlag)
	}

	// Fallback to simple mode for non-TTY or testing
	var reader *bufio.Reader
	if stdinForChat != nil {
		reader = bufio.NewReader(stdinForChat)
	} else {
		reader = bufio.NewReader(os.Stdin)
	}

	// Use injected output or default to os.Stdout
	output := chatOutputWriter
	if output == nil {
		output = os.Stdout
	}

	return runChatWithReader(reader, output)
}

// isTerminal checks if we're running in an interactive terminal
func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// runChatWithReader is the core chat function that can be tested
func runChatWithReader(reader *bufio.Reader, output io.Writer) error {
	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	// Conversation history
	chatSystem := "You are a helpful AI assistant. Be concise but thorough in your responses."
	if s := activeRoleSystem(); s != "" {
		chatSystem = s
	}
	messages := []client.ChatMessage{
		{
			Role:    "system",
			Content: chatSystem,
		},
	}

	// Print welcome message
	_, _ = fmt.Fprintln(output)
	_, _ = fmt.Fprintf(output, "╭─────────────────────────────────────────╮\n")
	_, _ = fmt.Fprintf(output, "│         LlamaTerm Chat Session          │\n")
	_, _ = fmt.Fprintf(output, "│   Type 'exit' or 'quit' to end chat     │\n")
	_, _ = fmt.Fprintf(output, "╰─────────────────────────────────────────╯\n")
	_, _ = fmt.Fprintf(output, "  Model: %s\n", cfg.Model)
	_, _ = fmt.Fprintf(output, "  API:   %s\n", cfg.BaseURL)
	_, _ = fmt.Fprintln(output)

	for {
		// Print prompt
		_, _ = fmt.Fprint(output, "You: ")

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
			_, _ = fmt.Fprintln(output)
			_, _ = fmt.Fprintln(output, "Goodbye! 👋")
			return nil
		}

		// Special commands
		if input == "/clear" {
			messages = messages[:1] // Keep system prompt
			_, _ = fmt.Fprintln(output, "Chat history cleared.")
			_, _ = fmt.Fprintln(output)
			continue
		}
		if input == "/help" {
			printChatHelpTo(output)
			continue
		}
		if input == "/save" {
			if err := saveHistory(messages, cfg.Model); err != nil {
				_, _ = fmt.Fprintf(output, "Failed to save: %v\n", err)
			} else {
				_, _ = fmt.Fprintln(output, "✓ Conversation saved")
			}
			_, _ = fmt.Fprintln(output)
			continue
		}

		// Add user message to history
		messages = append(messages, client.ChatMessage{
			Role:    "user",
			Content: input,
		})

		// Get response
		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprint(output, "Assistant: ")

		var responseBuilder strings.Builder

		if cfg.Stream {
			err = apiClient.ChatCompletionStream(messages, cfg.MaxTokens, cfg.Temperature, func(content string) {
				_, _ = fmt.Fprint(output, content)
				responseBuilder.WriteString(content)
			})
		} else {
			resp, err := apiClient.ChatCompletion(messages, cfg.MaxTokens, cfg.Temperature)
			if err == nil && len(resp.Choices) > 0 {
				content := resp.Choices[0].Message.Content
				_, _ = fmt.Fprint(output, content)
				responseBuilder.WriteString(content)
			}
		}

		if err != nil {
			_, _ = fmt.Fprintf(output, "Error: %v\n", err)
			// Remove failed user message
			messages = messages[:len(messages)-1]
		} else {
			// Add assistant response to history
			messages = append(messages, client.ChatMessage{
				Role:    "assistant",
				Content: responseBuilder.String(),
			})
		}

		_, _ = fmt.Fprintln(output)
		_, _ = fmt.Fprintln(output)
	}
}

func printChatHelp() {
	printChatHelpTo(os.Stdout)
}

func printChatHelpTo(w io.Writer) {
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Chat Commands:")
	_, _ = fmt.Fprintln(w, "  /clear  - Clear conversation history")
	_, _ = fmt.Fprintln(w, "  /save   - Save conversation to history")
	_, _ = fmt.Fprintln(w, "  /help   - Show this help")
	_, _ = fmt.Fprintln(w, "  exit    - End the chat session")
	_, _ = fmt.Fprintln(w)
}
