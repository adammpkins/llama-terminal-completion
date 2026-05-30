package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/spf13/cobra"
)

// Session is a named, persistent conversation reused across invocations via the
// --session flag. Unlike auto-saved chat conversations, sessions are addressed
// by a stable user-chosen name.
type Session struct {
	Name      string               `json:"name"`
	Model     string               `json:"model"`
	UpdatedAt time.Time            `json:"updated_at"`
	Messages  []client.ChatMessage `json:"messages"`
}

// Injectable for testing.
var (
	sessionsConfigDirFunc = os.UserConfigDir
	sessionsHomeDirFunc   = os.UserHomeDir
	sessionsNowFunc       = time.Now
)

func getSessionsDir() string {
	if dir, err := sessionsConfigDirFunc(); err == nil {
		return filepath.Join(dir, "lt", "sessions")
	}
	if home, err := sessionsHomeDirFunc(); err == nil {
		return filepath.Join(home, ".lt", "sessions")
	}
	return filepath.Join(".", ".lt", "sessions")
}

func sessionPath(name string) string {
	return filepath.Join(getSessionsDir(), sanitizeFilename(name)+".json")
}

// loadSession returns the session's messages, or an empty slice if the session
// does not exist yet.
func loadSession(name string) ([]client.ChatMessage, error) {
	data, err := os.ReadFile(sessionPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session %q: %w", name, err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse session %q: %w", name, err)
	}
	return s.Messages, nil
}

// saveSession persists a session's messages.
func saveSession(name, model string, messages []client.ChatMessage) error {
	dir := getSessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}
	s := Session{Name: name, Model: model, UpdatedAt: sessionsNowFunc(), Messages: messages}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(sessionPath(name), data, 0o644); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

func listSessions() ([]Session, error) {
	entries, err := os.ReadDir(getSessionsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(getSessionsDir(), e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if json.Unmarshal(data, &s) == nil {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func deleteSession(name string) error {
	err := os.Remove(sessionPath(name))
	if os.IsNotExist(err) {
		return fmt.Errorf("session %q not found", name)
	}
	return err
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage named sessions (persistent context across runs)",
	Long: `Sessions keep conversation context across separate invocations. Pass
--session <name> to ask or agent to read and extend a session.

Examples:
  lt ask --session debugging "what does errno 13 mean?"
  lt ask --session debugging "and how do I fix it?"   # remembers the above
  lt session list`,
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := listSessions()
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions yet.")
			return nil
		}
		for _, s := range sessions {
			turns := 0
			for _, m := range s.Messages {
				if m.Role == "user" {
					turns++
				}
			}
			fmt.Printf("%-20s %2d turns  %s\n", s.Name, turns, s.UpdatedAt.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var sessionShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a session's messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		msgs, err := loadSession(args[0])
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			fmt.Printf("Session %q is empty or does not exist.\n", args[0])
			return nil
		}
		for _, m := range msgs {
			if m.Role == "system" {
				continue
			}
			fmt.Printf("\n[%s]\n%s\n", m.Role, m.Content)
		}
		return nil
	},
}

var sessionRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := deleteSession(args[0]); err != nil {
			return err
		}
		printSuccess("✓ Deleted session %q", args[0])
		return nil
	},
}

func init() {
	sessionCmd.AddCommand(sessionListCmd, sessionShowCmd, sessionRmCmd)
	rootCmd.AddCommand(sessionCmd)
}
