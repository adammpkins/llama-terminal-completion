// Package tools defines the Tool interface, a registry, and the built-in tools
// the agent can call. MCP-provided tools (see internal/mcp) implement the same
// interface and register here, so the agent loop treats all tools uniformly.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
)

// RiskLevel describes how dangerous a tool's side effects are. The agent uses
// it to decide when to require user confirmation.
type RiskLevel int

const (
	// RiskRead tools only read state (files, network) — no confirmation needed.
	RiskRead RiskLevel = iota
	// RiskWrite tools modify the filesystem — confirmation recommended.
	RiskWrite
	// RiskExec tools run arbitrary commands — confirmation strongly recommended.
	RiskExec
)

// Tool is a single callable capability exposed to the model.
type Tool interface {
	Name() string
	Description() string
	// Parameters returns the JSON Schema for the tool's arguments.
	Parameters() json.RawMessage
	// Risk reports the tool's side-effect level.
	Risk() RiskLevel
	// Execute runs the tool with the given JSON-encoded arguments and returns a
	// string result to feed back to the model.
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry holds a set of tools keyed by name, preserving registration order.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool, replacing any existing tool with the same name.
func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; !exists {
		r.order = append(r.order, t.Name())
	}
	r.tools[t.Name()] = t
}

// Get returns the tool registered under name, if any.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns the registered tools in registration order.
func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// Definitions converts the registered tools into the wire format the client
// sends to the model.
func (r *Registry) Definitions() []client.Tool {
	defs := make([]client.Tool, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		defs = append(defs, client.Tool{
			Type: "function",
			Function: client.FunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

// Filter returns a new registry containing only the named tools. Unknown names
// are ignored. Order follows the requested names.
func (r *Registry) Filter(names []string) *Registry {
	sub := NewRegistry()
	for _, n := range names {
		if t, ok := r.tools[n]; ok {
			sub.Register(t)
		}
	}
	return sub
}

// decodeArgs is a small helper for built-in tools to unmarshal their arguments.
func decodeArgs(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}
