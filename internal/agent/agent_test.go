package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/adammpkins/llama-terminal-completion/internal/tools"
)

// scriptedServer returns canned chat-completion responses in sequence.
func scriptedServer(t *testing.T, responses []string) *httptest.Server {
	t.Helper()
	i := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if i >= len(responses) {
			t.Errorf("server received more requests than scripted (%d)", i+1)
			http.Error(w, "no more responses", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responses[i]))
		i++
	}))
}

func toolCallResp(id, name, args string) string {
	b, _ := json.Marshal(args)
	return `{"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":"",` +
		`"tool_calls":[{"id":"` + id + `","type":"function","function":{"name":"` + name + `","arguments":` + string(b) + `}}]}}]}`
}

func finalResp(content string) string {
	b, _ := json.Marshal(content)
	return `{"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":` + string(b) + `}}]}`
}

func newAgent(t *testing.T, serverURL string) (*Agent, *tools.Workspace) {
	t.Helper()
	ws := &tools.Workspace{Root: t.TempDir(), Shell: "/bin/sh"}
	reg := tools.DefaultRegistry(ws, nil)
	return &Agent{
		Client:        client.NewClient(serverURL, "k", "test-model"),
		Registry:      reg,
		MaxIterations: 5,
	}, ws
}

func TestAgentRunsToolThenFinishes(t *testing.T) {
	srv := scriptedServer(t, []string{
		toolCallResp("c1", "write_file", `{"path":"out.txt","content":"data"}`),
		finalResp("Created out.txt as requested."),
	})
	defer srv.Close()

	a, ws := newAgent(t, srv.URL)
	approved := false
	a.Confirm = func(action, detail string) (bool, error) {
		approved = true
		return true, nil
	}

	out, err := a.Run(context.Background(), "make a file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Created out.txt") {
		t.Errorf("unexpected final answer: %q", out)
	}
	if !approved {
		t.Error("expected write_file to trigger confirmation")
	}
	if data, _ := os.ReadFile(filepath.Join(ws.Root, "out.txt")); string(data) != "data" {
		t.Errorf("file not written: %q", data)
	}
}

func TestAgentDeclinePath(t *testing.T) {
	srv := scriptedServer(t, []string{
		toolCallResp("c1", "run_command", `{"command":"rm -rf /"}`),
		finalResp("Understood, I won't run that."),
	})
	defer srv.Close()

	a, _ := newAgent(t, srv.URL)
	a.Confirm = func(action, detail string) (bool, error) { return false, nil }

	out, err := a.Run(context.Background(), "delete everything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "won't run") {
		t.Errorf("unexpected final answer: %q", out)
	}
}

func TestAgentNudgesOnEmptyTurn(t *testing.T) {
	// First response: no tool calls, empty content (premature stop).
	// After the nudge, the model gives a real answer.
	srv := scriptedServer(t, []string{
		finalResp(""),
		finalResp("All done — nothing more to do."),
	})
	defer srv.Close()

	a, _ := newAgent(t, srv.URL)
	out, err := a.Run(context.Background(), "do the thing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "All done") {
		t.Errorf("expected nudge to recover a real answer, got %q", out)
	}
}

func TestAgentAuditHook(t *testing.T) {
	srv := scriptedServer(t, []string{
		toolCallResp("c1", "list_dir", `{"path":"."}`),
		finalResp("done"),
	})
	defer srv.Close()

	a, _ := newAgent(t, srv.URL)
	var entries []AuditEntry
	a.Audit = func(e AuditEntry) { entries = append(entries, e) }

	if _, err := a.Run(context.Background(), "list things"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].Tool != "list_dir" || entries[0].Decision != "executed" {
		t.Errorf("unexpected audit entry: %+v", entries[0])
	}
}

func TestAgentEmitsEventSequence(t *testing.T) {
	srv := scriptedServer(t, []string{
		toolCallResp("c1", "list_dir", `{"path":"."}`),
		finalResp("All set."),
	})
	defer srv.Close()

	a, _ := newAgent(t, srv.URL)
	var kinds []EventKind
	var final string
	a.Emit = func(e Event) {
		kinds = append(kinds, e.Kind)
		if e.Kind == EventDone {
			final = e.Final
		}
	}

	if _, err := a.Run(context.Background(), "list"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: Started, ToolRequested, ToolResult, Token(final), Done.
	want := []EventKind{EventStarted, EventToolRequested, EventToolResult, EventToken, EventDone}
	if len(kinds) != len(want) {
		t.Fatalf("event kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("event[%d] = %d, want %d (full: %v)", i, kinds[i], want[i], kinds)
		}
	}
	if final != "All set." {
		t.Errorf("EventDone.Final = %q", final)
	}
}

func TestAgentEmitsDeclinedToolResult(t *testing.T) {
	srv := scriptedServer(t, []string{
		toolCallResp("c1", "write_file", `{"path":"x","content":"y"}`),
		finalResp("ok"),
	})
	defer srv.Close()

	a, _ := newAgent(t, srv.URL)
	a.Confirm = func(action, detail string) (bool, error) { return false, nil }
	var declined bool
	a.Emit = func(e Event) {
		if e.Kind == EventToolResult && e.Declined {
			declined = true
		}
	}
	if _, err := a.Run(context.Background(), "write"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !declined {
		t.Error("expected a declined EventToolResult")
	}
}

func TestAgentMaxIterations(t *testing.T) {
	// Always return a tool call so the loop never terminates naturally.
	loop := toolCallResp("c1", "list_dir", `{"path":"."}`)
	srv := scriptedServer(t, []string{loop, loop, loop, loop, loop})
	defer srv.Close()

	a, _ := newAgent(t, srv.URL)
	a.MaxIterations = 3
	_, err := a.Run(context.Background(), "loop forever")
	if err == nil || !strings.Contains(err.Error(), "max iterations") {
		t.Errorf("expected max-iterations error, got %v", err)
	}
}
