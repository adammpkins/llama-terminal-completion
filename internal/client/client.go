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
	// MaxRetries is the number of additional attempts on transient failures
	// (network errors and 429/5xx). Zero means use defaultMaxRetries.
	MaxRetries int
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

const defaultMaxRetries = 2

// retrySleep is the backoff sleep, overridable in tests.
var retrySleep = time.Sleep

// DisableRetryBackoffForTest removes the inter-retry backoff sleep. It is
// intended for use in other packages' tests (e.g. cli) whose mock servers
// return transient statuses; without it those tests would incur real delays.
func DisableRetryBackoffForTest() { retrySleep = func(time.Duration) {} }

func (c *Client) maxRetries() int {
	if c.MaxRetries > 0 {
		return c.MaxRetries
	}
	return defaultMaxRetries
}

// isRetryableStatus reports whether an HTTP status warrants a retry.
func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// doRequest builds and sends an HTTP request with exponential-backoff retry on
// transient failures (network errors and retryable statuses). The body bytes
// are re-sent on each attempt. On success it returns the response; the caller
// is responsible for closing the body and checking the status.
func (c *Client) doRequest(method, urlPath string, body []byte) (*http.Response, error) {
	attempts := c.maxRetries() + 1
	var lastErr error
	for attempt := range attempts {
		if attempt > 0 {
			retrySleep(time.Duration(1<<uint(attempt-1)) * 500 * time.Millisecond)
		}

		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequest(method, c.BaseURL+urlPath, reader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if isRetryableStatus(resp.StatusCode) && attempt < attempts-1 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("transient API error %d", resp.StatusCode)
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("request failed after %d attempts: %w", attempts, lastErr)
}

// CompletionOptions carries the tunable parameters for a chat completion
// request, including the tool-calling and structured-output fields.
type CompletionOptions struct {
	MaxTokens      int
	Temperature    float64
	Tools          []Tool
	ToolChoice     any
	ResponseFormat *ResponseFormat
	Stream         bool
}

// ChatCompletion sends a chat completion request and returns the full response
func (c *Client) ChatCompletion(messages []ChatMessage, maxTokens int, temperature float64) (*ChatCompletionResponse, error) {
	return c.ChatCompletionWithOptions(messages, CompletionOptions{
		MaxTokens:   maxTokens,
		Temperature: temperature,
	})
}

// ChatCompletionWithOptions sends a non-streaming chat completion request with
// the full set of options (tools, tool_choice, response_format). The
// model-quirk handling in buildRequest is applied uniformly.
func (c *Client) ChatCompletionWithOptions(messages []ChatMessage, opts CompletionOptions) (*ChatCompletionResponse, error) {
	opts.Stream = false
	req := c.buildRequest(messages, opts)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest("POST", "/chat/completions", body)
	if err != nil {
		return nil, err
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
func (c *Client) buildRequest(messages []ChatMessage, opts CompletionOptions) ChatCompletionRequest {
	req := ChatCompletionRequest{
		Model:          c.Model,
		Messages:       messages,
		Stream:         opts.Stream,
		Tools:          opts.Tools,
		ToolChoice:     opts.ToolChoice,
		ResponseFormat: opts.ResponseFormat,
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
		req.MaxCompletionTokens = opts.MaxTokens
	} else {
		req.MaxTokens = opts.MaxTokens
	}

	// Only set temperature for models that support it
	// o1 series and gpt-5+ don't support custom temperature
	supportsTemperature := !isO1 && !isGPT5Plus
	if supportsTemperature {
		temperature := opts.Temperature
		req.Temperature = &temperature
	}

	return req
}

// ChatCompletionStream sends a streaming chat completion request
// The callback is called for each content chunk received
func (c *Client) ChatCompletionStream(messages []ChatMessage, maxTokens int, temperature float64, callback func(content string)) error {
	req := c.buildRequest(messages, CompletionOptions{
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
	})

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest("POST", "/chat/completions", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return c.handleError(resp)
	}

	return c.processStream(resp.Body, callback)
}

// ChatCompletionStreamWithOptions streams a completion that may include tool
// calls. Content deltas are passed to onContent as they arrive (for live
// display); fragmented tool_calls deltas are reassembled. It returns the fully
// assembled result. onContent may be nil.
func (c *Client) ChatCompletionStreamWithOptions(messages []ChatMessage, opts CompletionOptions, onContent func(string)) (*StreamResult, error) {
	opts.Stream = true
	req := c.buildRequest(messages, opts)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest("POST", "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return c.processStreamFull(resp.Body, onContent)
}

// toolCallAccumulator collects fragmented tool-call deltas by index.
type toolCallAccumulator struct {
	id   string
	typ  string
	name string
	args strings.Builder
}

// processStreamFull parses an SSE stream, streaming content to onContent and
// reassembling tool calls, returning the final StreamResult.
func (c *Client) processStreamFull(body io.Reader, onContent func(string)) (*StreamResult, error) {
	reader := bufio.NewReader(body)
	var content strings.Builder
	finishReason := ""
	accs := map[int]*toolCallAccumulator{}
	var order []int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error reading stream: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamingChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			content.WriteString(choice.Delta.Content)
			if onContent != nil {
				onContent(choice.Delta.Content)
			}
		}
		for _, tc := range choice.Delta.ToolCalls {
			acc, ok := accs[tc.Index]
			if !ok {
				acc = &toolCallAccumulator{}
				accs[tc.Index] = acc
				order = append(order, tc.Index)
			}
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Type != "" {
				acc.typ = tc.Type
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				acc.args.WriteString(tc.Function.Arguments)
			}
		}
		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}
	}

	result := &StreamResult{Content: content.String(), FinishReason: finishReason}
	for _, idx := range order {
		acc := accs[idx]
		typ := acc.typ
		if typ == "" {
			typ = "function"
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:       acc.id,
			Type:     typ,
			Function: FunctionCall{Name: acc.name, Arguments: acc.args.String()},
		})
	}
	return result, nil
}

// ListModels fetches available models from the /models endpoint
func (c *Client) ListModels() (*ModelsResponse, error) {
	resp, err := c.doRequest("GET", "/models", nil)
	if err != nil {
		return nil, err
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

// Embeddings requests embedding vectors for the given inputs. If model is
// empty, the client's Model is used. The returned slice is index-aligned to
// input.
func (c *Client) Embeddings(input []string, model string) ([][]float32, error) {
	if model == "" {
		model = c.Model
	}
	reqBody := EmbeddingRequest{Model: model, Input: input}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest("POST", "/embeddings", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Order by Index to be safe, then return the raw vectors.
	out := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	return out, nil
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
