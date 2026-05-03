package mcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// StreamableHTTPTransport implements the MCP 2025-03-26 "Streamable HTTP" transport.
//
// Protocol: single endpoint handles both sending and receiving.
//   - POST {URL} with JSON-RPC body → response is either:
//     (a) Content-Type: application/json  → direct JSON-RPC response
//     (b) Content-Type: text/event-stream → SSE stream of JSON-RPC events
//   - 202/204 responses (notifications) carry no body and are silently ignored.
//
// No persistent connection is required: each Send() is an independent POST.
// This is the transport used by modern MCP servers such as Context7.
type StreamableHTTPTransport struct {
	// URL is the exact MCP endpoint (e.g. "https://mcp.context7.com/mcp").
	URL string
	// Headers are extra HTTP headers sent on every request (e.g. Authorization).
	Headers map[string]string

	client  *http.Client
	outChan chan []byte
	mu      sync.Mutex
	closed  bool
}

// NewStreamableHTTPTransport creates a StreamableHTTPTransport for the given endpoint URL.
func NewStreamableHTTPTransport(rawURL string, headers map[string]string) *StreamableHTTPTransport {
	return &StreamableHTTPTransport{
		URL:     strings.TrimRight(rawURL, "/"),
		Headers: headers,
		outChan: make(chan []byte, 64),
		client:  &http.Client{Timeout: 0},
	}
}

// Start is a no-op for Streamable HTTP — no persistent connection is needed.
func (t *StreamableHTTPTransport) Start(_ context.Context) error {
	return nil
}

// Send POSTs the JSON-RPC message and asynchronously forwards any response
// (direct JSON or SSE events) to the receive channel.
func (t *StreamableHTTPTransport) Send(ctx context.Context, msg []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("mcp http: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}

	go func() {
		resp, err := t.client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		// Notification ack — no response body expected.
		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusAccepted {
			return
		}

		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "text/event-stream") {
			t.drainSSE(resp.Body)
		} else {
			data, _ := io.ReadAll(resp.Body)
			if len(data) > 0 {
				t.forward(data)
			}
		}
	}()
	return nil
}

// drainSSE reads SSE events from r and forwards every non-empty data line.
func (t *StreamableHTTPTransport) drainSSE(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			if trimmed := strings.TrimSpace(data); trimmed != "" {
				t.forward([]byte(trimmed))
			}
		}
	}
}

func (t *StreamableHTTPTransport) forward(data []byte) {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return
	}
	select {
	case t.outChan <- data:
	default:
	}
}

// Receive returns the channel that delivers JSON-RPC responses.
func (t *StreamableHTTPTransport) Receive() (<-chan []byte, error) {
	return t.outChan, nil
}

// Close marks the transport as closed and drains the channel.
func (t *StreamableHTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		close(t.outChan)
	}
	return nil
}
