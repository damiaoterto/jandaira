package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Node represents an entity in the knowledge graph.
// Types: "agent", "topic", "tool".
type Node struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Label     string            `json:"label"`
	Props     map[string]string `json:"props,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Edge is a directed, typed relationship between two nodes.
// Common Rel values: "expert_in", "uses_tool", "related_to".
type Edge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Rel    string  `json:"rel"`
	Weight float32 `json:"weight"` // 0–1, higher = stronger
}

// KnowledgeGraph defines the contract for the hive's knowledge graph.
// It lets the Queen remember which specialist profiles handled which kinds
// of tasks and delegate future work with more precision.
type KnowledgeGraph interface {
	AddNode(ctx context.Context, node Node) error
	AddEdge(ctx context.Context, edge Edge) error
	GetNode(ctx context.Context, id string) (Node, bool)
	// GetNeighbors returns nodes reachable from nodeID via rel.
	// Pass rel="" to follow all outgoing edges.
	GetNeighbors(ctx context.Context, nodeID string, rel string) ([]Node, error)
	// FindExperts returns agent nodes linked by "expert_in" edges to topic nodes
	// whose labels contain the given topic string (case-insensitive).
	FindExperts(ctx context.Context, topic string) ([]Node, error)
	// QueryByType returns all nodes of the given type.
	QueryByType(ctx context.Context, nodeType string) ([]Node, error)
}

// graphData is the serialised form written to disk.
type graphData struct {
	Nodes map[string]Node `json:"nodes"`
	Edges []Edge          `json:"edges"`
}

// LocalKnowledgeGraph is a JSON file-backed, in-memory knowledge graph.
// All writes are persisted synchronously so the graph survives restarts.
type LocalKnowledgeGraph struct {
	mu   sync.RWMutex
	path string
	data graphData
}

// NewLocalKnowledgeGraph opens (or creates) a knowledge graph at path.
func NewLocalKnowledgeGraph(path string) (*LocalKnowledgeGraph, error) {
	g := &LocalKnowledgeGraph{
		path: path,
		data: graphData{Nodes: make(map[string]Node)},
	}
	if _, err := os.Stat(path); err == nil {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read graph file %q: %w", path, err)
		}
		if err := json.Unmarshal(raw, &g.data); err != nil {
			return nil, fmt.Errorf("graph file %q is corrupted: %w", path, err)
		}
		if g.data.Nodes == nil {
			g.data.Nodes = make(map[string]Node)
		}
	}
	return g, nil
}

func (g *LocalKnowledgeGraph) save() error {
	if err := os.MkdirAll(filepath.Dir(g.path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(g.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(g.path, raw, 0644)
}

func (g *LocalKnowledgeGraph) AddNode(ctx context.Context, node Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}
	g.data.Nodes[node.ID] = node
	return g.save()
}

func (g *LocalKnowledgeGraph) AddEdge(ctx context.Context, edge Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	// Deduplicate: skip if the same directed edge already exists.
	for _, e := range g.data.Edges {
		if e.From == edge.From && e.To == edge.To && e.Rel == edge.Rel {
			return nil
		}
	}
	g.data.Edges = append(g.data.Edges, edge)
	return g.save()
}

func (g *LocalKnowledgeGraph) GetNode(ctx context.Context, id string) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.data.Nodes[id]
	return n, ok
}

func (g *LocalKnowledgeGraph) GetNeighbors(ctx context.Context, nodeID string, rel string) ([]Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []Node
	for _, edge := range g.data.Edges {
		if edge.From != nodeID {
			continue
		}
		if rel != "" && edge.Rel != rel {
			continue
		}
		if n, ok := g.data.Nodes[edge.To]; ok {
			result = append(result, n)
		}
	}
	return result, nil
}

func (g *LocalKnowledgeGraph) FindExperts(ctx context.Context, topic string) ([]Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	topicLower := strings.ToLower(topic)

	// Collect topic node IDs whose label contains the search term.
	topicIDs := map[string]bool{}
	for id, node := range g.data.Nodes {
		if node.Type == "topic" && strings.Contains(strings.ToLower(node.Label), topicLower) {
			topicIDs[id] = true
		}
	}

	// Collect agent nodes connected to any of those topics.
	expertIDs := map[string]bool{}
	for _, edge := range g.data.Edges {
		if edge.Rel == "expert_in" && topicIDs[edge.To] {
			expertIDs[edge.From] = true
		}
	}

	var experts []Node
	for id := range expertIDs {
		if n, ok := g.data.Nodes[id]; ok {
			experts = append(experts, n)
		}
	}
	return experts, nil
}

func (g *LocalKnowledgeGraph) QueryByType(ctx context.Context, nodeType string) ([]Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []Node
	for _, node := range g.data.Nodes {
		if node.Type == nodeType {
			result = append(result, node)
		}
	}
	return result, nil
}
