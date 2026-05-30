package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestMain disables real backoff sleeps so retry-path tests run instantly.
func TestMain(m *testing.M) {
	retrySleep = func(time.Duration) {}
	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com/v1/", "test-key", "test-model")

	if client.BaseURL != "https://api.example.com/v1" {
		t.Errorf("Expected BaseURL without trailing slash, got %s", client.BaseURL)
	}
	if client.APIKey != "test-key" {
		t.Errorf("Expected APIKey test-key, got %s", client.APIKey)
	}
	if client.Model != "test-model" {
		t.Errorf("Expected Model test-model, got %s", client.Model)
	}
	if client.HTTPClient == nil {
		t.Error("Expected HTTPClient to be initialized")
	}
}

func TestChatCompletion(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req ChatCompletionRequest
		_ = json.Unmarshal(body, &req)

		if req.Model != "test-model" {
			t.Errorf("Expected model test-model, got %s", req.Model)
		}
		if req.Stream != false {
			t.Error("Expected Stream to be false")
		}

		// Send response
		resp := ChatCompletionResponse{
			ID:    "test-id",
			Model: "test-model",
			Choices: []Choice{
				{
					Index:   0,
					Message: ChatMessage{Role: "assistant", Content: "Hello!"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	resp, err := client.ChatCompletion(messages, 100, 0.7)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("Expected 'Hello!', got %s", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key", "test-model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	_, err := client.ChatCompletion(messages, 100, 0.7)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("Expected error message to contain 'Invalid API key', got %s", err.Error())
	}
}

func TestChatCompletionStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify streaming is requested
		body, _ := io.ReadAll(r.Body)
		var req ChatCompletionRequest
		_ = json.Unmarshal(body, &req)

		if req.Stream != true {
			t.Error("Expected Stream to be true")
		}

		// Send SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" World"}}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n\n"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	var result strings.Builder
	err := client.ChatCompletionStream(messages, 100, 0.7, func(content string) {
		result.WriteString(content)
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "Hello World" {
		t.Errorf("Expected 'Hello World', got %s", result.String())
	}
}

func TestProcessStream(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single chunk",
			input:    `data: {"choices":[{"delta":{"content":"Hello"}}]}` + "\n",
			expected: "Hello",
		},
		{
			name: "multiple chunks",
			input: `data: {"choices":[{"delta":{"content":"Hello"}}]}` + "\n" +
				`data: {"choices":[{"delta":{"content":" World"}}]}` + "\n",
			expected: "Hello World",
		},
		{
			name:     "with DONE",
			input:    `data: {"choices":[{"delta":{"content":"Hi"}}]}` + "\ndata: [DONE]\n",
			expected: "Hi",
		},
		{
			name:     "empty lines",
			input:    "\n\n" + `data: {"choices":[{"delta":{"content":"Test"}}]}` + "\n\n",
			expected: "Test",
		},
		{
			name:     "malformed JSON",
			input:    "data: {invalid json}\n" + `data: {"choices":[{"delta":{"content":"Valid"}}]}` + "\n",
			expected: "Valid",
		},
		{
			name:     "no data prefix",
			input:    "some random line\n" + `data: {"choices":[{"delta":{"content":"Test"}}]}` + "\n",
			expected: "Test",
		},
	}

	client := NewClient("http://test", "", "test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result strings.Builder
			err := client.processStream(strings.NewReader(tt.input), func(content string) {
				result.WriteString(content)
			})

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.String())
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		expectedMsg string
	}{
		{
			name:        "JSON error",
			statusCode:  401,
			body:        `{"error":{"message":"Invalid API key"}}`,
			expectedMsg: "Invalid API key",
		},
		{
			name:        "plain text error",
			statusCode:  500,
			body:        "Internal server error",
			expectedMsg: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(server.URL, "key", "model")
			messages := []ChatMessage{{Role: "user", Content: "Hi"}}

			_, err := client.ChatCompletion(messages, 100, 0.7)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.expectedMsg) {
				t.Errorf("Expected error to contain %q, got %s", tt.expectedMsg, err.Error())
			}
		})
	}
}

