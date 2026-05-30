package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/fetch"
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

// Reference-expansion limits.
const (
	maxRefBytes   = 32 * 1024  // per-reference cap
	maxTotalBytes = 128 * 1024 // total inlined cap
)

// referenceFetchFunc is injectable for tests so @url expansion need not hit the network.
var referenceFetchFunc = fetch.Fetch

// refPattern matches an @reference at a word boundary: start-of-string or
// whitespace, then @, then a run of non-whitespace. An email like a@b.com is
// not matched because the @ is preceded by a non-space character.
var refPattern = regexp.MustCompile(`(^|\s)@(\S+)`)

// expandReferences scans input for @file, @glob, and @http(s) references,
// appending the resolved content as labeled blocks after the original text. A
// literal @ can be written as \@. The original text is preserved; unknown or
// unreadable references are left untouched (and noted), so this is safe to run
// on any input.
func expandReferences(input string) (string, error) {
	if !strings.Contains(input, "@") {
		return input, nil
	}

	var blocks []string
	total := 0
	seen := map[string]bool{}

	matches := refPattern.FindAllStringSubmatch(input, -1)
	for _, m := range matches {
		lead, token := m[1], m[2]
		// Honor an escaped \@ (the backslash will be the trailing char of lead
		// or directly precede the token in the original — handled below).
		if strings.HasSuffix(lead, "\\") {
			continue
		}
		token = strings.TrimRight(token, ".,;:)]}\"'")
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true

		content, label, err := resolveReference(token)
		if err != nil {
			blocks = append(blocks, fmt.Sprintf("[@%s]\n(could not load: %v)", token, err))
			continue
		}
		if len(content) > maxRefBytes {
			content = content[:maxRefBytes] + "\n... [truncated]"
		}
		if total+len(content) > maxTotalBytes {
			blocks = append(blocks, fmt.Sprintf("[@%s]\n(skipped: total reference size limit reached)", token))
			continue
		}
		total += len(content)
		blocks = append(blocks, fmt.Sprintf("[%s]\n```\n%s\n```", label, content))
	}

	// Unescape \@ -> @ in the visible prompt text.
	out := strings.ReplaceAll(input, `\@`, "@")
	if len(blocks) == 0 {
		return out, nil
	}
	return out + "\n\n--- Referenced content ---\n" + strings.Join(blocks, "\n\n"), nil
}

// resolveReference loads a single @token, returning its content and a display
// label. URLs are fetched; everything else is treated as a file path or glob.
func resolveReference(token string) (content string, label string, err error) {
	if strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://") {
		data, err := referenceFetchFunc(context.Background(), token)
		if err != nil {
			return "", token, err
		}
		return data, token, nil
	}

	// File or glob path.
	matches, globErr := filepath.Glob(token)
	if globErr == nil && len(matches) > 0 {
		if len(matches) == 1 {
			data, err := os.ReadFile(matches[0])
			if err != nil {
				return "", token, err
			}
			return string(data), matches[0], nil
		}
		// Multiple files: concatenate with per-file headers.
		var b strings.Builder
		for _, p := range matches {
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			fmt.Fprintf(&b, "// %s\n%s\n", p, string(data))
		}
		return b.String(), token + " (" + fmt.Sprint(len(matches)) + " files)", nil
	}

	data, err := os.ReadFile(token)
	if err != nil {
		return "", token, err
	}
	return string(data), token, nil
}
