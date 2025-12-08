package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adammpkins/llamaterm/internal/client"
)

func TestGetConversationsDir(t *testing.T) {
	t.Run("uses config dir when available", func(t *testing.T) {
		originalConfigDir := conversationsConfigDirFunc
		defer func() { conversationsConfigDirFunc = originalConfigDir }()

		conversationsConfigDirFunc = func() (string, error) {
			return "/mock/config", nil
		}

		dir := getConversationsDir()
		expected := filepath.Join("/mock/config", "lt", "conversations")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	})

	t.Run("falls back to home dir", func(t *testing.T) {
		originalConfigDir := conversationsConfigDirFunc
		originalHomeDir := conversationsHomeDirFunc
		defer func() {
			conversationsConfigDirFunc = originalConfigDir
			conversationsHomeDirFunc = originalHomeDir
		}()

		conversationsConfigDirFunc = func() (string, error) {
			return "", errors.New("no config dir")
		}
		conversationsHomeDirFunc = func() (string, error) {
			return "/mock/home", nil
		}

		dir := getConversationsDir()
		expected := filepath.Join("/mock/home", ".lt", "conversations")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	})

	t.Run("falls back to current dir", func(t *testing.T) {
		originalConfigDir := conversationsConfigDirFunc
		originalHomeDir := conversationsHomeDirFunc
		defer func() {
			conversationsConfigDirFunc = originalConfigDir
			conversationsHomeDirFunc = originalHomeDir
		}()

		conversationsConfigDirFunc = func() (string, error) {
			return "", errors.New("no config dir")
		}
		conversationsHomeDirFunc = func() (string, error) {
			return "", errors.New("no home dir")
		}

		dir := getConversationsDir()
		expected := filepath.Join(".", ".lt", "conversations")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	})
}

func TestGenerateTitle(t *testing.T) {
	tests := []struct {
		name     string
		messages []client.ChatMessage
		expected string
	}{
		{
			name: "uses first user message",
			messages: []client.ChatMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "How do I fix this error?"},
			},
			expected: "How do I fix this error?",
		},
		{
			name: "truncates long messages",
			messages: []client.ChatMessage{
				{Role: "user", Content: "This is a very long message that should be truncated to 50 characters because it is too long"},
			},
			expected: "This is a very long message that should be trun...",
		},
		{
			name: "removes newlines",
			messages: []client.ChatMessage{
				{Role: "user", Content: "Line one\nLine two"},
			},
			expected: "Line one Line two",
		},
		{
			name:     "returns default for empty messages",
			messages: []client.ChatMessage{},
			expected: "Untitled Conversation",
		},
		{
			name: "returns default when no user messages",
			messages: []client.ChatMessage{
				{Role: "system", Content: "You are helpful"},
			},
			expected: "Untitled Conversation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateTitle(tt.messages)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal title", "normal title"},
		{"has/slash", "has_slash"},
		{"has\\backslash", "has_backslash"},
		{"has:colon", "has_colon"},
		{"has*star", "has_star"},
		{"has?question", "has_question"},
		{"has\"quote", "has_quote"},
		{"has<less", "has_less"},
		{"has>greater", "has_greater"},
		{"has|pipe", "has_pipe"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestListConversations(t *testing.T) {
	t.Run("returns empty list when directory doesn't exist", func(t *testing.T) {
		originalReadDir := conversationsReadDirFunc
		originalConfigDir := conversationsConfigDirFunc
		defer func() {
			conversationsReadDirFunc = originalReadDir
			conversationsConfigDirFunc = originalConfigDir
		}()

		conversationsConfigDirFunc = func() (string, error) {
			return "/mock", nil
		}
		conversationsReadDirFunc = func(name string) ([]fs.DirEntry, error) {
			return nil, os.ErrNotExist
		}

		convs, err := listConversations()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(convs) != 0 {
			t.Errorf("expected empty list, got %d items", len(convs))
		}
	})
}

func TestSaveAndLoadConversation(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()

	originalConfigDir := conversationsConfigDirFunc
	originalMkdirAll := conversationsMkdirAllFunc
	originalWriteFile := conversationsWriteFileFunc
	originalReadDir := conversationsReadDirFunc
	originalReadFile := conversationsReadFileFunc
	originalRemove := conversationsRemoveFunc

	defer func() {
		conversationsConfigDirFunc = originalConfigDir
		conversationsMkdirAllFunc = originalMkdirAll
		conversationsWriteFileFunc = originalWriteFile
		conversationsReadDirFunc = originalReadDir
		conversationsReadFileFunc = originalReadFile
		conversationsRemoveFunc = originalRemove
	}()

	conversationsConfigDirFunc = func() (string, error) {
		return tmpDir, nil
	}
	conversationsMkdirAllFunc = os.MkdirAll
	conversationsWriteFileFunc = os.WriteFile
	conversationsReadDirFunc = os.ReadDir
	conversationsReadFileFunc = os.ReadFile
	conversationsRemoveFunc = os.Remove

	t.Run("saves and loads conversation", func(t *testing.T) {
		conv := &Conversation{
			Model: "gpt-4o",
			Messages: []client.ChatMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello there!"},
				{Role: "assistant", Content: "Hi! How can I help?"},
			},
		}

		err := saveConversation(conv)
		if err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		if conv.ID == "" {
			t.Error("ID should be generated")
		}
		if conv.Title == "" {
			t.Error("Title should be generated")
		}
		if conv.Title != "Hello there!" {
			t.Errorf("expected title 'Hello there!', got %q", conv.Title)
		}

		// Load it back
		loaded, err := loadConversation(conv.ID)
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if loaded.ID != conv.ID {
			t.Errorf("ID mismatch: %s vs %s", loaded.ID, conv.ID)
		}
		if loaded.Title != conv.Title {
			t.Errorf("Title mismatch: %s vs %s", loaded.Title, conv.Title)
		}
		if len(loaded.Messages) != len(conv.Messages) {
			t.Errorf("Messages count mismatch: %d vs %d", len(loaded.Messages), len(conv.Messages))
		}
	})

	t.Run("lists conversations sorted by update time", func(t *testing.T) {
		// Create two conversations with different times
		conv1 := &Conversation{
			ID:        "older",
			Title:     "Older conversation",
			Model:     "gpt-4o",
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
			Messages:  []client.ChatMessage{{Role: "user", Content: "first"}},
		}
		conv2 := &Conversation{
			ID:        "newer",
			Title:     "Newer conversation",
			Model:     "gpt-4o",
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
			Messages:  []client.ChatMessage{{Role: "user", Content: "second"}},
		}

		// Save both (use raw file save to preserve timestamps)
		convDir := filepath.Join(tmpDir, "lt", "conversations")
		for _, c := range []*Conversation{conv1, conv2} {
			data, _ := json.MarshalIndent(c, "", "  ")
			filename := c.ID + "_" + sanitizeFilename(c.Title) + ".json"
			_ = os.WriteFile(filepath.Join(convDir, filename), data, 0644)
		}

		convs, err := listConversations()
		if err != nil {
			t.Fatalf("failed to list: %v", err)
		}

		// Should have at least 2 (plus the one from previous test)
		if len(convs) < 2 {
			t.Fatalf("expected at least 2 conversations, got %d", len(convs))
		}

		// Find newer and older in results
		var foundNewer, foundOlder bool
		var newerIdx, olderIdx int
		for i, c := range convs {
			if strings.HasPrefix(c.ID, "newer") {
				foundNewer = true
				newerIdx = i
			}
			if strings.HasPrefix(c.ID, "older") {
				foundOlder = true
				olderIdx = i
			}
		}

		if !foundNewer || !foundOlder {
			t.Error("missing expected conversations")
		}

		if newerIdx > olderIdx {
			t.Error("newer conversation should come before older")
		}
	})
}

