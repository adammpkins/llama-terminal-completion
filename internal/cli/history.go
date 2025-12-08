package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adammpkins/llamaterm/internal/client"
	"github.com/spf13/cobra"
)

// HistoryEntry represents a single conversation in history
type HistoryEntry struct {
	Timestamp time.Time            `json:"timestamp"`
	Model     string               `json:"model"`
	Messages  []client.ChatMessage `json:"messages"`
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View and manage chat history",
	Long: `View and manage your chat conversation history.

Examples:
  lt history list     List recent conversations
  lt history clear    Clear all history`,
}

var historyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		conversations, err := listConversations()
		if err != nil {
			return err
		}

		if len(conversations) == 0 {
			fmt.Println("No saved conversations found.")
			fmt.Println("Start a chat with: lt chat")
			return nil
		}

		fmt.Println()
		printInfo("Saved Conversations\n")
		fmt.Println("────────────────────────────────────────────────────────────────")

		for i, conv := range conversations {
			// Show up to 20 conversations
			if i >= 20 {
				fmt.Printf("  ... and %d more\n", len(conversations)-20)
				break
			}

			// Truncate title if too long
			title := conv.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}

			fmt.Printf("  %-18s │ %-10s │ %s\n",
				conv.ID,
				conv.Model,
				title)
		}
		fmt.Println()
		fmt.Println("Resume with: lt chat --resume")
		fmt.Println("Delete with: lt history delete <id>")
		return nil
	},
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all saved conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Delete all conversation files
		conversations, err := listConversations()
		if err != nil {
			return err
		}

		for _, conv := range conversations {
			_ = deleteConversation(conv.ID)
		}

		// Also clear legacy history file
		path := getHistoryPath()
		_ = os.Remove(path)

		printSuccess(fmt.Sprintf("✓ Cleared %d conversation(s)", len(conversations)))
		return nil
	},
}

var historyDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a saved conversation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if err := deleteConversation(id); err != nil {
			return err
		}
		printSuccess("✓ Conversation deleted: " + id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.AddCommand(historyListCmd)
	historyCmd.AddCommand(historyClearCmd)
	historyCmd.AddCommand(historyDeleteCmd)
}

// Injectable functions for testing
var (
	userConfigDirFunc = os.UserConfigDir
	userHomeDirFunc   = os.UserHomeDir
	mkdirAllFunc      = os.MkdirAll
	writeFileFunc     = os.WriteFile
)

// getHistoryPath returns the path to the history file
func getHistoryPath() string {
	if configDir, err := userConfigDirFunc(); err == nil {
		return filepath.Join(configDir, "lt", "history.json")
	}
	if home, err := userHomeDirFunc(); err == nil {
		return filepath.Join(home, ".lt_history.json")
	}
	return ".lt_history.json"
}

// loadHistory loads chat history from file
func loadHistory() ([]HistoryEntry, error) {
	path := getHistoryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryEntry{}, nil
		}
		return nil, err
	}

	var history []HistoryEntry
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return history, nil
}

// saveHistory saves a conversation to history
func saveHistory(messages []client.ChatMessage, model string) error {
	history, err := loadHistory()
	if err != nil {
		history = []HistoryEntry{}
	}

	// Add new entry
	entry := HistoryEntry{
		Timestamp: time.Now(),
		Model:     model,
		Messages:  messages,
	}
	history = append([]HistoryEntry{entry}, history...)

	// Keep only last 100 conversations
	if len(history) > 100 {
		history = history[:100]
	}

	// Ensure directory exists
	path := getHistoryPath()
	if err := mkdirAllFunc(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return writeFileFunc(path, data, 0644)
}
