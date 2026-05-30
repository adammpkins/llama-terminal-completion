package cli

import (
	"context"
	"os"
	"path/filepath"
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

func TestExpandReferences_NoAt(t *testing.T) {
	in := "just a plain question"
	out, err := expandReferences(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != in {
		t.Errorf("plain input changed: %q", out)
	}
}

func TestExpandReferences_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("file body here"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := expandReferences("summarize @" + path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Referenced content") || !strings.Contains(out, "file body here") {
		t.Errorf("file not inlined: %q", out)
	}
}

func TestExpandReferences_UnknownTokenNoted(t *testing.T) {
	out, err := expandReferences("look at @./definitely-missing-file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "could not load") {
		t.Errorf("expected a could-not-load note, got: %q", out)
	}
}

func TestExpandReferences_Email_NotMatched(t *testing.T) {
	in := "email me at user@example.com please"
	out, err := expandReferences(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Referenced content") {
		t.Errorf("email should not be treated as a reference: %q", out)
	}
}

func TestExpandReferences_Escaped(t *testing.T) {
	in := `mention \@handle literally`
	out, err := expandReferences(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Referenced content") {
		t.Errorf("escaped @ should not expand: %q", out)
	}
	if !strings.Contains(out, "@handle") || strings.Contains(out, `\@handle`) {
		t.Errorf("escaped @ should render literally: %q", out)
	}
}

func TestExpandReferences_URL(t *testing.T) {
	orig := referenceFetchFunc
	referenceFetchFunc = func(ctx context.Context, url string) (string, error) {
		return "fetched body for " + url, nil
	}
	defer func() { referenceFetchFunc = orig }()

	out, err := expandReferences("read @https://example.com/doc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "fetched body for https://example.com/doc") {
		t.Errorf("URL content not inlined: %q", out)
	}
}
