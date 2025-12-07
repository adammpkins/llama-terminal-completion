package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Injectable functions for testing
var (
	// getOS returns the operating system name (can be overridden in tests)
	getOS = func() string { return runtime.GOOS }

	// lookPath wraps exec.LookPath (can be overridden in tests)
	lookPath = exec.LookPath

	// runClipboardCmd executes the clipboard command (can be overridden in tests)
	runClipboardCmd = func(cmd *exec.Cmd) error { return cmd.Run() }
)

// copyToClipboard copies text to the system clipboard
func copyToClipboard(text string) error {
	cmd, err := getClipboardCommand()
	if err != nil {
		return err
	}

	cmd.Stdin = strings.NewReader(text)
	return runClipboardCmd(cmd)
}

// getClipboardCommand returns the appropriate clipboard command for the current OS
func getClipboardCommand() (*exec.Cmd, error) {
	switch getOS() {
	case "darwin":
		return exec.Command("pbcopy"), nil
	case "linux":
		// Try xclip first, then xsel
		if _, err := lookPath("xclip"); err == nil {
			return exec.Command("xclip", "-selection", "clipboard"), nil
		} else if _, err := lookPath("xsel"); err == nil {
			return exec.Command("xsel", "--clipboard", "--input"), nil
		} else {
			return nil, fmt.Errorf("no clipboard tool found (install xclip or xsel)")
		}
	case "windows":
		return exec.Command("cmd", "/c", "clip"), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", getOS())
	}
}

// extractCodeBlocks extracts code blocks from markdown text
func extractCodeBlocks(text string) []string {
	var blocks []string
	lines := strings.Split(text, "\n")
	var inBlock bool
	var currentBlock strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inBlock {
				// End of block
				blocks = append(blocks, strings.TrimSpace(currentBlock.String()))
				currentBlock.Reset()
				inBlock = false
			} else {
				// Start of block
				inBlock = true
			}
		} else if inBlock {
			currentBlock.WriteString(line)
			currentBlock.WriteString("\n")
		}
	}

	return blocks
}