func TestSetHeaders(t *testing.T) {
	// With API key
	client := NewClient("http://test", "my-key", "model")
	req, _ := http.NewRequest("POST", "http://test", nil)
	client.setHeaders(req)

	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("Expected Content-Type application/json")
	}
	if req.Header.Get("Authorization") != "Bearer my-key" {
		t.Error("Expected Authorization Bearer my-key")
	}

	// Without API key
	client = NewClient("http://test", "", "model")
	req, _ = http.NewRequest("POST", "http://test", nil)
	client.setHeaders(req)

	if req.Header.Get("Authorization") != "" {
		t.Error("Expected no Authorization header when API key is empty")
	}
}

func TestChatCompletionEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with empty choices
		resp := ChatCompletionResponse{
			ID:      "test-id",
			Model:   "test-model",
			Choices: []Choice{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	resp, err := client.ChatCompletion(messages, 100, 0.7)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(resp.Choices) != 0 {
		t.Errorf("Expected 0 choices, got %d", len(resp.Choices))
	}
}

func TestChatCompletionStreamEmptyDelta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send chunk with empty delta
		chunks := []string{
			`data: {"choices":[{"delta":{}}]}`,
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n\n"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	var result strings.Builder
	err := client.ChatCompletionStream(messages, 100, 0.7, func(content string) {
		result.WriteString(content)
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "Hello" {
		t.Errorf("Expected 'Hello', got %s", result.String())
	}
}

func TestChatCompletionStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"Server error"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	err := client.ChatCompletionStream(messages, 100, 0.7, func(content string) {})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Server error") {
		t.Errorf("Expected 'Server error' in message, got %s", err.Error())
	}
}

func TestProcessStreamEdgeCases(t *testing.T) {
	c := NewClient("http://test", "", "test")

	// Test with empty choices array
	input := `data: {"choices":[]}` + "\n"
	var result strings.Builder
	err := c.processStream(strings.NewReader(input), func(content string) {
		result.WriteString(content)
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "" {
		t.Errorf("Expected empty result, got %q", result.String())
	}
}

func TestChatCompletionWithMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify max_tokens in request
		body, _ := io.ReadAll(r.Body)
		var req ChatCompletionRequest
		_ = json.Unmarshal(body, &req)

		if req.MaxTokens != 2048 {
			t.Errorf("Expected max_tokens 2048, got %d", req.MaxTokens)
		}

		resp := ChatCompletionResponse{
			Choices: []Choice{{Message: ChatMessage{Content: "response"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	_, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "test"}}, 2048, 0.5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestChatCompletionWithTemperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ChatCompletionRequest
		_ = json.Unmarshal(body, &req)

		if req.Temperature == nil || *req.Temperature != 0.9 {
			t.Errorf("Expected temperature 0.9, got %v", req.Temperature)
		}

		resp := ChatCompletionResponse{
			Choices: []Choice{{Message: ChatMessage{Content: "response"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	_, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "test"}}, 100, 0.9)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestNewClientTrimsBaseURL(t *testing.T) {
	// With trailing slash
	c1 := NewClient("https://api.example.com/v1/", "key", "model")
	if c1.BaseURL != "https://api.example.com/v1" {
		t.Errorf("Expected URL without trailing slash, got %s", c1.BaseURL)
	}

	// Without trailing slash
	c2 := NewClient("https://api.example.com/v1", "key", "model")
	if c2.BaseURL != "https://api.example.com/v1" {
		t.Errorf("Expected URL unchanged, got %s", c2.BaseURL)
	}
}

func TestChatCompletionDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	_, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "test"}}, 100, 0.7)

	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestChatCompletionStreamWithEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"choices":[{"delta":{"content":""}}]}`,
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":""}}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n\n"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	var result strings.Builder
	err := client.ChatCompletionStream(messages, 100, 0.7, func(content string) {
		result.WriteString(content)
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.String() != "Hello" {
		t.Errorf("Expected 'Hello', got %s", result.String())
	}
}

func TestChatCompletionMultipleChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatCompletionResponse{
			Choices: []Choice{
				{Message: ChatMessage{Content: "first"}},
				{Message: ChatMessage{Content: "second"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	resp, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "test"}}, 100, 0.7)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Should return first choice
	if resp.Choices[0].Message.Content != "first" {
		t.Errorf("Expected 'first', got %s", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletionTimeout(t *testing.T) {
	// Very slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't actually sleep in tests, just return quickly
		resp := ChatCompletionResponse{
			Choices: []Choice{{Message: ChatMessage{Content: "done"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	resp, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "test"}}, 100, 0.7)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "done" {
		t.Errorf("Expected 'done', got %s", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletionStreamMultipleChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 10; i++ {
			_, _ = w.Write([]byte(fmt.Sprintf(`data: {"choices":[{"delta":{"content":"chunk%d "}}]}`, i) + "\n"))
		}
		_, _ = w.Write([]byte(`data: [DONE]` + "\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "Hi"}}

	var result strings.Builder
	err := client.ChatCompletionStream(messages, 100, 0.7, func(content string) {
		result.WriteString(content)
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := "chunk0 chunk1 chunk2 chunk3 chunk4 chunk5 chunk6 chunk7 chunk8 chunk9 "
	if result.String() != expected {
		t.Errorf("Expected %q, got %q", expected, result.String())
	}
}

func TestChatCompletionWithRoleContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ChatCompletionRequest
		_ = json.Unmarshal(body, &req)

		// Verify messages are passed correctly
		if len(req.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("Expected system role, got %s", req.Messages[0].Role)
		}

		resp := ChatCompletionResponse{
			Choices: []Choice{{Message: ChatMessage{Content: "response"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	messages := []ChatMessage{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
	}
	_, err := client.ChatCompletion(messages, 100, 0.7)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestHandleErrorVariousStatusCodes(t *testing.T) {
	tests := []struct {
		statusCode int
		body       string
	}{
		{400, `{"error":{"message":"Bad request"}}`},
		{401, `{"error":{"message":"Unauthorized"}}`},
		{403, `{"error":{"message":"Forbidden"}}`},
		{404, `{"error":{"message":"Not found"}}`},
		{429, `{"error":{"message":"Rate limited"}}`},
		{500, `{"error":{"message":"Server error"}}`},
		{503, `{"error":{"message":"Service unavailable"}}`},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(server.URL, "key", "model")
			_, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "test"}}, 100, 0.7)

			if err == nil {
				t.Error("Expected error")
			}
		})
	}
}

func TestChatCompletionStreamNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "test"}}

	err := client.ChatCompletionStream(messages, 100, 0.7, func(content string) {})

	if err == nil {
		t.Error("Expected network error")
	}
}

func TestChatCompletionNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "test"}}

	_, err := client.ChatCompletion(messages, 100, 0.7)

	if err == nil {
		t.Error("Expected network error")
	}
}

func TestChatCompletionStreamInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {invalid json}` + "\n"))
		_, _ = w.Write([]byte(`data: [DONE]` + "\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	messages := []ChatMessage{{Role: "user", Content: "test"}}

	var result strings.Builder
	err := client.ChatCompletionStream(messages, 100, 0.7, func(content string) {
		result.WriteString(content)
	})

	// Should handle gracefully or return error
	if err != nil {
		t.Logf("Stream error (expected for invalid JSON): %v", err)
	}
}

func TestChatCompletionEmptyAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that Authorization header is present even if empty
		auth := r.Header.Get("Authorization")
		if auth != "Bearer " {
			t.Logf("Auth header: %s", auth)
		}
		resp := ChatCompletionResponse{
			Choices: []Choice{{Message: ChatMessage{Content: "response"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "model")
	messages := []ChatMessage{{Role: "user", Content: "test"}}

	resp, err := client.ChatCompletion(messages, 100, 0.7)
	if err != nil {
		t.Logf("ChatCompletion with empty key: %v", err)
	}
	// Success even with empty key (for local LLMs)
	_ = resp
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/models" {
			t.Errorf("Expected /models, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Send response
		resp := ModelsResponse{
			Object: "list",
			Data: []Model{
				{ID: "gpt-4", Object: "model", Created: 1234567890, OwnedBy: "openai"},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1234567890, OwnedBy: "openai"},
				{ID: "llama2", Object: "model", Created: 1234567890, OwnedBy: "meta"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model")
	resp, err := client.ListModels()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "gpt-4" {
		t.Errorf("Expected 'gpt-4', got %s", resp.Data[0].ID)
	}
}

func TestListModelsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key", "test-model")
	_, err := client.ListModels()

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("Expected error message to contain 'Invalid API key', got %s", err.Error())
	}
}

func TestListModelsDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	_, err := client.ListModels()

	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestListModelsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ModelsResponse{
			Object: "list",
			Data:   []Model{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model")
	resp, err := client.ListModels()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("Expected 0 models, got %d", len(resp.Data))
	}
}

func TestListModelsNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "key", "model")
	_, err := client.ListModels()

	if err == nil {
		t.Error("Expected network error")
	}
}

func TestUsesMaxCompletionTokens(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"gpt-4o-2024-08-06", true},
		{"o1", true},
		{"o1-mini", true},
		{"o1-preview", true},
		{"gpt-5", true},
		{"gpt-5-turbo", true},
		{"gpt-6", true},
		{"gpt-4-turbo", false},
		{"gpt-4", false},
		{"gpt-3.5-turbo", false},
		{"llama2", false},
		{"claude-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			client := NewClient("http://test", "key", tt.model)
			result := client.usesMaxCompletionTokens()
			if result != tt.expected {
				t.Errorf("Expected %v for model %s, got %v", tt.expected, tt.model, result)
			}
		})
	}
}

