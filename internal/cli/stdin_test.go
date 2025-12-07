package cli

import (
	"strings"
	"testing"
)

func TestGetQuestionWithPipedInput(t *testing.T) {
	// Set up a test reader
	input := "What is the meaning of life?"
	stdinForQuestion = strings.NewReader(input)
	defer func() { stdinForQuestion = nil }()

	result, err := getQuestion([]string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("getQuestion() = %q, want %q", result, input)
	}
}

func TestGetQuestionWithMultilineInput(t *testing.T) {
	input := "Line 1\nLine 2\nLine 3"
	stdinForQuestion = strings.NewReader(input)
	defer func() { stdinForQuestion = nil }()

	result, err := getQuestion([]string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("getQuestion() = %q, want %q", result, input)
	}
}

func TestGetQuestionPrefersArgs(t *testing.T) {
	// Even with stdin set, args should take precedence
	stdinForQuestion = strings.NewReader("from stdin")
	defer func() { stdinForQuestion = nil }()

	result, err := getQuestion([]string{"from args"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "from args" {
		t.Errorf("getQuestion() = %q, want 'from args'", result)
	}
}

func TestGetErrorMessageWithPipedInput(t *testing.T) {
	input := "Error: module not found"
	stdinForError = strings.NewReader(input)
	defer func() { stdinForError = nil }()

	result, err := getErrorMessage([]string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("getErrorMessage() = %q, want %q", result, input)
	}
}

func TestGetErrorMessageWithStackTrace(t *testing.T) {
	input := `Error: Cannot find module 'express'
    at Module._resolveFilename (internal/modules/cjs/loader.js:885:15)
    at Module._load (internal/modules/cjs/loader.js:730:27)
    at require (internal/modules/cjs/helpers.js:14:16)`

	stdinForError = strings.NewReader(input)
	defer func() { stdinForError = nil }()

	result, err := getErrorMessage([]string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != strings.TrimSpace(input) {
		t.Errorf("getErrorMessage() = %q, want %q", result, input)
	}
}

func TestGetErrorMessagePrefersArgs(t *testing.T) {
	stdinForError = strings.NewReader("from stdin")
	defer func() { stdinForError = nil }()

	result, err := getErrorMessage([]string{"from args"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "from args" {
		t.Errorf("getErrorMessage() = %q, want 'from args'", result)
	}
}

func TestReadFromStdin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple line",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "with trailing newline",
			input:    "hello world\n",
			expected: "hello world",
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "just whitespace",
			input:    "   \n   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := readFromStdin(reader)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("readFromStdin() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReadErrorFromStdin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple error",
			input:    "Error: something went wrong",
			expected: "Error: something went wrong",
		},
		{
			name:     "multiline stack trace",
			input:    "Error at line 1\n  at file.go:10\n  at main.go:5",
			expected: "Error at line 1\n  at file.go:10\n  at main.go:5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := readErrorFromStdin(reader)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("readErrorFromStdin() = %q, want %q", result, tt.expected)
			}
		})
	}
}
