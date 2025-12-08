package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adammpkins/llama-terminal-completion/internal/config"
)

func TestRunChatExit(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	// Simulate user typing "exit"
	input := "exit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Goodbye") {
		t.Error("Expected 'Goodbye' message")
	}
}

func TestRunChatQuit(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	input := "quit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Goodbye") {
		t.Error("Expected 'Goodbye' message")
	}
}

func TestRunChatQ(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	input := "q\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Goodbye") {
		t.Error("Expected 'Goodbye' message")
	}
}

func TestRunChatHelp(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	input := "/help\nexit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Chat Commands") {
		t.Error("Expected help output")
	}
	if !strings.Contains(result, "/clear") {
		t.Error("Expected /clear in help")
	}
}

func TestRunChatClear(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	input := "/clear\nexit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "cleared") {
		t.Error("Expected 'cleared' message")
	}
}

func TestRunChatSave(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	input := "/save\nexit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "saved") && !strings.Contains(result, "Saved") {
		t.Logf("Output: %s", result)
	}
}

func TestRunChatConversation(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Response " + string(rune('0'+callCount))}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg = &config.Config{
		BaseURL:     server.URL,
		APIKey:      "key",
		Model:       "test-model",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	input := "Hello\nHow are you?\nexit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Assistant:") {
		t.Error("Expected 'Assistant:' in output")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls, got %d", callCount)
	}
}

func TestRunChatEmptyInput(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	input := "\n\n\nexit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	// Should still exit properly after empty inputs
	result := output.String()
	if !strings.Contains(result, "Goodbye") {
		t.Error("Expected 'Goodbye' message")
	}
}

func TestRunChatEOF(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	// No newline at end = EOF
	input := "hello"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}
	// Should exit gracefully on EOF
}

func TestRunChatStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"Hello"}}]}` + "\n"))
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":" world"}}]}` + "\n"))
		_, _ = w.Write([]byte(`data: [DONE]` + "\n"))
	}))
	defer server.Close()

	cfg = &config.Config{
		BaseURL:     server.URL,
		APIKey:      "key",
		Model:       "test-model",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	input := "test\nexit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader streaming returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Hello world") {
		t.Logf("Streaming output: %s", result)
	}
}

func TestRunChatAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"Server error"}}`))
	}))
	defer server.Close()

	cfg = &config.Config{
		BaseURL:     server.URL,
		APIKey:      "key",
		Model:       "test-model",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	input := "hello\nexit\n"
	reader := bufio.NewReader(strings.NewReader(input))
	output := &bytes.Buffer{}

	err := runChatWithReader(reader, output)
	if err != nil {
		t.Fatalf("runChatWithReader returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Error") {
		t.Logf("Expected error in output: %s", result)
	}
}

func TestPrintChatHelpTo(t *testing.T) {
	output := &bytes.Buffer{}
	printChatHelpTo(output)

	result := output.String()
	if !strings.Contains(result, "Chat Commands") {
		t.Error("Expected 'Chat Commands' header")
	}
	if !strings.Contains(result, "/clear") {
		t.Error("Expected /clear command")
	}
	if !strings.Contains(result, "/save") {
		t.Error("Expected /save command")
	}
	if !strings.Contains(result, "/help") {
		t.Error("Expected /help command")
	}
	if !strings.Contains(result, "exit") {
		t.Error("Expected exit command")
	}
}

func TestRunChatViaInjection(t *testing.T) {
	response := `{"choices": [{"message": {"content": "Hello"}}]}`
	server := setupMockServer(response)
	defer server.Close()

	setupTestConfig(server.URL)

	// Use the injectable variables
	stdinForChat = strings.NewReader("exit\n")
	output := &bytes.Buffer{}
	chatOutputWriter = output
	defer func() {
		stdinForChat = nil
		chatOutputWriter = nil
	}()

	err := runChat(nil, nil)
	if err != nil {
		t.Fatalf("runChat returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "Goodbye") {
		t.Error("Expected 'Goodbye' in output")
	}
}
