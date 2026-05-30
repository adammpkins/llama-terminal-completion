// Package fetch retrieves a URL and reduces HTML to readable plain text. It is
// shared by the web_fetch agent tool and the @url inline-reference expansion.
package fetch

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DefaultTimeout bounds a single fetch.
const DefaultTimeout = 30 * time.Second

var (
	scriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagRe       = regexp.MustCompile(`(?s)<[^>]+>`)
	blankLines  = regexp.MustCompile(`\n{3,}`)
)

// httpClient is overridable in tests.
var httpClient = &http.Client{Timeout: DefaultTimeout}

// Fetch GETs url and returns its body. HTML responses are reduced to plain
// text; other content types are returned as-is.
func Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	req.Header.Set("User-Agent", "llama-terminal-completion/lt")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MiB cap
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "html") || looksLikeHTML(body) {
		return HTMLToText(string(body)), nil
	}
	return string(body), nil
}

func looksLikeHTML(b []byte) bool {
	s := strings.ToLower(string(b[:min(len(b), 512)]))
	return strings.Contains(s, "<html") || strings.Contains(s, "<!doctype html")
}

// HTMLToText strips tags and collapses whitespace into readable plain text.
func HTMLToText(htmlStr string) string {
	s := scriptStyle.ReplaceAllString(htmlStr, " ")
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	// Normalize whitespace per line, then collapse runs of blank lines.
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	s = strings.Join(lines, "\n")
	s = blankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
