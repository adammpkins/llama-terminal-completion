package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeFileTool creates or overwrites a file within the workspace.
type writeFileTool struct{ ws *Workspace }

func (t *writeFileTool) Name() string { return "write_file" }
func (t *writeFileTool) Description() string {
	return "Create or overwrite a file with the given content."
}
func (t *writeFileTool) Risk() RiskLevel { return RiskWrite }
func (t *writeFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the file, relative to the working directory."},
			"content": {"type": "string", "description": "Full file content to write."}
		},
		"required": ["path", "content"]
	}`)
}

func (t *writeFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	abs, err := t.ws.resolve(a.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("create parent dirs: %w", err)
	}
	if err := os.WriteFile(abs, []byte(a.Content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", a.Path, err)
	}
	return fmt.Sprintf("Wrote %d bytes to %s", len(a.Content), a.Path), nil
}

// editFileTool replaces the first occurrence of a literal string in a file.
type editFileTool struct{ ws *Workspace }

func (t *editFileTool) Name() string { return "edit_file" }
func (t *editFileTool) Description() string {
	return "Replace the first occurrence of an exact string in a file with new text."
}
func (t *editFileTool) Risk() RiskLevel { return RiskWrite }
func (t *editFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the file, relative to the working directory."},
			"old": {"type": "string", "description": "Exact text to find (must be unique enough to locate)."},
			"new": {"type": "string", "description": "Replacement text."}
		},
		"required": ["path", "old", "new"]
	}`)
}

func (t *editFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
		Old  string `json:"old"`
		New  string `json:"new"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	abs, err := t.ws.resolve(a.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", a.Path, err)
	}
	content := string(data)
	if a.Old == "" {
		return "", fmt.Errorf("'old' must not be empty")
	}
	count := strings.Count(content, a.Old)
	switch count {
	case 0:
		return "", fmt.Errorf("'old' text not found in %s", a.Path)
	case 1:
		// unique match — proceed
	default:
		return "", fmt.Errorf("'old' text matches %d times in %s; include more surrounding context so it is unique", count, a.Path)
	}
	updated := strings.Replace(content, a.Old, a.New, 1)
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", a.Path, err)
	}
	return fmt.Sprintf("Edited %s\n%s", a.Path, PreviewEdit(a.Old, a.New)), nil
}

// PreviewEdit returns a short unified-style preview of an edit for confirmation
// prompts. It is best-effort and used by the agent's ConfirmFunc.
func PreviewEdit(old, new string) string {
	var b strings.Builder
	b.WriteString("- " + strings.ReplaceAll(strings.TrimRight(old, "\n"), "\n", "\n- ") + "\n")
	b.WriteString("+ " + strings.ReplaceAll(strings.TrimRight(new, "\n"), "\n", "\n+ "))
	return b.String()
}
