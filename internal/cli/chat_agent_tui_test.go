package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adammpkins/llama-terminal-completion/internal/agent"
	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/tools"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// newTestTUIModel builds a model sized and ready, without a real terminal.
func newTestTUIModel() agentChatModel {
	m := agentChatModel{
		events:   make(chan tea.Msg, 16),
		decision: make(chan bool, 1),
		textarea: textarea.New(),
		spinner:  spinner.New(),
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(agentChatModel)
}

func TestTUIHandlesTokenAndToolEvents(t *testing.T) {
	m := newTestTUIModel()

	m2, _ := m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventToolRequested, Tool: "read_file", Args: `{"path":"a"}`}})
	m = m2.(agentChatModel)
	if m.toolCount != 1 {
		t.Errorf("toolCount = %d, want 1", m.toolCount)
	}

	// Stream several tokens across separate Update calls. Each Update copies the
	// model by value, so this guards against storing a strings.Builder (which
	// panics when copied after first use) in the model.
	for _, tok := range []string{"hel", "lo ", "world"} {
		m2, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventToken, Token: tok}})
		m = m2.(agentChatModel)
	}
	if m.stream != "hello world" {
		t.Errorf("streamed text = %q, want %q", m.stream, "hello world")
	}

	m2, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventDone}})
	m = m2.(agentChatModel)
	if m.stream != "" {
		t.Errorf("stream should be flushed on done, got %q", m.stream)
	}

	view := m.View()
	if view == "" {
		t.Fatal("empty view")
	}
}

func TestTUIEmptyTurnShowsNote(t *testing.T) {
	m := newTestTUIModel()
	// A turn that starts and ends with no tokens and no tools (reasoning model
	// that exhausted its budget) must surface a note, not a blank screen.
	for _, ev := range []agent.Event{{Kind: agent.EventStarted}, {Kind: agent.EventDone}} {
		mi, _ := m.Update(agentEventMsg{ev: ev})
		m = mi.(agentChatModel)
	}
	joined := strings.Join(m.blocks, "\n")
	if !strings.Contains(joined, "no response") {
		t.Errorf("expected an empty-turn note, got blocks:\n%s", joined)
	}
}

func TestTUIConfirmFlowUnblocksWorker(t *testing.T) {
	m := newTestTUIModel()

	// A confirmation request puts the UI into confirm mode.
	m2, _ := m.Update(agentConfirmMsg{action: "run_command", detail: "ls"})
	m = m2.(agentChatModel)
	if !m.awaitConfirm {
		t.Fatal("expected awaitConfirm after confirm request")
	}

	// Pressing 'y' answers the worker (buffered decision chan) and exits mode.
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = m2.(agentChatModel)
	if m.awaitConfirm {
		t.Error("expected confirm mode to clear after answering")
	}
	select {
	case d := <-m.decision:
		if !d {
			t.Error("expected approval (true) on decision channel")
		}
	default:
		t.Error("expected a decision to be sent to the worker")
	}
}

// TestTUIRunsTurnEndToEnd drives a full agentic turn through the Bubble Tea
// model with a real agent goroutine and a scripted server, pumping the event
// loop the way the runtime would. This catches wiring bugs (events not flowing,
// turn not running) that the panic/confirm tests don't.
func TestTUIRunsTurnEndToEnd(t *testing.T) {
	i := 0
	responses := []string{
		`{"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"list_dir","arguments":"{\"path\":\".\"}"}}]}}]}`,
		`{"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"All done."}}]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if i < len(responses) {
			_, _ = w.Write([]byte(responses[i]))
			i++
		}
	}))
	defer srv.Close()
	setupTestConfig(srv.URL)
	cfg.Stream = false

	events := make(chan tea.Msg, 256)
	decision := make(chan bool, 1)
	ws := &tools.Workspace{Root: t.TempDir(), Shell: "/bin/sh"}
	ag := &agent.Agent{
		Client:   client.NewClient(srv.URL, "k", "test-model"),
		Registry: tools.DefaultRegistry(ws, nil),
		Emit:     func(e agent.Event) { events <- agentEventMsg{ev: e} },
	}

	m := agentChatModel{
		ag:       ag,
		model:    "test-model",
		messages: []client.ChatMessage{{Role: "system", Content: "sys"}},
		events:   events,
		decision: decision,
		textarea: textarea.New(),
		spinner:  spinner.New(),
	}
	mi, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mi.(agentChatModel)

	// Type a task and submit.
	m.textarea.SetValue("list the dir")
	mi, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mi.(agentChatModel)
	if !m.running {
		t.Fatal("expected running after submit")
	}

	// Pump the event loop until the turn completes (the runtime does this via
	// the outstanding waitForAgentMsg command).
	for range 50 {
		msg := waitForAgentMsg(events)()
		mi, _ = m.Update(msg)
		m = mi.(agentChatModel)
		if _, done := msg.(agentTurnDoneMsg); done {
			break
		}
	}

	if m.running {
		t.Error("turn should have completed")
	}
	if m.toolCount != 1 {
		t.Errorf("expected 1 tool call, got %d", m.toolCount)
	}
	transcript := strings.Join(m.blocks, "\n")
	if !strings.Contains(transcript, "list_dir") {
		t.Errorf("transcript missing tool line; got:\n%s", transcript)
	}
	if !strings.Contains(transcript, "All done.") {
		t.Errorf("transcript missing final answer; got:\n%s", transcript)
	}
}

func TestTUIConfirmDeclineSendsFalse(t *testing.T) {
	m := newTestTUIModel()
	m2, _ := m.Update(agentConfirmMsg{action: "write_file", detail: "x"})
	m = m2.(agentChatModel)
	// Any non-'y' key declines.
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = m2.(agentChatModel)
	select {
	case d := <-m.decision:
		if d {
			t.Error("expected decline (false)")
		}
	default:
		t.Error("expected a decision on the channel")
	}
}
