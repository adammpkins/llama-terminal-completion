package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn while capturing everything written to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}

func TestRunCompleteCleansOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Model wraps the command in a fenced block; complete must strip it.
		_, _ = w.Write([]byte("{\"choices\":[{\"index\":0,\"message\":{\"role\":\"assistant\",\"content\":\"```bash\\nls -la\\n```\"}}]}"))
	}))
	defer server.Close()
	setupTestConfig(server.URL)

	out := captureStdout(t, func() {
		_ = runComplete(completeCmd, []string{"list files in detail"})
	})
	if out != "ls -la" {
		t.Errorf("complete output = %q, want %q", out, "ls -la")
	}
}

func TestRunCompleteSilentOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	setupTestConfig(server.URL)

	out := captureStdout(t, func() {
		if err := runComplete(completeCmd, []string{"anything"}); err != nil {
			t.Errorf("complete should stay silent on error, got %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected empty output on error, got %q", out)
	}
}

func TestWidgetScripts(t *testing.T) {
	cases := map[string]string{
		"zsh":  "bindkey '^G'",
		"bash": `bind -x '"\C-g"`,
		"fish": `bind \cg`,
	}
	for shell, marker := range cases {
		out := captureStdout(t, func() {
			_ = widgetCmd.RunE(widgetCmd, []string{shell})
		})
		if !strings.Contains(out, marker) {
			t.Errorf("%s widget missing %q; got:\n%s", shell, marker, out)
		}
		if !strings.Contains(out, "lt complete") {
			t.Errorf("%s widget should call 'lt complete'", shell)
		}
	}
}
