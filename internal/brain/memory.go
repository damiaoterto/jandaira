package brain

import "context"

// Result represents a matched document/memory from the vector database.
type Result struct {
	ID       string
	Score    float32
	Metadata map[string]string
}

// Honeycomb defines the contract for the hive's memory (Vector DB).
// This allows swapping LanceDB for ChromaDB or Pinecone in the future if needed.
type Honeycomb interface {
	// Store saves a new memory vector in the specified collection/namespace.
	Store(ctx context.Context, collection string, id string, vector []float32, metadata map[string]string) error

	// Search finds the most similar memories based on a query vector.
	Search(ctx context.Context, collection string, query []float32, limit int) ([]Result, error)

	// EnsureCollection makes sure a collection exists (creates it if not).
	EnsureCollection(ctx context.Context, collection string, dimension int) error
}
