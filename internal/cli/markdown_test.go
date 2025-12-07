package cli

import (
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	// Initialize the renderer for testing
	initMarkdownRenderer()

	tests := []struct {
		name    string
		input   string
		wantLen bool // just check that output is non-empty
	}{
		{
			name:    "simple text",
			input:   "Hello world",
			wantLen: true,
		},
		{
			name:    "code block",
			input:   "```go\nfunc main() {}\n```",
			wantLen: true,
		},
		{
			name:    "heading",
			input:   "# Title\n\nSome content",
			wantLen: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantLen: false,
		},
		{
			name:    "bold and italic",
			input:   "**bold** and *italic*",
			wantLen: true,
		},
		{
			name:    "list",
			input:   "- item 1\n- item 2\n- item 3",
			wantLen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderMarkdown(tt.input)
			if tt.wantLen && len(result) == 0 {
				t.Error("Expected non-empty result")
			}
			if !tt.wantLen && tt.input == "" && result != "" {
				// Empty input should return empty output
				t.Logf("Empty input returned: %q", result)
			}
		})
	}
}

func TestRenderMarkdownFallback(t *testing.T) {
	// Save the current renderer and set to nil to test fallback
	oldRenderer := mdRenderer
	mdRenderer = nil
	defer func() { mdRenderer = oldRenderer }()

	input := "# Test\n\nThis should return as-is"
	result := renderMarkdown(input)

	if result != input {
		t.Errorf("Expected input to be returned unchanged when renderer is nil, got %q", result)
	}
}

func TestIsTerminalOutput(t *testing.T) {
	// This test just ensures the function runs without panic
	// The actual result depends on the test environment
	result := isTerminalOutput()
	t.Logf("isTerminalOutput returned: %v", result)
}

func TestInitMarkdownRenderer(t *testing.T) {
	// Reset renderer
	mdRenderer = nil

	// Initialize
	initMarkdownRenderer()

	// In test environment, stdout may or may not be a terminal
	// Just ensure it doesn't panic
	t.Logf("Renderer initialized: %v", mdRenderer != nil)
}
