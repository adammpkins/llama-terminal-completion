package client

import (
	"encoding/json"
	"strings"
)

// ChatMessage represents a message in a chat conversation.
//
// The tool-calling fields are additive and tagged omitempty so that
// conversations persisted before they existed continue to deserialize
// cleanly (see internal/cli/conversations.go).
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`

	// Parts holds multimodal content (text + images). When non-empty it is
	// serialized as the OpenAI "content" array instead of the Content string.
	// It is excluded from default (un)marshaling — see MarshalJSON/UnmarshalJSON.
	Parts []ContentPart `json:"-"`

	// ToolCalls is populated on assistant messages when the model decides
	// to invoke one or more tools.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID links a role:"tool" result message back to the assistant's
	// originating tool call.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// Name carries the tool name on role:"tool" result messages.
	Name string `json:"name,omitempty"`
}

// ContentPart is one element of a multimodal message: either text or an image.
type ContentPart struct {
	Type     string    `json:"type"` // "text" | "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL references an image, either a remote URL or a data: URI.
type ImageURL struct {
	URL string `json:"url"`
}

// MarshalJSON emits a string "content" for plain text messages (backward
// compatible with existing history) and an array "content" when Parts is set.
func (m ChatMessage) MarshalJSON() ([]byte, error) {
	type alias ChatMessage // avoids recursion; alias has no MarshalJSON
	if len(m.Parts) > 0 {
		return json.Marshal(struct {
			alias
			Content []ContentPart `json:"content"`
		}{alias(m), m.Parts})
	}
	return json.Marshal(alias(m))
}

// UnmarshalJSON accepts "content" as either a string or an array of parts. When
// an array is present, Parts is populated and Content holds the concatenated
// text (for display and history titles).
func (m *ChatMessage) UnmarshalJSON(data []byte) error {
	type alias ChatMessage
	aux := struct {
		*alias
		Content json.RawMessage `json:"content"`
	}{alias: (*alias)(m)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.Content) == 0 || string(aux.Content) == "null" {
		m.Content = ""
		return nil
	}
	// String form?
	var s string
	if err := json.Unmarshal(aux.Content, &s); err == nil {
		m.Content = s
		return nil
	}
	// Array form.
	var parts []ContentPart
	if err := json.Unmarshal(aux.Content, &parts); err != nil {
		return err
	}
	m.Parts = parts
	var text []string
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			text = append(text, p.Text)
		}
	}
	m.Content = strings.Join(text, "\n")
	return nil
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // always "function" today
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the name and raw JSON arguments of a tool call.
// Arguments is a JSON-encoded string as returned by the API, not a decoded object.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool describes a tool that the model may call.
type Tool struct {
	Type     string      `json:"type"` // always "function" today
	Function FunctionDef `json:"function"`
}

// FunctionDef is the schema for a callable function tool.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ResponseFormat requests a particular output format, e.g. a JSON schema.
type ResponseFormat struct {
	Type       string          `json:"type"` // "text" | "json_object" | "json_schema"
	JSONSchema *JSONSchemaSpec `json:"json_schema,omitempty"`
}

// JSONSchemaSpec describes a named JSON schema for structured output.
type JSONSchemaSpec struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict,omitempty"`
}

// ChatCompletionRequest represents an OpenAI-compatible chat completion request
type ChatCompletionRequest struct {
	Model               string          `json:"model"`
	Messages            []ChatMessage   `json:"messages"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	Stream              bool            `json:"stream"`
	Tools               []Tool          `json:"tools,omitempty"`
	ToolChoice          any             `json:"tool_choice,omitempty"`
	ResponseFormat      *ResponseFormat `json:"response_format,omitempty"`
}

// ChatCompletionResponse represents a non-streaming chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a single completion choice
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	Delta        ChatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a single chunk in a streaming response
type StreamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// StreamResult is the assembled outcome of a streaming completion: the full
// content, any reassembled tool calls, and the finish reason.
type StreamResult struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
}

// streamingChunk is the SSE chunk shape including fragmented tool-call deltas.
// tool_calls arrive across multiple chunks, keyed by index, with the function
// arguments string concatenated piece by piece.
type streamingChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// EmbeddingRequest is an OpenAI-compatible embeddings request.
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse is the response from the /embeddings endpoint.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

// EmbeddingData is a single embedding vector.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// Model represents a single model from the models endpoint
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse represents the response from the /models endpoint
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
