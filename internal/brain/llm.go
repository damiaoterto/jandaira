package brain

import "context"

// MessageRole defines the role of the message sender.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// Message represents a single interaction in the conversation history.
type Message struct {
	Role    MessageRole
	Content string
}

// ConsumptionReport tracks the nectar (token) usage of a request.
type ConsumptionReport struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Brain defines the contract for any LLM provider used by the Queen.
type Brain interface {
	// Chat processes a conversation and returns the response from the LLM.
	Chat(ctx context.Context, messages []Message) (string, ConsumptionReport, error)

	// Embed generates vector embeddings for text (used by LanceDB).
	Embed(ctx context.Context, text string) ([]float32, error)

	// GetProviderName returns the name of the LLM provider (e.g., "openai").
	GetProviderName() string
}
