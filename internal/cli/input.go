package cli

import (
	"bufio"
	"io"
	"strings"
)

// readFromReader reads all content from a reader
func readFromReader(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		builder.WriteString(line)
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(builder.String()), nil
}
