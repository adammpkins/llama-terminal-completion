package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is an OpenAI-compatible API client
type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewClient creates a new API client
func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// ChatCompletion sends a chat completion request and returns the full response
func (c *Client) ChatCompletion(messages []ChatMessage, maxTokens int, temperature float64) (*ChatCompletionResponse, error) {
	req := c.buildRequest(messages, maxTokens, temperature, false)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// usesMaxCompletionTokens returns true for models that require max_completion_tokens instead of max_tokens
func (c *Client) usesMaxCompletionTokens() bool {
	model := strings.ToLower(c.Model)
	// Models that require max_completion_tokens:
	// - o1 series (o1, o1-mini, o1-preview)
	// - gpt-4o series (but not gpt-4-turbo which uses max_tokens)
	// - gpt-5 and future models
	if strings.HasPrefix(model, "o1") {
		return true
	}
	// gpt-4o and variants use max_completion_tokens
	if strings.Contains(model, "gpt-4o") {
		return true
	}
	// gpt-5 and future models (gpt-5, gpt-6, etc.)
	if strings.HasPrefix(model, "gpt-5") || strings.HasPrefix(model, "gpt-6") ||
		strings.HasPrefix(model, "gpt-7") || strings.HasPrefix(model, "gpt-8") ||
		strings.HasPrefix(model, "gpt-9") {
		return true
	}
	return false
}

// buildRequest creates a ChatCompletionRequest with the correct token parameter for the model
func (c *Client) buildRequest(messages []ChatMessage, maxTokens int, temperature float64, stream bool) ChatCompletionRequest {
	req := ChatCompletionRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   stream,
	}

	// Determine how to set token limits based on model
	model := strings.ToLower(c.Model)
	isO1 := strings.HasPrefix(model, "o1")
	isGPT4O := strings.Contains(model, "gpt-4o")
	isGPT5Plus := strings.HasPrefix(model, "gpt-5") ||
		strings.HasPrefix(model, "gpt-6") ||
		strings.HasPrefix(model, "gpt-7") ||
		strings.HasPrefix(model, "gpt-8") ||
		strings.HasPrefix(model, "gpt-9")

	// gpt-5+ models: skip token limits entirely (let API use default)
	// o1 and gpt-4o: use max_completion_tokens
	// older models: use max_tokens
	if isGPT5Plus {
		// Don't set any token limit for gpt-5+
	} else if isO1 || isGPT4O {
		req.MaxCompletionTokens = maxTokens
	} else {
		req.MaxTokens = maxTokens
	}

	// Only set temperature for models that support it
	// o1 series and gpt-5+ don't support custom temperature
	supportsTemperature := !isO1 && !isGPT5Plus
	if supportsTemperature {
		req.Temperature = &temperature
	}

	return req
}

// ChatCompletionStream sends a streaming chat completion request
// The callback is called for each content chunk received
func (c *Client) ChatCompletionStream(messages []ChatMessage, maxTokens int, temperature float64, callback func(content string)) error {
	req := c.buildRequest(messages, maxTokens, temperature, true)

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.handleError(resp)
	}

	return c.processStream(resp.Body, callback)
}

// ListModels fetches available models from the /models endpoint
func (c *Client) ListModels() (*ModelsResponse, error) {
	httpReq, err := http.NewRequest("GET", c.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// setHeaders sets the required HTTP headers for API requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}

// handleError reads an error response and returns a formatted error
func (c *Client) handleError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
}

// processStream reads and processes Server-Sent Events (SSE) stream
func (c *Client) processStream(body io.Reader, callback func(content string)) error {
	reader := bufio.NewReader(body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading stream: %w", err)
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for SSE data prefix
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			return nil
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks
			continue
		}

		// Extract and send content
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				callback(content)
			}
		}
	}
}
