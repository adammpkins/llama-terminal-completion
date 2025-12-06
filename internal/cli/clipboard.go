package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// copyToClipboard copies text to the system clipboard
func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip or xsel)")
		}
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
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
