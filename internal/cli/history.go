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
	Short: "List recent conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		history, err := loadHistory()
		if err != nil {
			return err
		}

		if len(history) == 0 {
			fmt.Println("No chat history found.")
			return nil
		}

		fmt.Println()
		printInfo("Chat History\n")
		fmt.Println("────────────────────────────────────────")

		for i, entry := range history {
			// Show last 10
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(history)-10)
				break
			}

			// Find first user message
			var preview string
			for _, msg := range entry.Messages {
				if msg.Role == "user" {
					preview = msg.Content
					if len(preview) > 50 {
						preview = preview[:50] + "..."
					}
					break
				}
			}

			fmt.Printf("  %s | %s | %s\n",
				entry.Timestamp.Format("2006-01-02 15:04"),
				entry.Model,
				preview)
		}
		fmt.Println()
		return nil
	},
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all chat history",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := getHistoryPath()
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clear history: %w", err)
		}
		printSuccess("✓ Chat history cleared")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.AddCommand(historyListCmd)
	historyCmd.AddCommand(historyClearCmd)
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
