package cli

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunAgenticChat_BasicTurn verifies the agentic chat REPL runs a turn
// through the agent and exits cleanly. The mock model returns a plain final
// answer (no tool calls), exercising the REPL/agent wiring.
func TestRunAgenticChat_BasicTurn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"Hello there."}}]}`))
	}))
	defer server.Close()
	setupTestConfig(server.URL)

	// Auto-approve and non-streaming for deterministic output.
	agentYes = true
	defer func() { agentYes = false }()
	cfg.Stream = false

	reader := bufio.NewReader(strings.NewReader("say hi\nexit\n"))
	var out strings.Builder
	if err := runAgenticChat(reader, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Agentic Chat") {
		t.Errorf("expected agentic chat banner, got: %q", got)
	}
	if !strings.Contains(got, "Hello there.") {
		t.Errorf("expected assistant reply in output, got: %q", got)
	}
	if !strings.Contains(got, "Goodbye") {
		t.Errorf("expected goodbye on exit, got: %q", got)
	}
}

func TestRunAgenticChat_ToolsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()
	setupTestConfig(server.URL)
	cfg.Stream = false

	reader := bufio.NewReader(strings.NewReader("/tools\nexit\n"))
	var out strings.Builder
	if err := runAgenticChat(reader, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The built-in toolset should be listed.
	for _, name := range []string{"read_file", "run_command", "web_fetch"} {
		if !strings.Contains(out.String(), name) {
			t.Errorf("/tools output missing %q; got:\n%s", name, out.String())
		}
	}
}
