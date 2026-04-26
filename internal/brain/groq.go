package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const groqBaseURL = "https://api.groq.com/openai/v1/chat/completions"

// GroqBrain implements Brain using the Groq LPU inference API, which exposes
// an OpenAI-compatible interface.
type GroqBrain struct {
	APIKey    string
	Model     string
	MaxTokensFn func() int // nil = let the API use its default
	Client    *http.Client
}

// NewGroqBrain creates a new GroqBrain with a sensible HTTP timeout.
func NewGroqBrain(apiKey, model string) *GroqBrain {
	return &GroqBrain{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (b *GroqBrain) GetProviderName() string { return "groq" }

// Embed is not supported by Groq. Memory features are disabled when using
// this provider.
func (b *GroqBrain) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, fmt.Errorf("groq provider does not support embeddings; memory features are disabled")
}

func (b *GroqBrain) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, ConsumptionReport, error) {
	var formattedMessages []oaiMessage
	for _, msg := range messages {
		oaiMsg := oaiMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		for _, tc := range msg.ToolCalls {
			oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, oaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: tc.Name, Arguments: tc.ArgsJSON},
			})
		}
		formattedMessages = append(formattedMessages, oaiMsg)
	}

	payload := map[string]interface{}{
		"model":    b.Model,
		"messages": formattedMessages,
	}
	if b.MaxTokensFn != nil {
		if n := b.MaxTokensFn(); n > 0 {
			payload["max_completion_tokens"] = n
		}
	}
	if len(tools) > 0 {
		var oaiTools []map[string]interface{}
		for _, t := range tools {
			oaiTools = append(oaiTools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			})
		}
		payload["tools"] = oaiTools
		payload["tool_choice"] = "auto"
	}

	body, status, err := doPostWithFallback(ctx, b.Client, groqBaseURL, b.APIKey, payload)
	if err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("groq chat request: %w", err)
	}
	if status != http.StatusOK {
		return "", nil, ConsumptionReport{}, fmt.Errorf("groq API error %d: %s", status, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string        `json:"content"`
				ToolCalls []oaiToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("groq chat decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", nil, ConsumptionReport{}, fmt.Errorf("groq returned empty choices (check API key or model quota)")
	}

	report := ConsumptionReport{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
	}

	msg := result.Choices[0].Message
	var toolCalls []ToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:       tc.ID,
			Name:     tc.Function.Name,
			ArgsJSON: tc.Function.Arguments,
		})
	}

	return msg.Content, toolCalls, report, nil
}

// ChatJSON enforces JSON output via Groq's response_format parameter.
func (b *GroqBrain) ChatJSON(ctx context.Context, messages []Message, schema map[string]interface{}) (string, ConsumptionReport, error) {
	var formattedMessages []oaiMessage
	for _, msg := range messages {
		formattedMessages = append(formattedMessages, oaiMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	payload := map[string]interface{}{
		"model":    b.Model,
		"messages": formattedMessages,
		"response_format": map[string]interface{}{
			"type":        "json_schema",
			"json_schema": schema,
		},
	}
	if b.MaxTokensFn != nil {
		if n := b.MaxTokensFn(); n > 0 {
			payload["max_completion_tokens"] = n
		}
	}

	body, status, err := doPostWithFallback(ctx, b.Client, groqBaseURL, b.APIKey, payload)
	if err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("groq json chat request: %w", err)
	}
	if status != http.StatusOK {
		return "", ConsumptionReport{}, fmt.Errorf("groq API error %d: %s", status, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("groq json chat decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", ConsumptionReport{}, fmt.Errorf("groq returned empty choices")
	}

	report := ConsumptionReport{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
	}

	return result.Choices[0].Message.Content, report, nil
}
