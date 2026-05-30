package cli

import (
	"testing"
	"time"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
)

func useTempSessions(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	origCfg := sessionsConfigDirFunc
	sessionsConfigDirFunc = func() (string, error) { return dir, nil }
	t.Cleanup(func() { sessionsConfigDirFunc = origCfg })
}

func TestSessionSaveLoadRoundTrip(t *testing.T) {
	useTempSessions(t)

	if msgs, err := loadSession("nope"); err != nil || msgs != nil {
		t.Fatalf("missing session should be empty/nil, got %v err=%v", msgs, err)
	}

	msgs := []client.ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	if err := saveSession("work", "test-model", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadSession("work")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 3 || loaded[2].Content != "hello" {
		t.Errorf("round-trip mismatch: %+v", loaded)
	}
}

func TestSessionListAndDelete(t *testing.T) {
	useTempSessions(t)
	sessionsNowFunc = func() time.Time { return time.Unix(1700000000, 0) }
	defer func() { sessionsNowFunc = time.Now }()

	_ = saveSession("a", "m", []client.ChatMessage{{Role: "user", Content: "x"}})
	_ = saveSession("b", "m", []client.ChatMessage{{Role: "user", Content: "y"}})

	sessions, err := listSessions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	if err := deleteSession("a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := deleteSession("a"); err == nil {
		t.Error("deleting a missing session should error")
	}
	sessions, _ = listSessions()
	if len(sessions) != 1 || sessions[0].Name != "b" {
		t.Errorf("expected only session b to remain, got %+v", sessions)
	}
}
