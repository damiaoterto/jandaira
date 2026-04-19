package brain

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

// QdrantHoneycomb implements the Honeycomb interface backed by a Qdrant instance.
type QdrantHoneycomb struct {
	client      *qdrant.Client
	collections map[string]int
	mu          sync.RWMutex
}

// NewQdrantHoneycomb creates a QdrantHoneycomb connected to the given Qdrant host and port.
func NewQdrantHoneycomb(host string, port int) (*QdrantHoneycomb, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}
	return &QdrantHoneycomb{
		client:      client,
		collections: make(map[string]int),
	}, nil
}

func (q *QdrantHoneycomb) EnsureCollection(ctx context.Context, collection string, dimension int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	exists, err := q.client.CollectionExists(ctx, collection)
	if err != nil {
		return fmt.Errorf("failed to check qdrant collection %q: %w", collection, err)
	}
	if exists {
		q.collections[collection] = dimension
		return nil
	}

	if err := q.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(dimension),
			Distance: qdrant.Distance_Cosine,
		}),
	}); err != nil {
		return fmt.Errorf("failed to create qdrant collection %q: %w", collection, err)
	}
	q.collections[collection] = dimension
	return nil
}

func stringToPointID(id string) *qdrant.PointId {
	return qdrant.NewIDUUID(uuid.NewSHA1(uuid.Nil, []byte(id)).String())
}

func (q *QdrantHoneycomb) Store(ctx context.Context, collection string, id string, vector []float32, metadata map[string]string) error {
	payload := make(map[string]any, len(metadata)+1)
	for k, v := range metadata {
		payload[k] = v
	}
	payload["_id"] = id

	_, err := q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points: []*qdrant.PointStruct{
			{
				Id:      stringToPointID(id),
				Vectors: qdrant.NewVectors(vector...),
				Payload: qdrant.NewValueMap(payload),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upsert to qdrant collection %q: %w", collection, err)
	}
	return nil
}

func (q *QdrantHoneycomb) Search(ctx context.Context, collection string, query []float32, limit int) ([]Result, error) {
	limitU := uint64(limit)
	scored, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(query...),
		Limit:          &limitU,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant query failed: %w", err)
	}

	results := make([]Result, 0, len(scored))
	for _, r := range scored {
		metadata := make(map[string]string)
		originalID := ""
		for k, v := range r.GetPayload() {
			if k == "_id" {
				originalID = v.GetStringValue()
				continue
			}
			metadata[k] = v.GetStringValue()
		}
		if originalID == "" {
			originalID = r.GetId().String()
		}
		results = append(results, Result{
			ID:       originalID,
			Score:    r.GetScore(),
			Metadata: metadata,
		})
	}
	return results, nil
}
