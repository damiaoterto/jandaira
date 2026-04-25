package brain

import "context"

// Result represents a matched document/memory from the vector database.
type Result struct {
	ID       string
	Score    float32
	Metadata map[string]string
}

// Honeycomb defines the contract for the hive's vector memory store.
type Honeycomb interface {
	// Store upserts a vector with metadata into the collection.
	Store(ctx context.Context, collection string, id string, vector []float32, metadata map[string]string) error

	// Search returns up to limit results ranked by semantic similarity.
	Search(ctx context.Context, collection string, query []float32, limit int) ([]Result, error)

	// EnsureCollection creates the collection if it does not exist.
	EnsureCollection(ctx context.Context, collection string, dimension int) error

	// DeleteByFilter removes all documents whose metadata matches every filter pair.
	DeleteByFilter(ctx context.Context, collection string, filter map[string]string) error
}

// Document is the unit stored in a Honeycomb collection.
type Document struct {
	ID       string            `json:"id"`
	Vector   []float32         `json:"vector"`
	Metadata map[string]string `json:"metadata"`
}
