package brain

import (
	"context"
	"encoding/json"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultAnthropicMaxTokens = 8096

type AnthropicBrain struct {
	APIKey    string
	Model     string
	MaxTokens int // 0 = use defaultAnthropicMaxTokens
	client    anthropic.Client
}

func NewAnthropicBrain(apiKey, model string) *AnthropicBrain {
	return &AnthropicBrain{
		APIKey: apiKey,
		Model:  model,
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (b *AnthropicBrain) maxTokens() int64 {
	if b.MaxTokens > 0 {
		return int64(b.MaxTokens)
	}
	return defaultAnthropicMaxTokens
}

func (b *AnthropicBrain) GetProviderName() string { return "anthropic" }

// Embed is not supported by Anthropic. Memory features are disabled when
// using this provider.
func (b *AnthropicBrain) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, fmt.Errorf("anthropic provider does not support embeddings; memory features are disabled")
}

func (b *AnthropicBrain) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, ConsumptionReport, error) {
	system, msgParams, err := convertMessages(messages)
	if err != nil {
		return "", nil, ConsumptionReport{}, err
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(b.Model),
		MaxTokens: b.maxTokens(),
		Messages:  msgParams,
	}
	if len(system) > 0 {
		params.System = system
	}
	if len(tools) > 0 {
		params.Tools = convertTools(tools)
	}

	resp, err := b.client.Messages.New(ctx, params)
	if err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("anthropic chat: %w", err)
	}

	report := ConsumptionReport{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}

	var text string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			text += v.Text
		case anthropic.ToolUseBlock:
			argsJSON, _ := json.Marshal(v.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:       v.ID,
				Name:     v.Name,
				ArgsJSON: string(argsJSON),
			})
		}
	}

	return text, toolCalls, report, nil
}

// ChatJSON uses forced tool_choice to guarantee a schema-valid JSON response.
// The schema parameter must follow the OpenAI json_schema format
// (name, strict, schema fields) — the inner schema is extracted and used as
// the Anthropic tool input schema.
func (b *AnthropicBrain) ChatJSON(ctx context.Context, messages []Message, schema map[string]interface{}) (string, ConsumptionReport, error) {
	toolName, _ := schema["name"].(string)
	if toolName == "" {
		toolName = "structured_output"
	}

	innerSchema, _ := schema["schema"].(map[string]any)
	var properties any
	var required []string
	if innerSchema != nil {
		properties = innerSchema["properties"]
		if req, ok := innerSchema["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
		}
	}

	tool := anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name: toolName,
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: properties,
				Required:   required,
			},
		},
	}

	system, msgParams, err := convertMessages(messages)
	if err != nil {
		return "", ConsumptionReport{}, err
	}

	params := anthropic.MessageNewParams{
		Model:      anthropic.Model(b.Model),
		MaxTokens:  8096,
		Messages:   msgParams,
		Tools:      []anthropic.ToolUnionParam{tool},
		ToolChoice: anthropic.ToolChoiceParamOfTool(toolName),
	}
	if len(system) > 0 {
		params.System = system
	}

	resp, err := b.client.Messages.New(ctx, params)
	if err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("anthropic structured chat: %w", err)
	}

	report := ConsumptionReport{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}

	for _, block := range resp.Content {
		if v, ok := block.AsAny().(anthropic.ToolUseBlock); ok && v.Name == toolName {
			argsJSON, err := json.Marshal(v.Input)
			if err != nil {
				return "", report, fmt.Errorf("marshal tool input: %w", err)
			}
			return string(argsJSON), report, nil
		}
	}

	return "", report, fmt.Errorf("anthropic did not return a tool call for '%s'", toolName)
}

// convertMessages splits system blocks off and converts the internal message
// format into Anthropic's message params. Consecutive RoleTool messages are
// grouped into a single user message as required by the Anthropic API.
func convertMessages(messages []Message) ([]anthropic.TextBlockParam, []anthropic.MessageParam, error) {
	var system []anthropic.TextBlockParam
	var params []anthropic.MessageParam
	var pendingToolResults []anthropic.ContentBlockParamUnion

	flushToolResults := func() {
		if len(pendingToolResults) > 0 {
			params = append(params, anthropic.NewUserMessage(pendingToolResults...))
			pendingToolResults = nil
		}
	}

	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			flushToolResults()
			system = append(system, anthropic.TextBlockParam{Text: msg.Content})

		case RoleUser:
			flushToolResults()
			params = append(params, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))

		case RoleAssistant:
			flushToolResults()
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var input interface{}
				_ = json.Unmarshal([]byte(tc.ArgsJSON), &input)
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
			}
			if len(blocks) > 0 {
				params = append(params, anthropic.MessageParam{
					Role:    anthropic.MessageParamRoleAssistant,
					Content: blocks,
				})
			}

		case RoleTool:
			pendingToolResults = append(pendingToolResults, anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false))
		}
	}
	flushToolResults()

	return system, params, nil
}

func convertTools(tools []ToolDefinition) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		var properties interface{}
		var required []string
		if props, ok := t.Parameters["properties"]; ok {
			properties = props
		}
		if req, ok := t.Parameters["required"].([]interface{}); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
		}
		desc := t.Description
		out = append(out, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(desc),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
					Required:   required,
				},
			},
		})
	}
	return out
}
