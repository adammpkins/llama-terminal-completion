package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestGetQuestion(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "argument provided",
			args:     []string{"What is 2+2?"},
			expected: "What is 2+2?",
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getQuestion(tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("getQuestion(%v) = %q, want %q", tt.args, result, tt.expected)
			}
		})
	}
}

func TestGetErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "argument provided",
			args:     []string{"Error: module not found"},
			expected: "Error: module not found",
		},
		{
			name:     "empty args no stdin",
			args:     []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getErrorMessage(tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("getErrorMessage(%v) = %q, want %q", tt.args, result, tt.expected)
			}
		})
	}
}

func TestGetHistoryPathVariations(t *testing.T) {
	// Test that getHistoryPath returns a valid path in different scenarios
	path := getHistoryPath()

	if path == "" {
		t.Error("getHistoryPath should not return empty")
	}

	// Path should contain "history.json" or ".lt_history.json"
	if !strings.Contains(path, "history") {
		t.Errorf("Expected path to contain 'history', got %s", path)
	}
}

func TestPrintFunctions(t *testing.T) {
	// Test that print functions don't panic and produce output
	// printError goes to stderr, others go to stdout

	// Test stderr (printError)
	oldErr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	printError("test error %s", "msg")

	_ = wErr.Close()
	os.Stderr = oldErr

	var bufErr bytes.Buffer
	_, _ = bufErr.ReadFrom(rErr)
	errOutput := bufErr.String()

	if !strings.Contains(errOutput, "test error msg") {
		t.Errorf("printError output expected 'test error msg', got: %s", errOutput)
	}

	// Test stdout (printSuccess, printWarning, printInfo)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSuccess("test success %s", "msg")
	printWarning("test warning %s", "msg")
	printInfo("test info %s", "msg")

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test success msg") {
		t.Errorf("printSuccess output expected, got: %s", output)
	}
	if !strings.Contains(output, "test warning msg") {
		t.Errorf("printWarning output expected, got: %s", output)
	}
	if !strings.Contains(output, "test info msg") {
		t.Errorf("printInfo output expected, got: %s", output)
	}
}

func TestPrintChatHelp(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printChatHelp()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Verify help contains expected commands
	if !strings.Contains(output, "/clear") {
		t.Error("Help should contain /clear command")
	}
	if !strings.Contains(output, "/save") {
		t.Error("Help should contain /save command")
	}
	if !strings.Contains(output, "exit") {
		t.Error("Help should contain exit command")
	}
}

func TestHandleCopyFlag(t *testing.T) {
	// Test with copyOutput = false (no clipboard action)
	copyOutput = false
	handleCopyFlag("test response")
	// Should not panic or error

	// Test with empty response
	copyOutput = true
	handleCopyFlag("")
	// Should not try to copy empty string

	// Test with actual content
	copyOutput = true
	handleCopyFlag("actual content to copy")
	// May fail on clipboard but shouldn't panic

	// Reset
	copyOutput = false
}

func TestHandleCopyFlagWithClipboard(t *testing.T) {
	// Specifically test the clipboard branch
	copyOutput = true
	defer func() { copyOutput = false }()

	// This should attempt clipboard copy
	handleCopyFlag("clipboard test content")
}

func TestCopyToClipboard(t *testing.T) {
	// This test is platform-dependent but should not panic
	err := copyToClipboard("test content")
	// We don't fail on error since clipboard may not be available in CI
	if err != nil {
		t.Logf("Clipboard test skipped (expected in CI): %v", err)
	}
}
