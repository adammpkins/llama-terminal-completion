package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// readFileTool reads a UTF-8 text file within the workspace.
type readFileTool struct{ ws *Workspace }

func (t *readFileTool) Name() string        { return "read_file" }
func (t *readFileTool) Description() string { return "Read the contents of a text file." }
func (t *readFileTool) Risk() RiskLevel     { return RiskRead }
func (t *readFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the file, relative to the working directory."}
		},
		"required": ["path"]
	}`)
}

func (t *readFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
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
	return t.ws.truncate(string(data)), nil
}

// listDirTool lists the entries of a directory within the workspace.
type listDirTool struct{ ws *Workspace }

func (t *listDirTool) Name() string { return "list_dir" }
func (t *listDirTool) Description() string {
	return "List the files and subdirectories of a directory."
}
func (t *listDirTool) Risk() RiskLevel { return RiskRead }
func (t *listDirTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path, relative to the working directory. Defaults to '.'."}
		}
	}`)
}

func (t *listDirTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if a.Path == "" {
		a.Path = "."
	}
	abs, err := t.ws.resolve(a.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", fmt.Errorf("list %s: %w", a.Path, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += string(filepath.Separator)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(empty directory)", nil
	}
	return t.ws.truncate(strings.Join(names, "\n")), nil
}
