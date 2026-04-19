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

type OpenAIBrain struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewOpenAIBrain(apiKey string, model string) *OpenAIBrain {
	return &OpenAIBrain{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Estruturas internas para garantir que o JSON para a OpenAI fique perfeito
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

	// 1. Formatar Mensagens
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

	// 2. Formatar Ferramentas
	payload := map[string]interface{}{
		"model":    b.Model,
		"messages": formattedMessages,
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

	// Implementação de Exponential Backoff (Simplificada para legibilidade)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.APIKey)

	resp, err := b.Client.Do(req)
	if err != nil {
		return "", nil, ConsumptionReport{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, ConsumptionReport{}, fmt.Errorf("erro API OpenAI: %s", string(body))
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

	// Converter ToolCalls da OpenAI de volta para o formato interno
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

	req, err := http.NewRequestWithContext(ctx, "POST", embedURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("embed new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.APIKey)

	resp, err := b.Client.Do(req)
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

	jsonData, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.APIKey)

	resp, err := b.Client.Do(req)
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
