package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/adammpkins/llama-terminal-completion/internal/fetch"
)

// FetchFunc retrieves a URL and returns its text content. It is injectable so
// tests need not hit the network.
type FetchFunc func(ctx context.Context, url string) (string, error)

// webFetchTool retrieves a URL and returns its readable text.
type webFetchTool struct {
	ws    *Workspace
	fetch FetchFunc
}

func (t *webFetchTool) Name() string        { return "web_fetch" }
func (t *webFetchTool) Description() string { return "Fetch a URL and return its text content." }
func (t *webFetchTool) Risk() RiskLevel     { return RiskRead }
func (t *webFetchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "The http(s) URL to fetch."}
		},
		"required": ["url"]
	}`)
}

func (t *webFetchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		URL string `json:"url"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if a.URL == "" {
		return "", fmt.Errorf("url is required")
	}
	f := t.fetch
	if f == nil {
		f = fetch.Fetch
	}
	out, err := f(ctx, a.URL)
	if err != nil {
		return "", err
	}
	return t.ws.truncate(out), nil
}
