package brain

import (
	"context"
	"fmt"
	"sync"

	chromav2 "github.com/amikos-tech/chroma-go/pkg/api/v2"
	chromaembeddings "github.com/amikos-tech/chroma-go/pkg/embeddings"
)

// ChromaHoneycomb implements the Honeycomb interface backed by a ChromaDB instance.
type ChromaHoneycomb struct {
	client      chromav2.Client
	collections map[string]chromav2.Collection
	mu          sync.RWMutex
}

// NewChromaHoneycomb creates a ChromaHoneycomb connected to the given ChromaDB base URL.
// Example: NewChromaHoneycomb(ctx, "http://localhost:8000")
func NewChromaHoneycomb(ctx context.Context, baseURL string) (*ChromaHoneycomb, error) {
	client, err := chromav2.NewHTTPClient(
		chromav2.WithBaseURL(baseURL),
		chromav2.WithDefaultDatabaseAndTenant(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chroma client: %w", err)
	}

	if err := client.PreFlight(ctx); err != nil {
		return nil, fmt.Errorf("chroma preflight check failed (is the server running at %s?): %w", baseURL, err)
	}

	return &ChromaHoneycomb{
		client:      client,
		collections: make(map[string]chromav2.Collection),
	}, nil
}

func (c *ChromaHoneycomb) EnsureCollection(ctx context.Context, collection string, dimension int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.collections[collection]; exists {
		return nil
	}

	col, err := c.client.GetOrCreateCollection(ctx, collection)
	if err != nil {
		return fmt.Errorf("failed to ensure chroma collection %q: %w", collection, err)
	}
	c.collections[collection] = col
	return nil
}

func (c *ChromaHoneycomb) getCollection(ctx context.Context, name string) (chromav2.Collection, error) {
	c.mu.RLock()
	col, exists := c.collections[name]
	c.mu.RUnlock()

	if exists {
		return col, nil
	}

	// Collection wasn't pre-created via EnsureCollection — create it on demand.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring the write lock.
	if col, exists = c.collections[name]; exists {
		return col, nil
	}

	col, err := c.client.GetOrCreateCollection(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get chroma collection %q: %w", name, err)
	}
	c.collections[name] = col
	return col, nil
}

func (c *ChromaHoneycomb) Store(ctx context.Context, collection string, id string, vector []float32, metadata map[string]string) error {
	col, err := c.getCollection(ctx, collection)
	if err != nil {
		return err
	}

	emb := &chromaembeddings.Float32Embedding{}
	if err := emb.FromFloat32(vector...); err != nil {
		return fmt.Errorf("failed to build embedding: %w", err)
	}

	rawMeta := make(map[string]interface{}, len(metadata))
	for k, v := range metadata {
		rawMeta[k] = v
	}
	meta, err := chromav2.NewDocumentMetadataFromMap(rawMeta)
	if err != nil {
		return fmt.Errorf("failed to build metadata: %w", err)
	}

	return col.Upsert(ctx,
		chromav2.WithIDs(chromav2.DocumentID(id)),
		chromav2.WithEmbeddings(emb),
		chromav2.WithMetadatas(meta),
	)
}

func (c *ChromaHoneycomb) Search(ctx context.Context, collection string, query []float32, limit int) ([]Result, error) {
	col, err := c.getCollection(ctx, collection)
	if err != nil {
		return nil, err
	}

	emb := &chromaembeddings.Float32Embedding{}
	if err := emb.FromFloat32(query...); err != nil {
		return nil, fmt.Errorf("failed to build query embedding: %w", err)
	}

	qr, err := col.Query(ctx,
		chromav2.WithQueryEmbeddings(emb),
		chromav2.WithNResults(limit),
		chromav2.WithInclude(chromav2.IncludeMetadatas, chromav2.IncludeDistances),
	)
	if err != nil {
		return nil, fmt.Errorf("chroma query failed: %w", err)
	}

	idGroups := qr.GetIDGroups()
	metaGroups := qr.GetMetadatasGroups()
	distGroups := qr.GetDistancesGroups()

	if len(idGroups) == 0 {
		return nil, nil
	}

	ids := idGroups[0]
	var metas []chromav2.DocumentMetadata
	if len(metaGroups) > 0 {
		metas = metaGroups[0]
	}
	var dists chromaembeddings.Distances
	if len(distGroups) > 0 {
		dists = distGroups[0]
	}

	results := make([]Result, 0, len(ids))
	for i, docID := range ids {
		// ChromaDB returns cosine distance (0=identical, 2=opposite).
		// Convert to a similarity score in [0,1]: score = 1 - distance/2.
		score := float32(1.0)
		if i < len(dists) {
			score = float32(1.0 - float64(dists[i])/2.0)
		}

		metadata := make(map[string]string)
		if i < len(metas) && metas[i] != nil {
			// DocumentMetadataImpl is the concrete type returned by NewDocumentMetadataFromMap.
			// Keys() is only on the concrete type, not the interface.
			if impl, ok := metas[i].(*chromav2.DocumentMetadataImpl); ok {
				for _, key := range impl.Keys() {
					if v, ok := impl.GetString(key); ok {
						metadata[key] = v
					}
				}
			}
		}

		results = append(results, Result{
			ID:       string(docID),
			Score:    score,
			Metadata: metadata,
		})
	}

	return results, nil
}
