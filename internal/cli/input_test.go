package cli

import (
	"strings"
	"testing"
)

func TestReadFromReader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line",
			input:    "hello world",
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
			name:     "with trailing newline",
			input:    "hello world\n",
			expected: "hello world",
		},
		{
			name:     "with leading and trailing whitespace",
			input:    "  hello world  \n",
			expected: "hello world",
		},
		{
			name:     "long multi-line input",
			input:    "Error: something went wrong\nStack trace:\n  at main.go:10\n  at handler.go:25\n",
			expected: "Error: something went wrong\nStack trace:\n  at main.go:10\n  at handler.go:25",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := readFromReader(reader)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("readFromReader() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReadFromReaderLargeInput(t *testing.T) {
	// Test with large input
	input := strings.Repeat("This is a long line of text. ", 1000)
	reader := strings.NewReader(input)
	result, err := readFromReader(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != strings.TrimSpace(input) {
		t.Error("Large input not read correctly")
	}
}

func TestReadFromReaderWithNewlines(t *testing.T) {
	input := "line1\nline2\nline3\nline4\n"
	reader := strings.NewReader(input)
	result, err := readFromReader(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := "line1\nline2\nline3\nline4"
	if result != expected {
		t.Errorf("readFromReader() = %q, want %q", result, expected)
	}
}
