package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
)

// schemaPath is the value of ask's --schema flag.
var schemaPath string

// runAskStructured handles `lt ask --schema`: it requests JSON output matching
// the given schema, with a graceful fallback for providers that don't support
// native response_format. The raw JSON is printed (no markdown rendering).
func runAskStructured(apiClient *client.Client, question string) error {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if !json.Valid(raw) {
		return fmt.Errorf("schema file is not valid JSON")
	}

	mode := cfg.StructuredOutputMode
	if mode == "" {
		mode = "auto"
	}

	// Native attempt (unless forced to prompt mode).
	if mode == "native" || mode == "auto" {
		out, err := requestNativeSchema(apiClient, question, raw)
		if err == nil {
			fmt.Println(out)
			return nil
		}
		if mode == "native" {
			return err
		}
		// auto: fall through to prompt-based fallback.
	}

	out, err := requestPromptSchema(apiClient, question, raw)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

// requestNativeSchema uses the response_format json_schema feature.
func requestNativeSchema(apiClient *client.Client, question string, schema json.RawMessage) (string, error) {
	system := "You extract structured data and respond only with JSON matching the provided schema."
	if s := activeRoleSystem(); s != "" {
		system = s
	}
	resp, err := apiClient.ChatCompletionWithOptions(
		[]client.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: question},
		},
		client.CompletionOptions{
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
			ResponseFormat: &client.ResponseFormat{
				Type: "json_schema",
				JSONSchema: &client.JSONSchemaSpec{
					Name:   "output",
					Schema: schema,
					Strict: true,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response")
	}
	return resp.Choices[0].Message.Content, nil
}

// requestPromptSchema injects the schema into the prompt and extracts JSON from
// the reply — the fallback for providers without native structured output.
func requestPromptSchema(apiClient *client.Client, question string, schema json.RawMessage) (string, error) {
	system := fmt.Sprintf(`You extract structured data. Respond with ONLY a single JSON value that conforms to this JSON Schema. Do not include markdown fences, commentary, or any text outside the JSON.

JSON Schema:
%s`, string(schema))

	resp, err := apiClient.ChatCompletion(
		[]client.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: question},
		},
		cfg.MaxTokens, cfg.Temperature,
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response")
	}
	out := extractJSON(resp.Choices[0].Message.Content)
	if !json.Valid([]byte(out)) {
		return "", fmt.Errorf("model did not return valid JSON:\n%s", out)
	}
	return out, nil
}

// extractJSON strips markdown fences and surrounding prose, returning the most
// likely JSON payload from a model reply.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip ```json ... ``` or ``` ... ``` fences.
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i != -1 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}
	// Trim to the outermost JSON object/array if there's leading/trailing prose.
	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start != -1 && end != -1 && end >= start {
		return s[start : end+1]
	}
	return s
}
