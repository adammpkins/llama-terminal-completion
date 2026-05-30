package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
)

func TestExtractJSON(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"Here is the result:\n{\"a\":1}\nDone.", `{"a":1}`},
		{"```\n[1,2,3]\n```", `[1,2,3]`},
	}
	for _, c := range cases {
		if got := extractJSON(c.in); got != c.want {
			t.Errorf("extractJSON(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRequestNativeSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !bytes.Contains(body, []byte("json_schema")) {
			t.Errorf("expected json_schema in request, got %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"name\":\"Ann\"}"}}]}`))
	}))
	defer srv.Close()
	setupTestConfig(srv.URL)

	apiClient := client.NewClient(srv.URL, "k", "test-model")
	out, err := requestNativeSchema(apiClient, "extract", []byte(`{"type":"object"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `{"name":"Ann"}` {
		t.Errorf("got %q", out)
	}
}

func TestRequestPromptSchemaExtractsFencedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Model wraps JSON in a markdown fence and prose — fallback must extract it.
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"Sure!\n` + "```json\\n{\\\"ok\\\":true}\\n```" + `"}}]}`))
	}))
	defer srv.Close()
	setupTestConfig(srv.URL)

	apiClient := client.NewClient(srv.URL, "k", "test-model")
	out, err := requestPromptSchema(apiClient, "extract", []byte(`{"type":"object"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `{"ok":true}` {
		t.Errorf("got %q", out)
	}
}
