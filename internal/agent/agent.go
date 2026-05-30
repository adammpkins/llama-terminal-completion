// Package agent implements a bounded, confirmation-gated tool-use loop on top
// of the OpenAI-compatible client. It is intentionally not a fully autonomous
// coding agent: write/exec tools require confirmation, iterations are capped,
// and tool output is truncated.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/tools"
)

// DefaultMaxIterations bounds the number of model round-trips.
const DefaultMaxIterations = 12

// DefaultSystemPrompt guides the model toward concise, tool-driven behavior.
const DefaultSystemPrompt = `You are a capable terminal assistant operating in an agentic loop.
You can call tools to read and modify files, run shell commands, and fetch URLs.
Plan briefly, then act using the available tools. Prefer reading before writing.
When the task is complete, stop calling tools and give a short summary of what you did.`

// ConfirmFunc asks the user to approve a side-effecting action. action is a
// short label (e.g. the tool name) and detail is the specifics (command text
// or a diff preview). It returns true to proceed.
type ConfirmFunc func(action, detail string) (bool, error)

// Agent runs the tool-use loop.
type Agent struct {
	Client        *client.Client
	Registry      *tools.Registry
	MaxIterations int
	MaxTokens     int
	Temperature   float64
	SystemPrompt  string
	// Confirm is called before executing any tool whose Risk is >= RiskWrite.
	// It blocks the calling goroutine until the user decides; frontends choose
	// how (the REPL reads stdin; the TUI blocks on a channel filled by the UI).
	// If nil, such tools are auto-approved.
	Confirm ConfirmFunc
	// Emit receives structured events as the turn progresses. The core never
	// writes to stdout directly — frontends render these. If nil, events are
	// dropped (headless use).
	Emit func(Event)
	// Audit, if set, is called once per tool invocation for logging.
	Audit func(AuditEntry)
	// Stream, when true, streams the model's natural-language turns as
	// EventToken events (tool calls are still reassembled before execution).
	Stream bool
}

// emit delivers an event to the configured handler, if any.
func (a *Agent) emit(e Event) {
	if a.Emit != nil {
		a.Emit(e)
	}
}

// Run executes the loop for the given task and returns the model's final
// natural-language response.
func (a *Agent) Run(ctx context.Context, task string) (string, error) {
	sys := a.SystemPrompt
	if sys == "" {
		sys = DefaultSystemPrompt
	}
	messages := []client.ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: task},
	}
	_, out, err := a.loop(ctx, messages)
	return out, err
}

// RunTurn runs one agentic turn against an existing conversation history (which
// must already include a system message and the latest user message). It
// returns the updated history (including the assistant's reply and any tool
// messages) and the assistant's final content. This is what the agentic chat
// REPL uses to preserve context across turns.
func (a *Agent) RunTurn(ctx context.Context, history []client.ChatMessage) ([]client.ChatMessage, string, error) {
	return a.loop(ctx, history)
}

// loop drives the tool-use loop over the given messages, returning the final
// messages and the assistant's last content.
func (a *Agent) loop(ctx context.Context, messages []client.ChatMessage) ([]client.ChatMessage, string, error) {
	maxIter := a.MaxIterations
	if maxIter <= 0 {
		maxIter = DefaultMaxIterations
	}
	defs := a.Registry.Definitions()

	a.emit(Event{Kind: EventStarted})

	nudged := false
	for iter := 0; iter < maxIter; iter++ {
		opts := client.CompletionOptions{
			MaxTokens:   a.MaxTokens,
			Temperature: a.Temperature,
			Tools:       defs,
			ToolChoice:  "auto",
		}

		var msg client.ChatMessage
		if a.Stream {
			// Stream natural-language content live; tool calls are reassembled.
			res, err := a.Client.ChatCompletionStreamWithOptions(messages, opts, func(s string) {
				a.emit(Event{Kind: EventToken, Token: s})
			})
			if err != nil {
				a.emit(Event{Kind: EventError, Err: err})
				return messages, "", err
			}
			msg = client.ChatMessage{Role: "assistant", Content: res.Content, ToolCalls: res.ToolCalls}
		} else {
			resp, err := a.Client.ChatCompletionWithOptions(messages, opts)
			if err != nil {
				a.emit(Event{Kind: EventError, Err: err})
				return messages, "", err
			}
			if len(resp.Choices) == 0 {
				err := fmt.Errorf("model returned no choices")
				a.emit(Event{Kind: EventError, Err: err})
				return messages, "", err
			}
			msg = resp.Choices[0].Message
			if strings.TrimSpace(msg.Content) != "" {
				a.emit(Event{Kind: EventToken, Token: msg.Content})
			}
		}
		messages = append(messages, msg)

		// No tool calls => the model is done...
		if len(msg.ToolCalls) == 0 {
			// ...unless it returned an empty final turn. Some reasoning models
			// intermittently emit no content and no tool calls mid-task; nudge
			// once before giving up to avoid stopping prematurely.
			if strings.TrimSpace(msg.Content) == "" && !nudged {
				nudged = true
				messages = append(messages, client.ChatMessage{
					Role:    "user",
					Content: "Continue with the task. If it is already complete, briefly summarize what you did.",
				})
				continue
			}
			a.emit(Event{Kind: EventDone, Final: msg.Content})
			return messages, msg.Content, nil
		}
		nudged = false // made progress via tools; reset the nudge budget

		// Execute each requested tool call and append a result message.
		for _, call := range msg.ToolCalls {
			result := a.runToolCall(ctx, call)
			messages = append(messages, client.ChatMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    result,
			})
		}
	}

	err := fmt.Errorf("agent stopped after reaching max iterations (%d)", maxIter)
	a.emit(Event{Kind: EventError, Err: err})
	return messages, "", err
}

