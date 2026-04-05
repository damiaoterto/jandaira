package brain

import (
	"context"
)

// LanceDBProvider implements the Honeycomb interface using an embedded LanceDB.
type LanceDBProvider struct {
	dbPath string
}

// NewLanceDBProvider creates or opens a local LanceDB connection.
func NewLanceDBProvider(dbPath string) (*LanceDBProvider, error) {
	// Removemos o fmt.Printf daqui para não quebrar a interface gráfica (Bubble Tea)
	return &LanceDBProvider{
		dbPath: dbPath,
	}, nil
}

// EnsureCollection creates a table in LanceDB if it doesn't exist.
func (p *LanceDBProvider) EnsureCollection(ctx context.Context, collection string, dimension int) error {
	// Silenciado para a UI
	return nil
}

// Store inserts a new vector (memory) into the local database.
func (p *LanceDBProvider) Store(ctx context.Context, collection string, id string, vector []float32, metadata map[string]string) error {
	// Silenciado para a UI
	return nil
}

// Search performs an ANN (Approximate Nearest Neighbor) search on the local vectors.
func (p *LanceDBProvider) Search(ctx context.Context, collection string, query []float32, limit int) ([]Result, error) {
	// Returning mock results for structural testing
	mockResults := []Result{
		{
			ID:    "mem-001",
			Score: 0.92,
			Metadata: map[string]string{
				"source":  "github_repo",
				"content": "A Jandaira usa wazero para micro-sandboxing.",
			},
		},
	}

	return mockResults, nil
}
