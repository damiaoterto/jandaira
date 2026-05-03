package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type OpenAIBrain struct {
	APIKey      string
	Model       string
	MaxTokensFn func() int // nil = let the API use its default
	Client      *http.Client
}

// isTransientNetworkError returns true for connection-level errors that are
// safe to retry (GOAWAY, reset, EOF). HTTP 4xx/5xx arrive as responses with
// err==nil and must NOT be retried here.
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "GOAWAY") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "broken pipe")
}

// httpDoWithRetry executes the request returned by newReq, retrying up to
// maxRetries times on transient network errors with exponential backoff.
func httpDoWithRetry(ctx context.Context, client *http.Client, newReq func() (*http.Request, error), maxRetries int) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			wait := time.Duration(math.Pow(2, float64(i-1))) * 500 * time.Millisecond
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		req, err := newReq()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		if !isTransientNetworkError(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// NewOpenAIBrain creates a new OpenAIBrain. No client-level timeout is set —
// deadline is controlled by the caller's context (typically the 10-minute
// workflow context), which allows reasoning models (o1, o3) to respond.
func NewOpenAIBrain(apiKey string, model string) *OpenAIBrain {
	return &OpenAIBrain{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{},
	}
}

// Internal structures to ensure JSON for OpenAI is perfect
type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

func (b *OpenAIBrain) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, ConsumptionReport, error) {
	url := "https://api.openai.com/v1/chat/completions"

	// 1. Format Messages
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

	// 2. Format Tools
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

	jsonData, _ := json.Marshal(payload)

	newReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
		return req, nil
	}

	resp, err := httpDoWithRetry(ctx, b.Client, newReq, 3)
	if err != nil {
		return "", nil, ConsumptionReport{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, ConsumptionReport{}, fmt.Errorf("OpenAI API error: %s", string(body))
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
		return "", nil, ConsumptionReport{}, err
	}

	if len(result.Choices) == 0 {
		return "", nil, ConsumptionReport{}, fmt.Errorf("OpenAI returned an empty choices list (possible API key or quota issue)")
	}

	report := ConsumptionReport{
		PromptTokens: result.Usage.PromptTokens, TotalTokens: result.Usage.TotalTokens,
	}

	msg := result.Choices[0].Message

	// Convert OpenAI ToolCalls back to internal format
	var internalToolCalls []ToolCall
	for _, tc := range msg.ToolCalls {
		internalToolCalls = append(internalToolCalls, ToolCall{
			ID: tc.ID, Name: tc.Function.Name, ArgsJSON: tc.Function.Arguments,
		})
	}

	return msg.Content, internalToolCalls, report, nil
}

func (b *OpenAIBrain) Embed(ctx context.Context, text string) ([]float32, error) {
	const embedURL = "https://api.openai.com/v1/embeddings"
	const embedModel = "text-embedding-3-small"

	payload := map[string]interface{}{
		"model": embedModel,
		"input": text,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("embed marshal: %w", err)
	}

	newEmbedReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", embedURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("embed new request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
		return req, nil
	}

	resp, err := httpDoWithRetry(ctx, b.Client, newEmbedReq, 3)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embeddings API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("embed decode response: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai embeddings API returned empty embedding")
	}

	return result.Data[0].Embedding, nil
}

func (b *OpenAIBrain) GetProviderName() string { return "openai" }

// ChatJSON calls the OpenAI Structured Outputs API, enforcing the given JSON
// schema so the response is always valid and parseable without sanitization.
func (b *OpenAIBrain) ChatJSON(ctx context.Context, messages []Message, schema map[string]interface{}) (string, ConsumptionReport, error) {
	url := "https://api.openai.com/v1/chat/completions"

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

	jsonData, _ := json.Marshal(payload)

	newJSONReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
		return req, nil
	}

	resp, err := httpDoWithRetry(ctx, b.Client, newJSONReq, 3)
	if err != nil {
		return "", ConsumptionReport{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", ConsumptionReport{}, fmt.Errorf("OpenAI API error: %s", string(body))
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
		return "", ConsumptionReport{}, err
	}

	if len(result.Choices) == 0 {
		return "", ConsumptionReport{}, fmt.Errorf("OpenAI returned empty choices")
	}

	report := ConsumptionReport{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
	}

	return result.Choices[0].Message.Content, report, nil
}
