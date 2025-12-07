package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adammpkins/llamaterm/internal/client"
)

func TestSaveAndLoadHistory(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create test messages
	messages := []client.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	// Save history
	err := saveHistory(messages, "test-model")
	if err != nil {
		t.Fatalf("saveHistory failed: %v", err)
	}

	// Load history
	history, err := loadHistory()
	if err != nil {
		t.Fatalf("loadHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(history))
	}

	entry := history[0]
	if entry.Model != "test-model" {
		t.Errorf("Expected model test-model, got %s", entry.Model)
	}
	if len(entry.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(entry.Messages))
	}
	if entry.Messages[1].Content != "Hello" {
		t.Errorf("Expected user message 'Hello', got %s", entry.Messages[1].Content)
	}
}

func TestLoadHistoryEmpty(t *testing.T) {
	// Create a temp directory with no history
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	history, err := loadHistory()
	if err != nil {
		t.Fatalf("loadHistory failed on empty: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d entries", len(history))
	}
}

func TestLoadHistoryMalformedJSON(t *testing.T) {
	// Write malformed JSON to the actual history path
	historyPath := getHistoryPath()

	// Create dir if needed
	os.MkdirAll(filepath.Dir(historyPath), 0755)

	// Backup existing if any
	origData, _ := os.ReadFile(historyPath)
	defer func() {
		if len(origData) > 0 {
			os.WriteFile(historyPath, origData, 0644)
		} else {
			os.Remove(historyPath)
		}
	}()

	// Write malformed JSON
	os.WriteFile(historyPath, []byte("not valid json"), 0644)

	_, err := loadHistory()
	if err == nil {
		t.Error("Expected error for malformed JSON")
	}
}

func TestHistoryLimit(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	messages := []client.ChatMessage{
		{Role: "user", Content: "Test"},
	}

	// Save more than 100 entries
	for i := 0; i < 105; i++ {
		err := saveHistory(messages, "model")
		if err != nil {
			t.Fatalf("saveHistory failed: %v", err)
		}
	}

	history, err := loadHistory()
	if err != nil {
		t.Fatalf("loadHistory failed: %v", err)
	}

	// Should be capped at 100
	if len(history) > 100 {
		t.Errorf("Expected max 100 entries, got %d", len(history))
	}
}

func TestHistoryTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	before := time.Now()

	messages := []client.ChatMessage{
		{Role: "user", Content: "Test"},
	}
	saveHistory(messages, "model")

	after := time.Now()

	history, _ := loadHistory()
	if len(history) == 0 {
		t.Fatal("Expected at least one entry")
	}

	ts := history[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("Timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}

func TestGetHistoryPath(t *testing.T) {
	path := getHistoryPath()

	if path == "" {
		t.Error("Expected non-empty history path")
	}

	// Should end with history.json or .lt_history.json
	base := filepath.Base(path)
	if base != "history.json" && base != ".lt_history.json" {
		t.Errorf("Unexpected history filename: %s", base)
	}
}

func TestGetHistoryPathConfigDir(t *testing.T) {
	origConfigDir := userConfigDirFunc
	origHomeDir := userHomeDirFunc
	defer func() {
		userConfigDirFunc = origConfigDir
		userHomeDirFunc = origHomeDir
	}()

	userConfigDirFunc = func() (string, error) {
		return "/mock/config", nil
	}

	path := getHistoryPath()
	if path != "/mock/config/lt/history.json" {
		t.Errorf("Expected /mock/config/lt/history.json, got %s", path)
	}
}

func TestGetHistoryPathFallbackToHome(t *testing.T) {
	origConfigDir := userConfigDirFunc
	origHomeDir := userHomeDirFunc
	defer func() {
		userConfigDirFunc = origConfigDir
		userHomeDirFunc = origHomeDir
	}()

	userConfigDirFunc = func() (string, error) {
		return "", os.ErrNotExist
	}
	userHomeDirFunc = func() (string, error) {
		return "/mock/home", nil
	}

	path := getHistoryPath()
	if path != "/mock/home/.lt_history.json" {
		t.Errorf("Expected /mock/home/.lt_history.json, got %s", path)
	}
}

func TestGetHistoryPathFallbackToLocal(t *testing.T) {
	origConfigDir := userConfigDirFunc
	origHomeDir := userHomeDirFunc
	defer func() {
		userConfigDirFunc = origConfigDir
		userHomeDirFunc = origHomeDir
	}()

	userConfigDirFunc = func() (string, error) {
		return "", os.ErrNotExist
	}
	userHomeDirFunc = func() (string, error) {
		return "", os.ErrNotExist
	}

	path := getHistoryPath()
	if path != ".lt_history.json" {
		t.Errorf("Expected .lt_history.json, got %s", path)
	}
}

func TestSaveHistoryMkdirError(t *testing.T) {
	origMkdir := mkdirAllFunc
	defer func() { mkdirAllFunc = origMkdir }()

	mkdirAllFunc = func(path string, perm os.FileMode) error {
		return fmt.Errorf("permission denied")
	}

	messages := []client.ChatMessage{
		{Role: "user", Content: "test"},
	}

	err := saveHistory(messages, "test-model")
	if err == nil {
		t.Error("Expected error when MkdirAll fails")
	}
	if err.Error() != "permission denied" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSaveHistoryWriteError(t *testing.T) {
	origWrite := writeFileFunc
	defer func() { writeFileFunc = origWrite }()

	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("disk full")
	}

	messages := []client.ChatMessage{
		{Role: "user", Content: "test"},
	}

	err := saveHistory(messages, "test-model")
	if err == nil {
		t.Error("Expected error when WriteFile fails")
	}
	if err.Error() != "disk full" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSaveHistorySuccess(t *testing.T) {
	origMkdir := mkdirAllFunc
	origWrite := writeFileFunc
	defer func() {
		mkdirAllFunc = origMkdir
		writeFileFunc = origWrite
	}()

	mkdirAllFunc = func(path string, perm os.FileMode) error {
		return nil
	}
	var savedData []byte
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		savedData = data
		return nil
	}

	messages := []client.ChatMessage{
		{Role: "user", Content: "test question"},
		{Role: "assistant", Content: "test answer"},
	}

	err := saveHistory(messages, "test-model")
	if err != nil {
		t.Fatalf("saveHistory failed: %v", err)
	}

	if len(savedData) == 0 {
		t.Error("No data was written")
	}
}
