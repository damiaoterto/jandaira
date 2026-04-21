package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/damiaoterto/jandaira/internal/brain"
)

type SearchMemoryTool struct {
	Brain      brain.Brain
	Honeycomb  brain.Honeycomb
	Collection string
}

type StoreMemoryTool struct {
	Brain      brain.Brain
	Honeycomb  brain.Honeycomb
	Collection string
}

func (t *SearchMemoryTool) Name() string { return "search_memory" }

func (t *SearchMemoryTool) Description() string {
	return "Searches the swarm's long-term vector memory using semantic similarity. Use to recall past records, financial entries, audit history, or any previously stored knowledge."
}

func (t *SearchMemoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The question or concept to search for in memory.",
			},
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Optional. The memory collection to search. Use the value from [HIVE MEMORY COLLECTION: ...] in your context if present.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchMemoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query      string `json:"query"`
		Collection string `json:"collection"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	collection := t.Collection
	if args.Collection != "" {
		collection = args.Collection
	}

	vector, embedErr := t.Brain.Embed(ctx, args.Query)
	if embedErr != nil {
		return fmt.Sprintf("Semantic search unavailable (embedding error: %v). Store operations still work in degraded mode.", embedErr), nil
	}

	results, err := t.Honeycomb.Search(ctx, collection, vector, 3)
	if err != nil {
		return "", fmt.Errorf("vector search failed: %w", err)
	}

	if len(results) == 0 {
		return "No memories found for this query.", nil
	}

	var response strings.Builder
	response.WriteString("Memories found:\n")
	for _, res := range results {
		fmt.Fprintf(&response, "- ID: %s | Score: %.2f | Content: %s\n", res.ID, res.Score, res.Metadata["content"])
	}

	return response.String(), nil
}

func (t *StoreMemoryTool) Name() string { return "store_memory" }

func (t *StoreMemoryTool) Description() string {
	return "Persists data to the vector database (Qdrant). This is the ONLY permanent storage mechanism — use it for financial records, calculation results, and any data that must survive across sessions. Never use write_file to persist data."
}

func (t *StoreMemoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The information, summary, or record to store.",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Category of the data (e.g. 'financial_entry', 'calculation_result', 'agent_note').",
			},
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Optional. The memory collection to store into. Use the value from [HIVE MEMORY COLLECTION: ...] in your context if present.",
			},
			"metadata": map[string]interface{}{
				"type":        "object",
				"description": "Optional free-form key-value fields (e.g. {\"category\": \"income\", \"amount\": \"10000\"}).",
				"additionalProperties": map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"content"},
	}
}

const fallbackVectorDim = 1536

func fallbackVector() []float32 {
	v := make([]float32, fallbackVectorDim)
	// uniform unit-ish vector so cosine similarity is defined but scores low
	val := float32(1.0 / fallbackVectorDim)
	for i := range v {
		v[i] = val
	}
	return v
}

func (t *StoreMemoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Content    string            `json:"content"`
		Type       string            `json:"type"`
		Collection string            `json:"collection"`
		Metadata   map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	collection := t.Collection
	if args.Collection != "" {
		collection = args.Collection
	}

	docType := args.Type
	if docType == "" {
		docType = "agent_note"
	}

	meta := map[string]string{
		"content": args.Content,
		"type":    docType,
	}
	for k, v := range args.Metadata {
		meta[k] = v
	}

	vector, embedErr := t.Brain.Embed(ctx, args.Content)
	if embedErr != nil {
		// Embedding unavailable (e.g. Anthropic provider without OpenAI key).
		// Persist data with a fallback vector so no records are lost.
		vector = fallbackVector()
		meta["embedding"] = "none"
	}

	docID := fmt.Sprintf("mem-%d", time.Now().UnixMilli())
	if err := t.Honeycomb.Store(ctx, collection, docID, vector, meta); err != nil {
		return "", fmt.Errorf("failed to store in vector database: %w", err)
	}

	if embedErr != nil {
		return fmt.Sprintf("Stored with ID %s (type: %s) [degraded: no embedding — semantic search disabled for this record].", docID, docType), nil
	}
	return fmt.Sprintf("Stored in vector database with ID %s (type: %s).", docID, docType), nil
}
