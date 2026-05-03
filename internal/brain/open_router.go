package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"
const openRouterEmbedURL = "https://openrouter.ai/api/v1/embeddings"

var openRouterAttributionHeaders = map[string]string{
	"HTTP-Referer":            "https://github.com/damiaoterto/jandaira",
	"X-OpenRouter-Title":     "Jandaira",
	"X-OpenRouter-Categories": "cli-agent,cloud-agent",
}

// DefaultOpenRouterEmbeddingModel is used when EmbeddingModel is not set.
// Any model from https://openrouter.ai/models?output_modalities=embeddings can be used.
const DefaultOpenRouterEmbeddingModel = "openai/text-embedding-3-small"

// OpenRouterBrain implements Brain using the OpenRouter API, which exposes an
// OpenAI-compatible interface that routes requests to many upstream LLMs.
type OpenRouterBrain struct {
	APIKey         string
	Model          string
	EmbeddingModel string     // embedding model slug; defaults to DefaultOpenRouterEmbeddingModel
	MaxTokensFn    func() int // nil = let upstream use its default
	Client         *http.Client
}

// NewOpenRouterBrain creates a new OpenRouterBrain.
// No client-level timeout is set — deadline is controlled by the caller's context
// (typically the 10-minute workflow context), which allows slow models to respond.
func NewOpenRouterBrain(apiKey, model string) *OpenRouterBrain {
	return &OpenRouterBrain{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{},
	}
}

func (b *OpenRouterBrain) GetProviderName() string { return "openrouter" }

// embeddingModel returns the configured embedding model or the default.
func (b *OpenRouterBrain) embeddingModel() string {
	if b.EmbeddingModel != "" {
		return b.EmbeddingModel
	}
	return DefaultOpenRouterEmbeddingModel
}

// Embed generates a vector embedding via the OpenRouter embeddings API.
// The model used is EmbeddingModel (defaults to DefaultOpenRouterEmbeddingModel).
// Any model listed at https://openrouter.ai/models?output_modalities=embeddings is accepted.
func (b *OpenRouterBrain) Embed(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]interface{}{
		"model": b.embeddingModel(),
		"input": text,
	}
	body, status, err := doPost(ctx, b.Client, openRouterEmbedURL, b.APIKey, payload, openRouterAttributionHeaders)
	if err != nil {
		return nil, fmt.Errorf("openrouter embed request: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("openrouter embeddings API error %d: %s", status, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("openrouter embed decode: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openrouter embeddings API returned empty embedding")
	}
	return result.Data[0].Embedding, nil
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
	if b.MaxTokensFn != nil {
		if n := b.MaxTokensFn(); n > 0 {
			payload["max_tokens"] = n
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

	body, status, err := doPostWithFallback(ctx, b.Client, openRouterBaseURL, b.APIKey, payload, openRouterAttributionHeaders)
	if err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("openrouter chat request: %w", err)
	}
	if status != http.StatusOK {
		return "", nil, ConsumptionReport{}, fmt.Errorf("openrouter API error %d: %s", status, string(body))
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

	// Some models (e.g. DeepSeek via OpenRouter) embed tool calls as DSML text
	// in the content field instead of returning structured tool_calls.
	content := msg.Content
	if len(toolCalls) == 0 && strings.Contains(content, "DSML") {
		content, toolCalls = parseDSMLToolCalls(content, tools)
	}

	return content, toolCalls, report, nil
}

// dsml* regexes parse tool calls embedded as DSML markup in model text output.
// Matches both fullwidth ｜ (U+FF5C) and ASCII | characters.
var (
	dsmlBlockRe  = regexp.MustCompile(`(?s)<[｜|]{2}DSML[｜|]{2}tool_calls>(.*?)</[｜|]{2}DSML[｜|]{2}tool_calls>`)
	dsmlInvokeRe = regexp.MustCompile(`(?s)<[｜|]{2}DSML[｜|]{2}invoke\s+name="([^"]+)">(.*?)</[｜|]{2}DSML[｜|]{2}invoke>`)
	dsmlParamRe  = regexp.MustCompile(`(?s)<[｜|]{2}DSML[｜|]{2}parameter\s+name="([^"]+)">(.*?)</[｜|]{2}DSML[｜|]{2}parameter>`)
)

// parseDSMLToolCalls extracts tool calls from DSML-embedded text.
// Returns content with DSML blocks removed and the extracted ToolCalls.
// Tool names are matched against registered tools: exact first, then suffix after "_".
func parseDSMLToolCalls(content string, tools []ToolDefinition) (string, []ToolCall) {
	blocks := dsmlBlockRe.FindAllStringSubmatch(content, -1)
	if len(blocks) == 0 {
		return content, nil
	}

	var calls []ToolCall
	for i, block := range blocks {
		for _, invoke := range dsmlInvokeRe.FindAllStringSubmatch(block[1], -1) {
			name := matchDSMLToolName(invoke[1], tools)
			params := make(map[string]interface{})
			for _, param := range dsmlParamRe.FindAllStringSubmatch(invoke[2], -1) {
				params[param[1]] = strings.TrimSpace(param[2])
			}
			argsJSON, _ := json.Marshal(params)
			calls = append(calls, ToolCall{
				ID:       fmt.Sprintf("dsml-%d-%d", i, len(calls)),
				Name:     name,
				ArgsJSON: string(argsJSON),
			})
		}
	}

	return strings.TrimSpace(dsmlBlockRe.ReplaceAllString(content, "")), calls
}

// matchDSMLToolName finds the registered tool name for a DSML-provided name.
// Tries exact match first, then suffix match after the first "_" separator
// (e.g. DSML "resolve-library-id" → registered "context7_resolve-library-id").
func matchDSMLToolName(dsmlName string, tools []ToolDefinition) string {
	for _, t := range tools {
		if t.Name == dsmlName {
			return t.Name
		}
	}
	for _, t := range tools {
		if idx := strings.Index(t.Name, "_"); idx >= 0 && t.Name[idx+1:] == dsmlName {
			return t.Name
		}
	}
	return dsmlName
}

// ChatJSON enforces a JSON schema response via OpenRouter's response_format.
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
	if b.MaxTokensFn != nil {
		if n := b.MaxTokensFn(); n > 0 {
			payload["max_tokens"] = n
		}
	}

	body, status, err := doPostWithFallback(ctx, b.Client, openRouterBaseURL, b.APIKey, payload, openRouterAttributionHeaders)
	if err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("openrouter json chat request: %w", err)
	}
	if status != http.StatusOK {
		return "", ConsumptionReport{}, fmt.Errorf("openrouter API error %d: %s", status, string(body))
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