func TestDeleteConversation(t *testing.T) {
	tmpDir := t.TempDir()

	originalConfigDir := conversationsConfigDirFunc
	originalMkdirAll := conversationsMkdirAllFunc
	originalWriteFile := conversationsWriteFileFunc
	originalReadDir := conversationsReadDirFunc
	originalRemove := conversationsRemoveFunc

	defer func() {
		conversationsConfigDirFunc = originalConfigDir
		conversationsMkdirAllFunc = originalMkdirAll
		conversationsWriteFileFunc = originalWriteFile
		conversationsReadDirFunc = originalReadDir
		conversationsRemoveFunc = originalRemove
	}()

	conversationsConfigDirFunc = func() (string, error) {
		return tmpDir, nil
	}
	conversationsMkdirAllFunc = os.MkdirAll
	conversationsWriteFileFunc = os.WriteFile
	conversationsReadDirFunc = os.ReadDir
	conversationsRemoveFunc = os.Remove

	t.Run("deletes existing conversation", func(t *testing.T) {
		// Create a file
		convDir := filepath.Join(tmpDir, "lt", "conversations")
		_ = os.MkdirAll(convDir, 0755)
		testFile := filepath.Join(convDir, "test-id_test-title.json")
		_ = os.WriteFile(testFile, []byte(`{"id":"test-id"}`), 0644)

		err := deleteConversation("test-id")
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		// Verify file is gone
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("file should be deleted")
		}
	})

	t.Run("returns error for non-existent conversation", func(t *testing.T) {
		err := deleteConversation("non-existent-id")
		if err == nil {
			t.Error("expected error for non-existent conversation")
		}
	})
}

func TestNewConversation(t *testing.T) {
	conv := newConversation("gpt-4o")

	if conv.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", conv.Model)
	}

	if len(conv.Messages) != 1 {
		t.Errorf("expected 1 system message, got %d", len(conv.Messages))
	}

	if conv.Messages[0].Role != "system" {
		t.Errorf("expected system role, got %s", conv.Messages[0].Role)
	}
}
