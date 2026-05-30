package agent

// EventKind identifies the type of an agent Event.
type EventKind int

const (
	// EventStarted is emitted once when a turn begins.
	EventStarted EventKind = iota
	// EventToken carries a chunk of the assistant's natural-language output.
	EventToken
	// EventToolRequested is emitted when the model asks to call a tool, before
	// any confirmation or execution.
	EventToolRequested
	// EventToolResult is emitted after a tool runs (or is declined/errors).
	EventToolResult
	// EventError carries a fatal error that ends the turn.
	EventError
	// EventDone is emitted once when the turn completes, with the final content.
	EventDone
)

// Event is a structured signal emitted by the agent core. Frontends (the plain
// REPL and the Bubble Tea TUI) render these; the core never writes to stdout
// directly. This is what lets one agent back a polished UI and a line REPL.
type Event struct {
	Kind EventKind

	// Token holds assistant text for EventToken.
	Token string

	// Tool/Args describe a tool call for EventToolRequested and EventToolResult.
	Tool string
	Args string

	// Result is the tool's output for EventToolResult; Declined marks a
	// user-declined call.
	Result   string
	Declined bool

	// Final is the assistant's final content for EventDone.
	Final string

	// Err is the failure for EventError.
	Err error
}