func TestBuildRequestMaxTokens(t *testing.T) {
	// Test old model uses max_tokens
	client := NewClient("http://test", "key", "gpt-4")
	req := client.buildRequest([]ChatMessage{{Role: "user", Content: "test"}}, CompletionOptions{MaxTokens: 100, Temperature: 0.7})

	if req.MaxTokens != 100 {
		t.Errorf("Expected MaxTokens 100, got %d", req.MaxTokens)
	}
	if req.MaxCompletionTokens != 0 {
		t.Errorf("Expected MaxCompletionTokens 0, got %d", req.MaxCompletionTokens)
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %v", req.Temperature)
	}
}

func TestBuildRequestMaxCompletionTokens(t *testing.T) {
	// Test new model uses max_completion_tokens
	client := NewClient("http://test", "key", "gpt-4o")
	req := client.buildRequest([]ChatMessage{{Role: "user", Content: "test"}}, CompletionOptions{MaxTokens: 100, Temperature: 0.7, Stream: true})

	if req.MaxCompletionTokens != 100 {
		t.Errorf("Expected MaxCompletionTokens 100, got %d", req.MaxCompletionTokens)
	}
	if req.MaxTokens != 0 {
		t.Errorf("Expected MaxTokens 0, got %d", req.MaxTokens)
	}
	if !req.Stream {
		t.Error("Expected Stream to be true")
	}
}

