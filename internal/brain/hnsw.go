package brain

import (
	"container/heap"
	"math"
	"math/rand"
	"sort"
	"strings"
)

const (
	hnswM        = 16
	hnswMmax0    = 32
	hnswEfConstr = 100
	hnswEfSearch = 50
)

// hnswML is the level multiplier: 1/ln(M). Computed once at init.
var hnswML = 1.0 / math.Log(float64(hnswM))

// hnswCandidate pairs a node ID with its cosine distance to a query.
type hnswCandidate struct {
	id   string
	dist float32
}

// minHeap is a min-heap of hnswCandidate ordered by ascending distance.
type hnswMinHeap []hnswCandidate

func (h hnswMinHeap) Len() int           { return len(h) }
func (h hnswMinHeap) Less(i, j int) bool { return h[i].dist < h[j].dist }
func (h hnswMinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hnswMinHeap) Push(x any)        { *h = append(*h, x.(hnswCandidate)) }
func (h *hnswMinHeap) Pop() any          { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }

// maxHeap is a max-heap of hnswCandidate ordered by descending distance (worst first).
type hnswMaxHeap []hnswCandidate

func (h hnswMaxHeap) Len() int           { return len(h) }
func (h hnswMaxHeap) Less(i, j int) bool { return h[i].dist > h[j].dist }
func (h hnswMaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hnswMaxHeap) Push(x any)        { *h = append(*h, x.(hnswCandidate)) }
func (h *hnswMaxHeap) Pop() any          { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }

type hnswNode struct {
	id          string
	vector      []float32
	connections [][]string // connections[level] = slice of neighbor IDs
	deleted     bool
}

// HNSWIndex is a Hierarchical Navigable Small World approximate-nearest-neighbour
// graph. Caller must serialise access with an external mutex.
type HNSWIndex struct {
	nodes      map[string]*hnswNode
	entryPoint string
	maxLevel   int
	rng        *rand.Rand
}

func newHNSWIndex() *HNSWIndex {
	return &HNSWIndex{
		nodes: make(map[string]*hnswNode),
		rng:   rand.New(rand.NewSource(42)),
	}
}

func (h *HNSWIndex) randomLevel() int {
	return int(math.Floor(-math.Log(h.rng.Float64()+1e-10) * hnswML))
}

func cosineDistance(a, b []float32) float32 {
	var dot, na, nb float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := range n {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 1
	}
	sim := dot / (float32(math.Sqrt(float64(na))) * float32(math.Sqrt(float64(nb))))
	return 1 - sim
}

// Insert adds or updates a node. Existing node with same ID is replaced.
func (h *HNSWIndex) Insert(id string, vec []float32) {
	// Soft-remove existing node before re-inserting (handles updates).
	if existing, ok := h.nodes[id]; ok {
		existing.deleted = true
	}

	level := h.randomLevel()
	node := &hnswNode{
		id:          id,
		vector:      vec,
		connections: make([][]string, level+1),
	}
	for i := range node.connections {
		node.connections[i] = make([]string, 0, hnswM)
	}

	if h.entryPoint == "" {
		h.nodes[id] = node
		h.entryPoint = id
		h.maxLevel = level
		return
	}

	h.nodes[id] = node

	ep := h.entryPoint
	for lc := h.maxLevel; lc > level; lc-- {
		cs := h.searchLayer(vec, ep, 1, lc)
		if len(cs) > 0 {
			ep = cs[0].id
		}
	}

	for lc := min(level, h.maxLevel); lc >= 0; lc-- {
		mmax := hnswM
		if lc == 0 {
			mmax = hnswMmax0
		}

		cs := h.searchLayer(vec, ep, hnswEfConstr, lc)
		neighbors := h.selectNeighbors(cs, mmax)

		node.connections[lc] = make([]string, 0, len(neighbors))
		for _, nb := range neighbors {
			node.connections[lc] = append(node.connections[lc], nb.id)
		}

		// Bidirectional connections.
		for _, nb := range neighbors {
			nbNode, ok := h.nodes[nb.id]
			if !ok || nbNode.deleted || lc >= len(nbNode.connections) {
				continue
			}
			nbNode.connections[lc] = append(nbNode.connections[lc], id)
			if len(nbNode.connections[lc]) > mmax {
				nbNode.connections[lc] = h.shrinkConnections(nbNode.vector, nbNode.connections[lc], mmax)
			}
		}

		if len(cs) > 0 {
			ep = cs[0].id
		}
	}

	if level > h.maxLevel {
		h.maxLevel = level
		h.entryPoint = id
	}
}

