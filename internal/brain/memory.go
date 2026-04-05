package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

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

type Document struct {
	ID       string            `json:"id"`
	Vector   []float32         `json:"vector"`
	Metadata map[string]string `json:"metadata"`
}

type LocalVectorDB struct {
	dbPath      string
	collections map[string]map[string]Document
	mu          sync.RWMutex
}

func NewLocalVectorDB(dbPath string) (*LocalVectorDB, error) {
	db := &LocalVectorDB{
		dbPath:      dbPath,
		collections: make(map[string]map[string]Document),
	}

	// Load existing data from disk if the file already exists.
	// The parent directory is created automatically on the first save.
	if _, err := os.Stat(dbPath); err == nil {
		data, err := os.ReadFile(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read memory file %q: %w", dbPath, err)
		}
		if err := json.Unmarshal(data, &db.collections); err != nil {
			return nil, fmt.Errorf("memory file %q is corrupted (invalid JSON): %w", dbPath, err)
		}
	}
	return db, nil
}

func (db *LocalVectorDB) save() error {
	dir := filepath.Dir(db.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(db.collections, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(db.dbPath, data, 0644)
}

func (db *LocalVectorDB) EnsureCollection(ctx context.Context, collection string, dimension int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.collections[collection]; !exists {
		db.collections[collection] = make(map[string]Document)
		return db.save()
	}
	return nil
}

func (db *LocalVectorDB) Store(ctx context.Context, collection string, id string, vector []float32, metadata map[string]string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.collections[collection]; !exists {
		db.collections[collection] = make(map[string]Document)
	}

	db.collections[collection][id] = Document{
		ID:       id,
		Vector:   vector,
		Metadata: metadata,
	}

	return db.save()
}

func cosineSimilarity(a, b []float32) float32 {
	var dotProduct, normA, normB float32
	for i := 0; i < len(a) && i < len(b); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

func (db *LocalVectorDB) Search(ctx context.Context, collection string, query []float32, limit int) ([]Result, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	docs, exists := db.collections[collection]
	if !exists || len(docs) == 0 {
		return nil, nil
	}

	var results []Result
	for _, doc := range docs {
		score := cosineSimilarity(query, doc.Vector)
		if score > 0.7 {
			results = append(results, Result{
				ID:       doc.ID,
				Score:    score,
				Metadata: doc.Metadata,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
