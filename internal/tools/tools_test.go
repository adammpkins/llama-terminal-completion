package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newWS(t *testing.T) *Workspace {
	t.Helper()
	return &Workspace{Root: t.TempDir(), Shell: "/bin/sh"}
}

func TestReadFileTool(t *testing.T) {
	ws := newWS(t)
	if err := os.WriteFile(filepath.Join(ws.Root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &readFileTool{ws: ws}
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Errorf("got %q, want hello", out)
	}
}

func TestWorkspaceBoundary(t *testing.T) {
	ws := newWS(t)
	tool := &readFileTool{ws: ws}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../etc/passwd"}`)); err == nil {
		t.Error("expected boundary error for path escaping workspace")
	}

	ws.AllowOutside = true
	// Now it should attempt the read (and fail only because the file may not
	// exist) rather than be rejected by the boundary.
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"/nonexistent-xyz"}`))
	if err != nil && strings.Contains(err.Error(), "outside the workspace") {
		t.Error("AllowOutside should bypass the boundary check")
	}
}

func TestWriteAndEditFileTool(t *testing.T) {
	ws := newWS(t)
	w := &writeFileTool{ws: ws}
	if _, err := w.Execute(context.Background(), json.RawMessage(`{"path":"sub/b.txt","content":"one two"}`)); err != nil {
		t.Fatalf("write error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws.Root, "sub", "b.txt"))
	if err != nil || string(data) != "one two" {
		t.Fatalf("file not written correctly: %q err=%v", data, err)
	}

	e := &editFileTool{ws: ws}
	if _, err := e.Execute(context.Background(), json.RawMessage(`{"path":"sub/b.txt","old":"two","new":"three"}`)); err != nil {
		t.Fatalf("edit error: %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(ws.Root, "sub", "b.txt"))
	if string(data) != "one three" {
		t.Errorf("edit result %q, want 'one three'", data)
	}

	// Editing a non-existent 'old' should error.
	if _, err := e.Execute(context.Background(), json.RawMessage(`{"path":"sub/b.txt","old":"zzz","new":"x"}`)); err == nil {
		t.Error("expected error when 'old' text not found")
	}

	// Ambiguous (non-unique) 'old' should error rather than edit the wrong spot.
	if _, err := w.Execute(context.Background(), json.RawMessage(`{"path":"dup.txt","content":"x x x"}`)); err != nil {
		t.Fatalf("write dup error: %v", err)
	}
	if _, err := e.Execute(context.Background(), json.RawMessage(`{"path":"dup.txt","old":"x","new":"y"}`)); err == nil {
		t.Error("expected error for ambiguous (multiple) match")
	}
}

func TestRunCommandTool(t *testing.T) {
	ws := newWS(t)
	tool := &runCommandTool{ws: ws}
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo hi"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hi") {
		t.Errorf("got %q, want output containing hi", out)
	}
}

func TestWebFetchToolUsesInjectedFetcher(t *testing.T) {
	ws := newWS(t)
	called := false
	tool := &webFetchTool{ws: ws, fetch: func(ctx context.Context, url string) (string, error) {
		called = true
		return "fetched:" + url, nil
	}}
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"url":"http://example.com"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called || out != "fetched:http://example.com" {
		t.Errorf("injected fetcher not used; got %q", out)
	}
}

func TestRegistryDefinitionsAndFilter(t *testing.T) {
	ws := newWS(t)
	r := DefaultRegistry(ws, nil)
	defs := r.Definitions()
	if len(defs) != len(r.List()) || len(defs) == 0 {
		t.Fatalf("definitions count mismatch: %d vs %d", len(defs), len(r.List()))
	}
	for _, d := range defs {
		if d.Type != "function" || d.Function.Name == "" {
			t.Errorf("bad definition: %+v", d)
		}
	}

	sub := r.Filter([]string{"read_file", "nope"})
	if len(sub.List()) != 1 {
		t.Errorf("filter expected 1 tool, got %d", len(sub.List()))
	}
	if _, ok := sub.Get("read_file"); !ok {
		t.Error("filtered registry missing read_file")
	}
}

func TestTruncate(t *testing.T) {
	ws := &Workspace{Root: ".", MaxOutput: 5}
	out := ws.truncate("abcdefghij")
	if !strings.HasPrefix(out, "abcde") || !strings.Contains(out, "truncated") {
		t.Errorf("unexpected truncation: %q", out)
	}
}