// searchLayer performs a beam search within a single layer.
func (h *HNSWIndex) searchLayer(query []float32, entryPoint string, ef, layer int) []hnswCandidate {
	epNode, ok := h.nodes[entryPoint]
	if !ok {
		return nil
	}

	epDist := cosineDistance(query, epNode.vector)
	visited := map[string]bool{entryPoint: true}

	cands := &hnswMinHeap{{id: entryPoint, dist: epDist}}
	heap.Init(cands)

	found := &hnswMaxHeap{{id: entryPoint, dist: epDist}}
	heap.Init(found)

	for cands.Len() > 0 {
		curr := heap.Pop(cands).(hnswCandidate)

		if found.Len() >= ef && curr.dist > (*found)[0].dist {
			break
		}

		currNode, ok := h.nodes[curr.id]
		if !ok || currNode.deleted || layer >= len(currNode.connections) {
			continue
		}

		for _, neighborID := range currNode.connections[layer] {
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true

			nbNode, ok := h.nodes[neighborID]
			if !ok || nbNode.deleted {
				continue
			}

			dist := cosineDistance(query, nbNode.vector)
			if found.Len() < ef || dist < (*found)[0].dist {
				heap.Push(cands, hnswCandidate{id: neighborID, dist: dist})
				heap.Push(found, hnswCandidate{id: neighborID, dist: dist})
				if found.Len() > ef {
					heap.Pop(found)
				}
			}
		}
	}

	// Return sorted ascending (closest first).
	out := make([]hnswCandidate, found.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(found).(hnswCandidate)
	}
	return out
}

func (h *HNSWIndex) selectNeighbors(cs []hnswCandidate, m int) []hnswCandidate {
	if len(cs) <= m {
		return cs
	}
	return cs[:m]
}

func (h *HNSWIndex) shrinkConnections(origin []float32, conns []string, mmax int) []string {
	if len(conns) <= mmax {
		return conns
	}
	scored := make([]hnswCandidate, 0, len(conns))
	for _, nid := range conns {
		nb, ok := h.nodes[nid]
		if !ok || nb.deleted {
			continue
		}
		scored = append(scored, hnswCandidate{id: nid, dist: cosineDistance(origin, nb.vector)})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].dist < scored[j].dist })
	result := make([]string, 0, mmax)
	for i := 0; i < mmax && i < len(scored); i++ {
		result = append(result, scored[i].id)
	}
	return result
}

// Search returns up to k nearest neighbours sorted by ascending cosine distance.
func (h *HNSWIndex) Search(query []float32, k int) []hnswCandidate {
	if h.entryPoint == "" {
		return nil
	}

	ep := h.entryPoint
	for lc := h.maxLevel; lc > 0; lc-- {
		cs := h.searchLayer(query, ep, 1, lc)
		if len(cs) > 0 {
			ep = cs[0].id
		}
	}

	ef := hnswEfSearch
	if k > ef {
		ef = k
	}
	cs := h.searchLayer(query, ep, ef, 0)
	if len(cs) > k {
		cs = cs[:k]
	}
	return cs
}

// Delete soft-deletes a node; excluded from future searches.
// Call Rebuild after bulk deletions to reclaim memory and restore graph quality.
func (h *HNSWIndex) Delete(id string) {
	if node, ok := h.nodes[id]; ok {
		node.deleted = true
	}
	// If the entry point is deleted, find a replacement.
	if h.entryPoint == id {
		h.entryPoint = ""
		for nid, n := range h.nodes {
			if !n.deleted {
				h.entryPoint = nid
				break
			}
		}
	}
}

// Rebuild reconstructs the graph from the provided live documents,
// discarding all deleted nodes.
func (h *HNSWIndex) Rebuild(docs map[string]Document) {
	fresh := newHNSWIndex()
	for _, doc := range docs {
		fresh.Insert(doc.ID, doc.Vector)
	}
	h.nodes = fresh.nodes
	h.entryPoint = fresh.entryPoint
	h.maxLevel = fresh.maxLevel
}

// Size returns the count of non-deleted nodes.
func (h *HNSWIndex) Size() int {
	n := 0
	for _, node := range h.nodes {
		if !node.deleted {
			n++
		}
	}
	return n
}

// splitCollectionKey parses "enxame-alfa/mem-123" from a raw db key
// by stripping the "d/" prefix and splitting on the first "/".
func splitCollectionKey(rawKey []byte, prefixLen int) (collection, id string, ok bool) {
	rest := string(rawKey[prefixLen:])
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "", "", false
	}
	return rest[:slash], rest[slash+1:], true
}
