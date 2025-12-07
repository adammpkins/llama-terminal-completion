package cli

import (
	"os"

	"github.com/charmbracelet/glamour"
)

// Global markdown renderer for non-TUI commands
var mdRenderer *glamour.TermRenderer

// initMarkdownRenderer initializes the global markdown renderer
func initMarkdownRenderer() {
	// Only initialize if stdout is a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(100),
		)
		if err == nil {
			mdRenderer = r
		}
	}
}

// renderMarkdown renders markdown content if a terminal is available
// Falls back to plain text if not
func renderMarkdown(content string) string {
	if mdRenderer == nil {
		return content
	}
	rendered, err := mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return rendered
}

// isTerminalOutput returns true if stdout is a terminal
func isTerminalOutput() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
