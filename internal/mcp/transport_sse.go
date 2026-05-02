package mcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SSETransport connects to a remote MCP server that uses the HTTP+SSE transport.
//
// Protocol:
//  1. Client opens GET {URL} → server streams SSE events.
//  2. First SSE event type "endpoint" carries the POST URL for sending messages.
//  3. Subsequent SSE events type "message" carry JSON-RPC responses.
//  4. Client sends JSON-RPC requests via POST to the endpoint URL.
//
// URL must be the exact SSE endpoint (e.g. "https://mcp.example.com/sse" or
// "https://mcp.context7.com/mcp"). No path is appended automatically.
type SSETransport struct {
	// BaseURL is the exact SSE endpoint URL.
	BaseURL string
	// Headers are extra HTTP headers sent on every request (e.g. Authorization).
	Headers map[string]string

	client      *http.Client
	postURL     string
	outChan     chan []byte
	sseCancel   context.CancelFunc
}

// NewSSETransport creates an SSETransport targeting the given base URL.
func NewSSETransport(baseURL string, headers map[string]string) *SSETransport {
	return &SSETransport{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Headers: headers,
		outChan: make(chan []byte, 64),
		client:  &http.Client{Timeout: 0}, // no global timeout for streaming
	}
}

// Start connects to the SSE endpoint and waits until the server announces the
// POST endpoint URL before returning.
func (t *SSETransport) Start(ctx context.Context) error {
	sseCtx, cancel := context.WithCancel(ctx)
	t.sseCancel = cancel

	endpointCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go t.sseLoop(sseCtx, endpointCh, errCh)

	// Block until we receive the POST endpoint or an error.
	select {
	case endpoint := <-endpointCh:
		if strings.HasPrefix(endpoint, "http") {
			t.postURL = endpoint
		} else {
			// Relative path — resolve against the SSE URL's origin.
			base, err := url.Parse(t.BaseURL)
			if err != nil {
				cancel()
				return fmt.Errorf("mcp sse: invalid base URL: %w", err)
			}
			ref, err := url.Parse(endpoint)
			if err != nil {
				cancel()
				return fmt.Errorf("mcp sse: invalid endpoint path %q: %w", endpoint, err)
			}
			t.postURL = base.ResolveReference(ref).String()
		}
		return nil
	case err := <-errCh:
		return fmt.Errorf("mcp sse: connect failed: %w", err)
	case <-time.After(30 * time.Second):
		cancel()
		return fmt.Errorf("mcp sse: timeout waiting for endpoint event")
	}
}

// Send posts a JSON-RPC message to the server endpoint.
func (t *SSETransport) Send(ctx context.Context, msg []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.postURL, bytes.NewReader(msg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("mcp sse: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mcp sse: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// Receive returns the channel that delivers JSON-RPC messages from SSE events.
func (t *SSETransport) Receive() (<-chan []byte, error) {
	return t.outChan, nil
}

// Close terminates the SSE connection.
func (t *SSETransport) Close() error {
	if t.sseCancel != nil {
		t.sseCancel()
	}
	return nil
}

// sseLoop opens the SSE endpoint and processes events.
func (t *SSETransport) sseLoop(ctx context.Context, endpointCh chan<- string, errCh chan<- error) {
	defer close(t.outChan)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.BaseURL, nil)
	if err != nil {
		errCh <- err
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		errCh <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errCh <- fmt.Errorf("mcp sse: GET %s returned %d", t.BaseURL, resp.StatusCode)
		return
	}

	endpointSent := false
	scanner := bufio.NewScanner(resp.Body)

	var eventType, dataLine string

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLine = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "":
			// End of SSE event block — process it.
			if !endpointSent && eventType == "endpoint" {
				endpointCh <- dataLine
				endpointSent = true
			} else if eventType == "message" && dataLine != "" {
				cp := []byte(dataLine)
				select {
				case t.outChan <- cp:
				case <-ctx.Done():
					return
				}
			}
			eventType = ""
			dataLine = ""
		}
	}
}
