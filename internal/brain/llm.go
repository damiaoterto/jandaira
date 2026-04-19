package brain

import "context"

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type ToolCall struct {
	ID       string
	Name     string
	ArgsJSON string
}

type Message struct {
	Role       MessageRole
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

type ConsumptionReport struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type Brain interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, ConsumptionReport, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	GetProviderName() string
}

// StructuredBrain is an optional extension of Brain for providers that support
// native structured/JSON output, guaranteeing schema-valid responses.
type StructuredBrain interface {
	Brain
	ChatJSON(ctx context.Context, messages []Message, schema map[string]interface{}) (string, ConsumptionReport, error)
}
