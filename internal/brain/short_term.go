package brain

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryEntry holds a single message alongside its TTL metadata.
type MemoryEntry struct {
	ID        string
	Message   Message
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (e MemoryEntry) expired() bool {
	return time.Now().After(e.ExpiresAt)
}

// ShortTermMemory is a TTL-aware, auto-compacting conversation buffer.
//
// Design:
//   - Each message carries an expiry time (TTL from insertion).
//   - When the buffer reaches maxEntries, or when Flush is called, expired
//     and excess messages are summarised by the LLM and archived in Honeycomb
//     as a dense long-term record.
//   - Only the most recent, non-expired messages remain in RAM.
//
// This prevents context-window overflow when a Swarm runs for extended periods.
type ShortTermMemory struct {
	mu         sync.Mutex
	entries    []MemoryEntry
	maxEntries int
	ttl        time.Duration
	brain      Brain
	honeycomb  Honeycomb
	collection string
}

// NewShortTermMemory creates a short-term memory buffer.
//
//   - b:          LLM used for summarisation during compaction.
//   - h:          Vector DB used to archive compacted summaries.
//   - collection: Honeycomb collection name to store archives in.
//   - maxEntries: maximum active messages before forced compaction.
//   - ttl:        lifetime of each entry; expired entries are compacted first.
func NewShortTermMemory(b Brain, h Honeycomb, collection string, maxEntries int, ttl time.Duration) *ShortTermMemory {
	return &ShortTermMemory{
		maxEntries: maxEntries,
		ttl:        ttl,
		brain:      b,
		honeycomb:  h,
		collection: collection,
	}
}

// Append adds a message to the buffer.
// Expired entries are purged first; if the buffer is still full, a compaction
// cycle runs before the new message is appended.
func (m *ShortTermMemory) Append(ctx context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.purgeExpired()
	if len(m.entries) >= m.maxEntries {
		_ = m.compact(ctx)
	}

	m.entries = append(m.entries, MemoryEntry{
		ID:        fmt.Sprintf("stm-%d", time.Now().UnixNano()),
		Message:   msg,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.ttl),
	})
	return nil
}

// Messages returns all active (non-expired) messages in insertion order.
func (m *ShortTermMemory) Messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.purgeExpired()
	msgs := make([]Message, 0, len(m.entries))
	for _, e := range m.entries {
		msgs = append(msgs, e.Message)
	}
	return msgs
}

// Flush archives any remaining entries to long-term memory and clears the buffer.
// Call this at session end to ensure nothing is lost.
func (m *ShortTermMemory) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.entries) == 0 {
		return nil
	}
	return m.compact(ctx)
}

// Size returns the count of active (non-expired) entries.
func (m *ShortTermMemory) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.purgeExpired()
	return len(m.entries)
}

// purgeExpired drops entries past their TTL. Must be called with mu held.
func (m *ShortTermMemory) purgeExpired() {
	active := m.entries[:0]
	for _, e := range m.entries {
		if !e.expired() {
			active = append(active, e)
		}
	}
	m.entries = active
}

// compact summarises current entries via the LLM, archives the result in
// Honeycomb, then empties the buffer. Must be called with mu held.
func (m *ShortTermMemory) compact(ctx context.Context) error {
	if len(m.entries) == 0 {
		return nil
	}

	// Build a readable transcript.
	var sb strings.Builder
	for _, e := range m.entries {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", e.Message.Role, e.Message.Content))
	}
	transcript := sb.String()

	// Summarise with the LLM; fall back to the raw transcript on error.
	summary := transcript
	if m.brain != nil {
		summaryMsgs := []Message{
			{
				Role:    RoleSystem,
				Content: "You are a memory archiver. Summarize the following conversation transcript into a single dense paragraph that preserves all key decisions, findings, and technical details. Output only the summary — no preamble, no meta-commentary.",
			},
			{Role: RoleUser, Content: transcript},
		}
		if s, _, _, err := m.brain.Chat(ctx, summaryMsgs, nil); err == nil {
			summary = s
		}
	}
	if len(summary) > 4000 {
		summary = summary[:4000] + "...[truncated]"
	}

	// Embed and archive in Honeycomb.
	if m.honeycomb != nil && m.brain != nil {
		if vector, err := m.brain.Embed(ctx, summary); err == nil {
			docID := fmt.Sprintf("stm-archive-%d", time.Now().UnixNano())
			_ = m.honeycomb.Store(ctx, m.collection, docID, vector, map[string]string{
				"type":        "short_term_archive",
				"content":     summary,
				"entry_count": fmt.Sprintf("%d", len(m.entries)),
				"archived_at": time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	m.entries = m.entries[:0]
	return nil
}
