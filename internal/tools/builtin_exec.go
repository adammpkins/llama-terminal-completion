package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/adammpkins/llama-terminal-completion/internal/shellrun"
)

// runCommandTool executes a shell command and returns its combined output.
// It routes through shellrun so dangerous-command detection is shared with the
// `cmd` CLI command; the agent loop confirms RiskExec tools before calling.
type runCommandTool struct{ ws *Workspace }

func (t *runCommandTool) Name() string { return "run_command" }
func (t *runCommandTool) Description() string {
	return "Run a shell command and return its combined stdout and stderr."
}
func (t *runCommandTool) Risk() RiskLevel { return RiskExec }
func (t *runCommandTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The shell command to execute."}
		},
		"required": ["command"]
	}`)
}

func (t *runCommandTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Command string `json:"command"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if a.Command == "" {
		return "", fmt.Errorf("command is required")
	}
	out, err := shellrun.RunCapture(ctx, a.Command, t.ws.Shell)
	out = t.ws.truncate(out)
	if err != nil {
		// Return the output alongside the error message so the model can react.
		if out != "" {
			return out, err
		}
		return "", err
	}
	if out == "" {
		out = "(command produced no output)"
	}
	return out, nil
}
