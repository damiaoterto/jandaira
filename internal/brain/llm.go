package brain

import "context"

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool" // Novo: papel para os resultados das ferramentas
)

// ToolCall representa o pedido da IA para invocar uma função
type ToolCall struct {
	ID       string
	Name     string
	ArgsJSON string
}

// Message agora suporta chamadas de ferramentas e IDs
type Message struct {
	Role       MessageRole
	Content    string
	ToolCalls  []ToolCall // Usado quando a IA (assistant) pede ferramentas
	ToolCallID string     // Usado quando devolvemos o resultado (tool) à IA
}

// ToolDefinition ajuda o Brain a traduzir as ferramentas Go para o formato da API (ex: OpenAI)
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

// Brain foi atualizado para receber as definições das ferramentas e devolver chamadas
type Brain interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (string, []ToolCall, ConsumptionReport, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	GetProviderName() string
}