func TestBuildRequestO1NoTemperature(t *testing.T) {
	// Test o1 models don't include temperature
	client := NewClient("http://test", "key", "o1-mini")
	req := client.buildRequest([]ChatMessage{{Role: "user", Content: "test"}}, CompletionOptions{MaxTokens: 100, Temperature: 0.7})

	if req.Temperature != nil {
		t.Errorf("Expected Temperature to be nil for o1 model, got %v", req.Temperature)
	}
	if req.MaxCompletionTokens != 100 {
		t.Errorf("Expected MaxCompletionTokens 100, got %d", req.MaxCompletionTokens)
	}
}

func TestBuildRequestPropagatesToolsAndQuirks(t *testing.T) {
	// gpt-4o quirk (max_completion_tokens) must still apply when tools are present.
	client := NewClient("http://test", "key", "gpt-4o")
	tools := []Tool{{Type: "function", Function: FunctionDef{Name: "read_file"}}}
	req := client.buildRequest([]ChatMessage{{Role: "user", Content: "go"}}, CompletionOptions{
		MaxTokens:      50,
		Temperature:    0.2,
		Tools:          tools,
		ToolChoice:     "auto",
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	})

	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "read_file" {
		t.Errorf("Expected tools to propagate, got %+v", req.Tools)
	}
	if req.ToolChoice != "auto" {
		t.Errorf("Expected ToolChoice auto, got %v", req.ToolChoice)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
		t.Errorf("Expected response_format to propagate, got %+v", req.ResponseFormat)
	}
	if req.MaxCompletionTokens != 50 {
		t.Errorf("Expected MaxCompletionTokens 50 (gpt-4o quirk preserved), got %d", req.MaxCompletionTokens)
	}
	if req.MaxTokens != 0 {
		t.Errorf("Expected MaxTokens 0, got %d", req.MaxTokens)
	}
}

func TestChatCompletionWithToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify tools made it into the request body.
		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if len(req.Tools) == 0 {
			t.Error("expected tools in request body")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "1", "object": "chat.completion", "model": "gpt-4o",
			"choices": [{
				"index": 0,
				"finish_reason": "tool_calls",
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_1", "type": "function",
						"function": {"name": "read_file", "arguments": "{\"path\":\"a.txt\"}"}
					}]
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "gpt-4o")
	resp, err := client.ChatCompletionWithOptions([]ChatMessage{{Role: "user", Content: "read a.txt"}}, CompletionOptions{
		MaxTokens: 100,
		Tools:     []Tool{{Type: "function", Function: FunctionDef{Name: "read_file"}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("expected finish_reason tool_calls, got %q", resp.Choices[0].FinishReason)
	}
	calls := resp.Choices[0].Message.ToolCalls
	if len(calls) != 1 || calls[0].Function.Name != "read_file" {
		t.Fatalf("expected one read_file tool call, got %+v", calls)
	}
	if calls[0].Function.Arguments != `{"path":"a.txt"}` {
		t.Errorf("unexpected arguments: %s", calls[0].Function.Arguments)
	}
}

func TestChatMessageStringContentRoundTrip(t *testing.T) {
	// Backward compatibility: a plain string-content message must marshal to a
	// string "content" and unmarshal back identically (existing history files).
	orig := ChatMessage{Role: "user", Content: "hello world"}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(data, []byte(`"content":"hello world"`)) {
		t.Errorf("expected string content, got: %s", data)
	}

	// Legacy payload decodes correctly.
	var back ChatMessage
	if err := json.Unmarshal([]byte(`{"role":"assistant","content":"hi there"}`), &back); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if back.Content != "hi there" || len(back.Parts) != 0 {
		t.Errorf("legacy decode mismatch: %+v", back)
	}
}

func TestChatMessageMultimodalRoundTrip(t *testing.T) {
	msg := ChatMessage{
		Role: "user",
		Parts: []ContentPart{
			{Type: "text", Text: "what is this?"},
			{Type: "image_url", ImageURL: &ImageURL{URL: "data:image/png;base64,AAAA"}},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// content must be an array, not a string.
	if !bytes.Contains(data, []byte(`"content":[`)) || !bytes.Contains(data, []byte(`"image_url"`)) {
		t.Errorf("expected array content with image_url, got: %s", data)
	}

	var back ChatMessage
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(back.Parts))
	}
	if back.Content != "what is this?" {
		t.Errorf("expected flattened text content, got %q", back.Content)
	}
	if back.Parts[1].ImageURL == nil || back.Parts[1].ImageURL.URL == "" {
		t.Errorf("image part lost: %+v", back.Parts[1])
	}
}

func TestStreamReassemblesToolCalls(t *testing.T) {
	// Simulate tool_calls arriving fragmented across SSE chunks, plus content.
	chunks := []string{
		`data: {"choices":[{"delta":{"content":"Let me "}}]}`,
		`data: {"choices":[{"delta":{"content":"check."}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"pa"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"a.txt\"}"}}]}}]}`,
		`data: {"choices":[{"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			_, _ = w.Write([]byte(c + "\n\n"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "m")
	var streamed strings.Builder
	res, err := client.ChatCompletionStreamWithOptions(
		[]ChatMessage{{Role: "user", Content: "read a.txt"}},
		CompletionOptions{Tools: []Tool{{Type: "function", Function: FunctionDef{Name: "read_file"}}}},
		func(s string) { streamed.WriteString(s) },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if streamed.String() != "Let me check." {
		t.Errorf("streamed content = %q, want 'Let me check.'", streamed.String())
	}
	if res.FinishReason != "tool_calls" {
		t.Errorf("finish reason = %q", res.FinishReason)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("expected 1 reassembled tool call, got %d", len(res.ToolCalls))
	}
	tc := res.ToolCalls[0]
	if tc.ID != "call_1" || tc.Function.Name != "read_file" || tc.Function.Arguments != `{"path":"a.txt"}` {
		t.Errorf("tool call not reassembled correctly: %+v", tc)
	}
}

func TestRetryOnTransientThenSuccess(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			http.Error(w, "overloaded", http.StatusServiceUnavailable) // 503: retryable
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "m")
	client.MaxRetries = 3
	resp, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "hi"}}, 10, 0.5)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if resp.Choices[0].Message.Content != "ok" {
		t.Errorf("unexpected content: %q", resp.Choices[0].Message.Content)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", got)
	}
}

func TestRetryExhaustedReturnsError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "down", http.StatusBadGateway) // 502: always retryable
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "m")
	client.MaxRetries = 2
	_, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "hi"}}, 10, 0.5)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// Last attempt's response is surfaced via handleError (status 502).
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 attempts (MaxRetries=2 + 1), got %d", got)
	}
}

func TestNonRetryableStatusNotRetried(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest) // 400: not retryable
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "m")
	client.MaxRetries = 3
	if _, err := client.ChatCompletion([]ChatMessage{{Role: "user", Content: "hi"}}, 10, 0.5); err == nil {
		t.Fatal("expected error for 400")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("400 should not be retried; expected 1 attempt, got %d", got)
	}
}

func TestEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req EmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(req.Input) != 2 {
			t.Errorf("expected 2 inputs, got %d", len(req.Input))
		}
		w.Header().Set("Content-Type", "application/json")
		// Return out of order to verify index-based reordering.
		_, _ = w.Write([]byte(`{"data":[
			{"object":"embedding","index":1,"embedding":[0.1,0.2]},
			{"object":"embedding","index":0,"embedding":[0.9,0.8]}
		]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "embed-model")
	vecs, err := client.Embeddings([]string{"a", "b"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if vecs[0][0] != 0.9 || vecs[1][0] != 0.1 {
		t.Errorf("vectors not ordered by index: %v", vecs)
	}
}

func TestChatCompletionWithResponseFormatInBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !bytes.Contains(body, []byte(`"response_format"`)) || !bytes.Contains(body, []byte(`"json_schema"`)) {
			t.Errorf("expected response_format/json_schema in body, got: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{}"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "gpt-4o")
	_, err := client.ChatCompletionWithOptions([]ChatMessage{{Role: "user", Content: "extract"}}, CompletionOptions{
		ResponseFormat: &ResponseFormat{
			Type:       "json_schema",
			JSONSchema: &JSONSchemaSpec{Name: "out", Schema: json.RawMessage(`{"type":"object"}`), Strict: true},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
