package brain

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

const (
	keyPrefixDoc = "d/" // d/{collection}/{id} → gob(Document)
	gcInterval   = 5 * time.Minute
	rebuildThreshold = 10 // rebuild HNSW after this many bulk deletions
)

// VectorEngine is an embedded, single-process vector database.
// Storage: BadgerDB (binary key-value). Index: in-memory HNSW per collection.
// Cache: all live document vectors are kept in memory for O(1) retrieval.
// Implements the Honeycomb interface.
type VectorEngine struct {
	db      *badger.DB
	mu      sync.RWMutex
	indexes map[string]*HNSWIndex      // per-collection HNSW graph
	docs    map[string]map[string]Document // hot cache: collection → id → doc
	gcStop  chan struct{}
}

// NewVectorEngine opens (or creates) a VectorEngine at the given directory.
func NewVectorEngine(dir string) (*VectorEngine, error) {
	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open vector engine at %q: %w", dir, err)
	}

	e := &VectorEngine{
		db:      db,
		indexes: make(map[string]*HNSWIndex),
		docs:    make(map[string]map[string]Document),
		gcStop:  make(chan struct{}),
	}

	if err := e.loadAll(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load vector engine: %w", err)
	}

	go e.gcLoop()
	return e, nil
}

// Close flushes pending writes and shuts down the underlying DB.
func (e *VectorEngine) Close() error {
	close(e.gcStop)
	return e.db.Close()
}

func docKey(collection, id string) []byte {
	return []byte(keyPrefixDoc + collection + "/" + id)
}

func encodeDoc(d Document) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeDoc(data []byte) (Document, error) {
	var d Document
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&d); err != nil {
		return Document{}, err
	}
	return d, nil
}

// loadAll rebuilds all in-memory indexes from BadgerDB on startup.
func (e *VectorEngine) loadAll() error {
	return e.db.View(func(txn *badger.Txn) error {
		prefix := []byte(keyPrefixDoc)
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   100,
			Prefix:         prefix,
		})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			if err := item.Value(func(val []byte) error {
				doc, err := decodeDoc(val)
				if err != nil {
					return err
				}
				collection, _, ok := splitCollectionKey(item.Key(), len(keyPrefixDoc))
				if !ok {
					return nil
				}
				if e.docs[collection] == nil {
					e.docs[collection] = make(map[string]Document)
					e.indexes[collection] = newHNSWIndex()
				}
				e.docs[collection][doc.ID] = doc
				e.indexes[collection].Insert(doc.ID, doc.Vector)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// EnsureCollection creates an empty collection if it does not exist.
func (e *VectorEngine) EnsureCollection(_ context.Context, collection string, _ int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.indexes[collection] == nil {
		e.indexes[collection] = newHNSWIndex()
		e.docs[collection] = make(map[string]Document)
	}
	return nil
}

// Store upserts a document into the collection, persisting to disk and
// updating the in-memory HNSW index.
func (e *VectorEngine) Store(_ context.Context, collection, id string, vector []float32, metadata map[string]string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.indexes[collection] == nil {
		e.indexes[collection] = newHNSWIndex()
		e.docs[collection] = make(map[string]Document)
	}

	doc := Document{ID: id, Vector: vector, Metadata: metadata}
	data, err := encodeDoc(doc)
	if err != nil {
		return fmt.Errorf("encode document %q: %w", id, err)
	}

	if err := e.db.Update(func(txn *badger.Txn) error {
		return txn.Set(docKey(collection, id), data)
	}); err != nil {
		return fmt.Errorf("persist document %q in %q: %w", id, collection, err)
	}

	e.docs[collection][id] = doc
	e.indexes[collection].Insert(id, vector)
	return nil
}

// Search returns up to limit results ranked by cosine similarity (highest first).
// No minimum score threshold is applied; the caller decides relevance from Score.
// To compensate for HNSW beam-search recall at small ef, we request limit*3
// candidates and trim to limit after scoring.
func (e *VectorEngine) Search(_ context.Context, collection string, query []float32, limit int) ([]Result, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	idx := e.indexes[collection]
	if idx == nil || idx.Size() == 0 {
		return nil, nil
	}

	// Over-fetch to improve recall: HNSW may miss close neighbours when ef is
	// small relative to the collection size.
	fetch := max(limit*3, hnswEfSearch)

	candidates := idx.Search(query, fetch)
	results := make([]Result, 0, min(len(candidates), limit))
	docs := e.docs[collection]

	for _, c := range candidates {
		doc, ok := docs[c.id]
		if !ok {
			continue
		}
		results = append(results, Result{
			ID:       c.id,
			Score:    1 - c.dist, // cosine distance → similarity
			Metadata: doc.Metadata,
		})
		if len(results) == limit {
			break
		}
	}
	return results, nil
}

// DeleteByFilter removes all documents in a collection whose metadata matches
// every key/value pair in filter.
func (e *VectorEngine) DeleteByFilter(_ context.Context, collection string, filter map[string]string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	docs := e.docs[collection]
	if docs == nil {
		return nil
	}

	var toDelete []string
	for id, doc := range docs {
		if matchesFilter(doc.Metadata, filter) {
			toDelete = append(toDelete, id)
		}
	}
	if len(toDelete) == 0 {
		return nil
	}

	wb := e.db.NewWriteBatch()
	for _, id := range toDelete {
		if err := wb.Delete(docKey(collection, id)); err != nil {
			wb.Cancel()
			return fmt.Errorf("delete %q from %q: %w", id, collection, err)
		}
		delete(docs, id)
		e.indexes[collection].Delete(id)
	}
	if err := wb.Flush(); err != nil {
		return fmt.Errorf("flush delete batch for %q: %w", collection, err)
	}

	if len(toDelete) >= rebuildThreshold {
		e.indexes[collection].Rebuild(docs)
	}
	return nil
}

func matchesFilter(metadata, filter map[string]string) bool {
	for k, v := range filter {
		if metadata[k] != v {
			return false
		}
	}
	return true
}

// gcLoop runs BadgerDB value-log GC every gcInterval to reclaim disk space.
func (e *VectorEngine) gcLoop() {
	t := time.NewTicker(gcInterval)
	defer t.Stop()
	for {
		select {
		case <-e.gcStop:
			return
		case <-t.C:
			_ = e.db.RunValueLogGC(0.5)
		}
	}
}
