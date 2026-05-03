package brain

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

const defaultGeminiEmbedModel = "gemini-embedding-2"

type GeminiBrain struct {
	APIKey      string
	Model       string
	MaxTokensFn func() int
	client      *genai.Client
}

func NewGeminiBrain(apiKey, model string) (*GeminiBrain, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini client init: %w", err)
	}
	return &GeminiBrain{
		APIKey: apiKey,
		Model:  model,
		client: client,
	}, nil
}

func (b *GeminiBrain) GetProviderName() string { return "gemini" }

func (b *GeminiBrain) Embed(ctx context.Context, text string) ([]float32, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(text, genai.RoleUser),
	}
	result, err := b.client.Models.EmbedContent(ctx, defaultGeminiEmbedModel, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: %w", err)
	}
	if len(result.Embeddings) == 0 || len(result.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("gemini embed: empty response")
	}
	return result.Embeddings[0].Values, nil
}

func (b *GeminiBrain) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, ConsumptionReport, error) {
	var systemInstruction *genai.Content
	var contents []*genai.Content

	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			systemInstruction = genai.NewContentFromText(msg.Content, genai.RoleUser)

		case RoleUser:
			contents = append(contents, genai.NewContentFromText(msg.Content, genai.RoleUser))

		case RoleAssistant:
			var parts []*genai.Part
			if msg.Content != "" {
				parts = append(parts, genai.NewPartFromText(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.ArgsJSON), &args)
				parts = append(parts, genai.NewPartFromFunctionCall(tc.Name, args))
				// store call ID on the FunctionCall so Gemini can match the response
				if tc.ID != "" && parts[len(parts)-1].FunctionCall != nil {
					parts[len(parts)-1].FunctionCall.ID = tc.ID
				}
			}
			if len(parts) > 0 {
				contents = append(contents, &genai.Content{Role: "model", Parts: parts})
			}

		case RoleTool:
			var response map[string]any
			if err := json.Unmarshal([]byte(msg.Content), &response); err != nil {
				response = map[string]any{"output": msg.Content}
			}
			fr := &genai.FunctionResponse{
				ID:       msg.ToolCallID,
				Name:     msg.ToolCallID,
				Response: response,
			}
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{FunctionResponse: fr}},
			})
		}
	}

	cfg := &genai.GenerateContentConfig{}
	if systemInstruction != nil {
		cfg.SystemInstruction = systemInstruction
	}
	if b.MaxTokensFn != nil {
		if n := b.MaxTokensFn(); n > 0 {
			cfg.MaxOutputTokens = int32(n)
		}
	}
	if len(tools) > 0 {
		cfg.Tools = []*genai.Tool{buildGeminiTools(tools)}
	}

	resp, err := b.client.Models.GenerateContent(ctx, b.Model, contents, cfg)
	if err != nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("gemini chat: %w", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", nil, ConsumptionReport{}, fmt.Errorf("gemini returned no candidates")
	}

	report := ConsumptionReport{}
	if resp.UsageMetadata != nil {
		report.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
		report.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
		report.TotalTokens = int(resp.UsageMetadata.TotalTokenCount)
	}

	var text string
	var toolCalls []ToolCall

	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			text += part.Text
		}
		if part.FunctionCall != nil {
			fc := part.FunctionCall
			id := fc.ID
			if id == "" {
				id = fc.Name
			}
			argsJSON, _ := json.Marshal(fc.Args)
			toolCalls = append(toolCalls, ToolCall{
				ID:       id,
				Name:     fc.Name,
				ArgsJSON: string(argsJSON),
			})
		}
	}

	return text, toolCalls, report, nil
}

// ChatJSON enforces a JSON schema response via Gemini's native structured
// output. The schema parameter must follow the OpenAI json_schema envelope
// format (name, schema fields); the inner schema is converted to a Gemini
// Schema and set on ResponseSchema.
func (b *GeminiBrain) ChatJSON(ctx context.Context, messages []Message, schema map[string]interface{}) (string, ConsumptionReport, error) {
	var systemInstruction *genai.Content
	var contents []*genai.Content

	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			systemInstruction = genai.NewContentFromText(msg.Content, genai.RoleUser)
		case RoleUser:
			contents = append(contents, genai.NewContentFromText(msg.Content, genai.RoleUser))
		case RoleAssistant:
			if msg.Content != "" {
				contents = append(contents, genai.NewContentFromText(msg.Content, "model"))
			}
		}
	}

	innerSchema, _ := schema["schema"].(map[string]any)
	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   jsonSchemaToGemini(innerSchema),
	}
	if systemInstruction != nil {
		cfg.SystemInstruction = systemInstruction
	}
	if b.MaxTokensFn != nil {
		if n := b.MaxTokensFn(); n > 0 {
			cfg.MaxOutputTokens = int32(n)
		}
	}

	resp, err := b.client.Models.GenerateContent(ctx, b.Model, contents, cfg)
	if err != nil {
		return "", ConsumptionReport{}, fmt.Errorf("gemini json chat: %w", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", ConsumptionReport{}, fmt.Errorf("gemini returned no candidates")
	}

	report := ConsumptionReport{}
	if resp.UsageMetadata != nil {
		report.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
		report.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
		report.TotalTokens = int(resp.UsageMetadata.TotalTokenCount)
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			return part.Text, report, nil
		}
	}
	return "", report, fmt.Errorf("gemini json chat: no text in response")
}

func buildGeminiTools(tools []ToolDefinition) *genai.Tool {
	fns := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		fns = append(fns, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  jsonSchemaToGemini(t.Parameters),
		})
	}
	return &genai.Tool{FunctionDeclarations: fns}
}

func jsonSchemaToGemini(m map[string]interface{}) *genai.Schema {
	s := &genai.Schema{Type: genai.TypeObject}
	if props, ok := m["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*genai.Schema, len(props))
		for k, v := range props {
			if vm, ok := v.(map[string]interface{}); ok {
				s.Properties[k] = mapFieldToGemini(vm)
			}
		}
	}
	if req, ok := m["required"].([]interface{}); ok {
		for _, r := range req {
			if rs, ok := r.(string); ok {
				s.Required = append(s.Required, rs)
			}
		}
	}
	return s
}

func mapFieldToGemini(m map[string]interface{}) *genai.Schema {
	s := &genai.Schema{}
	if t, ok := m["type"].(string); ok {
		switch t {
		case "string":
			s.Type = genai.TypeString
		case "number", "float":
			s.Type = genai.TypeNumber
		case "integer":
			s.Type = genai.TypeInteger
		case "boolean":
			s.Type = genai.TypeBoolean
		case "array":
			s.Type = genai.TypeArray
		case "object":
			s.Type = genai.TypeObject
		}
	}
	if desc, ok := m["description"].(string); ok {
		s.Description = desc
	}
	if props, ok := m["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*genai.Schema, len(props))
		for k, v := range props {
			if vm, ok := v.(map[string]interface{}); ok {
				s.Properties[k] = mapFieldToGemini(vm)
			}
		}
	}
	if req, ok := m["required"].([]interface{}); ok {
		for _, r := range req {
			if rs, ok := r.(string); ok {
				s.Required = append(s.Required, rs)
			}
		}
	}
	if items, ok := m["items"].(map[string]interface{}); ok {
		s.Items = mapFieldToGemini(items)
	}
	return s
}
