package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouterBrain implements Brain using the OpenRouter API, which exposes an
// OpenAI-compatible interface that routes requests to many upstream LLMs.
type OpenRouterBrain struct {
	APIKey    string
	Model     string
	MaxTokens int // 0 = let upstream use its default
	Client    *http.Client
}

// NewOpenRouterBrain creates a new OpenRouterBrain with a sensible HTTP timeout.
func NewOpenRouterBrain(apiKey, model string) *OpenRouterBrain {
	return &OpenRouterBrain{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (b *OpenRouterBrain) GetProviderName() string { return "openrouter" }

// Embed is not supported by OpenRouter. Memory features are disabled when using
// this provider.
func (b *OpenRouterBrain) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, fmt.Errorf("openrouter provider does not support embeddings; memory features are disabled")
}

func (b *OpenRouterBrain) addAuthHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.APIKey)
}

func (b *OpenRouterBrain) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, ConsumptionReport, error) {
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
	if b.MaxTokens > 0 {
		payload["max_tokens"] = b.MaxTokens
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

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("openrouter chat marshal: %w", err)
	}

	newReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterBaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		b.addAuthHeaders(req)
		return req, nil
	}

	resp, err := httpDoWithRetry(ctx, b.Client, newReq, 3)
	if err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("openrouter chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, ConsumptionReport{}, fmt.Errorf("openrouter API error %d: %s", resp.StatusCode, string(body))
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("openrouter chat decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", nil, ConsumptionReport{}, fmt.Errorf("openrouter returned empty choices (check API key or model quota)")
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

// ChatJSON enforces a JSON schema response via OpenRouter's response_format,
// which is forwarded to upstream providers that support structured outputs.
func (b *OpenRouterBrain) ChatJSON(ctx context.Context, messages []Message, schema map[string]interface{}) (string, ConsumptionReport, error) {
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
	if b.MaxTokens > 0 {
		payload["max_tokens"] = b.MaxTokens
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("openrouter json chat marshal: %w", err)
	}

	newReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterBaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		b.addAuthHeaders(req)
		return req, nil
	}

	resp, err := httpDoWithRetry(ctx, b.Client, newReq, 3)
	if err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("openrouter json chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", ConsumptionReport{}, fmt.Errorf("openrouter API error %d: %s", resp.StatusCode, string(body))
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("openrouter json chat decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", ConsumptionReport{}, fmt.Errorf("openrouter returned empty choices")
	}

	report := ConsumptionReport{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
	}

	return result.Choices[0].Message.Content, report, nil
}