// AuditEntry records a single tool invocation for the audit log.
type AuditEntry struct {
	Tool     string `json:"tool"`
	Args     string `json:"args"`
	Decision string `json:"decision"` // "executed" | "declined" | "error" | "unknown_tool"
	Result   string `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
}

// runToolCall resolves, optionally confirms, and executes a single tool call,
// returning the string to feed back to the model (errors included as text so
// the model can adapt rather than the loop aborting). Each call is recorded via
// the optional Audit hook.
func (a *Agent) runToolCall(ctx context.Context, call client.ToolCall) string {
	name := call.Function.Name
	args := json.RawMessage(call.Function.Arguments)
	entry := AuditEntry{Tool: name, Args: call.Function.Arguments}

	tool, ok := a.Registry.Get(name)
	if !ok {
		entry.Decision = "unknown_tool"
		out := fmt.Sprintf("Error: unknown tool %q", name)
		entry.Error = out
		a.audit(entry)
		return out
	}

	a.emit(Event{Kind: EventToolRequested, Tool: name, Args: summarizeArgs(call.Function.Arguments)})

	if tool.Risk() >= tools.RiskWrite && a.Confirm != nil {
		ok, err := a.Confirm(name, confirmDetail(name, args))
		if err != nil {
			entry.Decision = "error"
			entry.Error = err.Error()
			a.audit(entry)
			a.emit(Event{Kind: EventToolResult, Tool: name, Result: "error obtaining confirmation", Err: err})
			return fmt.Sprintf("Error obtaining confirmation: %v", err)
		}
		if !ok {
			entry.Decision = "declined"
			a.audit(entry)
			a.emit(Event{Kind: EventToolResult, Tool: name, Declined: true})
			return "The user declined to run this tool. Consider an alternative approach or ask for clarification."
		}
	}

	out, err := tool.Execute(ctx, args)
	if err != nil {
		entry.Decision = "error"
		entry.Error = err.Error()
		entry.Result = truncateAudit(out)
		a.audit(entry)
		a.emit(Event{Kind: EventToolResult, Tool: name, Result: out, Err: err})
		// Include any partial output the tool returned.
		if out != "" {
			return fmt.Sprintf("Error: %v\nOutput:\n%s", err, out)
		}
		return fmt.Sprintf("Error: %v", err)
	}
	entry.Decision = "executed"
	entry.Result = truncateAudit(out)
	a.audit(entry)
	a.emit(Event{Kind: EventToolResult, Tool: name, Result: out})
	return out
}

// audit invokes the Audit hook if one is configured.
func (a *Agent) audit(e AuditEntry) {
	if a.Audit != nil {
		a.Audit(e)
	}
}

// truncateAudit bounds the size of audited result text.
func truncateAudit(s string) string {
	const max = 2000
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// summarizeArgs renders a compact one-line view of tool arguments for display.
func summarizeArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return ""
	}
	const max = 120
	raw = strings.ReplaceAll(raw, "\n", " ")
	if len(raw) > max {
		return raw[:max] + "…"
	}
	return raw
}

// confirmDetail produces a human-readable detail string for the confirm prompt.
func confirmDetail(name string, args json.RawMessage) string {
	switch name {
	case "run_command":
		var a struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(args, &a) == nil && a.Command != "" {
			return a.Command
		}
	case "edit_file":
		var a struct {
			Path string `json:"path"`
			Old  string `json:"old"`
			New  string `json:"new"`
		}
		if json.Unmarshal(args, &a) == nil {
			return fmt.Sprintf("%s\n%s", a.Path, tools.PreviewEdit(a.Old, a.New))
		}
	case "write_file":
		var a struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if json.Unmarshal(args, &a) == nil {
			return fmt.Sprintf("%s (%d bytes)", a.Path, len(a.Content))
		}
	}
	return string(args)
}
