package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adammpkins/llamaterm/internal/client"
)

// Conversation represents a single chat conversation
type Conversation struct {
	ID        string               `json:"id"`
	Title     string               `json:"title"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
	Model     string               `json:"model"`
	Messages  []client.ChatMessage `json:"messages"`
}

// Injectable functions for testing
var (
	conversationsConfigDirFunc = os.UserConfigDir
	conversationsHomeDirFunc   = os.UserHomeDir
	conversationsMkdirAllFunc  = os.MkdirAll
	conversationsWriteFileFunc = os.WriteFile
	conversationsReadDirFunc   = os.ReadDir
	conversationsReadFileFunc  = os.ReadFile
	conversationsRemoveFunc    = os.Remove
)

// getConversationsDir returns the path to the conversations directory
func getConversationsDir() string {
	if configDir, err := conversationsConfigDirFunc(); err == nil {
		return filepath.Join(configDir, "lt", "conversations")
	}
	if home, err := conversationsHomeDirFunc(); err == nil {
		return filepath.Join(home, ".lt", "conversations")
	}
	return filepath.Join(".", ".lt", "conversations")
}

// generateConversationID creates a unique ID based on timestamp
func generateConversationID() string {
	return time.Now().Format("2006-01-02_150405")
}

// generateTitle creates a title from the first user message
func generateTitle(messages []client.ChatMessage) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			title := msg.Content
			// Truncate to 50 chars max
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			// Remove newlines
			title = strings.ReplaceAll(title, "\n", " ")
			title = strings.TrimSpace(title)
			if title != "" {
				return title
			}
		}
	}
	return "Untitled Conversation"
}

// sanitizeFilename removes characters that aren't safe for filenames
func sanitizeFilename(s string) string {
	// Replace unsafe characters with underscore
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := s
	for _, char := range unsafe {
		result = strings.ReplaceAll(result, char, "_")
	}
	return result
}

// listConversations returns all saved conversations, sorted by most recent first
func listConversations() ([]Conversation, error) {
	dir := getConversationsDir()
	entries, err := conversationsReadDirFunc(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Conversation{}, nil
		}
		return nil, fmt.Errorf("failed to read conversations directory: %w", err)
	}

	var conversations []Conversation
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		conv, err := loadConversationFromFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // Skip corrupted files
		}
		conversations = append(conversations, *conv)
	}

	// Sort by UpdatedAt descending (most recent first)
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].UpdatedAt.After(conversations[j].UpdatedAt)
	})

	return conversations, nil
}

// loadConversation loads a conversation by ID
func loadConversation(id string) (*Conversation, error) {
	dir := getConversationsDir()

	// Find file matching this ID
	entries, err := conversationsReadDirFunc(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversations directory: %w", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), id) && strings.HasSuffix(entry.Name(), ".json") {
			return loadConversationFromFile(filepath.Join(dir, entry.Name()))
		}
	}

	return nil, fmt.Errorf("conversation not found: %s", id)
}

// loadConversationFromFile loads a conversation from a specific file path
func loadConversationFromFile(path string) (*Conversation, error) {
	data, err := conversationsReadFileFunc(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversation file: %w", err)
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("failed to parse conversation: %w", err)
	}

	return &conv, nil
}

// saveConversation saves or updates a conversation
func saveConversation(conv *Conversation) error {
	// Ensure directory exists
	dir := getConversationsDir()
	if err := conversationsMkdirAllFunc(dir, 0755); err != nil {
		return fmt.Errorf("failed to create conversations directory: %w", err)
	}

	// Generate ID if new conversation
	if conv.ID == "" {
		conv.ID = generateConversationID()
		conv.CreatedAt = time.Now()
	}

	// Auto-generate title if empty
	if conv.Title == "" {
		conv.Title = generateTitle(conv.Messages)
	}

	// Update timestamp
	conv.UpdatedAt = time.Now()

	// Create filename: ID_sanitized-title.json
	safeTitle := sanitizeFilename(conv.Title)
	if len(safeTitle) > 30 {
		safeTitle = safeTitle[:30]
	}
	filename := fmt.Sprintf("%s_%s.json", conv.ID, safeTitle)
	path := filepath.Join(dir, filename)

	// Delete old file if title changed (filename would be different)
	_ = deleteConversationFiles(conv.ID, path)

	// Marshal and save
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	return conversationsWriteFileFunc(path, data, 0644)
}

// deleteConversationFiles removes any existing files for this ID except the current one
func deleteConversationFiles(id string, exceptPath string) error {
	dir := getConversationsDir()
	entries, err := conversationsReadDirFunc(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), id) && strings.HasSuffix(entry.Name(), ".json") {
			fullPath := filepath.Join(dir, entry.Name())
			if fullPath != exceptPath {
				_ = conversationsRemoveFunc(fullPath)
			}
		}
	}
	return nil
}

// deleteConversation removes a conversation by ID
func deleteConversation(id string) error {
	dir := getConversationsDir()
	entries, err := conversationsReadDirFunc(dir)
	if err != nil {
		return fmt.Errorf("failed to read conversations directory: %w", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), id) && strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(dir, entry.Name())
			if err := conversationsRemoveFunc(path); err != nil {
				return fmt.Errorf("failed to delete conversation: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("conversation not found: %s", id)
}

// newConversation creates a new conversation with initial system message
func newConversation(model string) *Conversation {
	return &Conversation{
		Model: model,
		Messages: []client.ChatMessage{
			{Role: "system", Content: "You are a helpful AI assistant. Be concise but thorough. Format code with markdown."},
		},
	}
}
