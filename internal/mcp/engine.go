package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Tool represents a capability advertised by an MCP server.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Resource represents a data resource advertised by an MCP server.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ToolResult holds the output of a tools/call invocation.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a single piece of content returned by a tool.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Engine manages a single connection to an MCP server, handling the JSON-RPC
// lifecycle: initialize handshake, tool discovery, and tool invocation.
type Engine struct {
	transport Transport

	idCounter int64
	mu        sync.Mutex
	pending   map[int64]chan rpcResponse
}

// NewEngine creates an Engine backed by the given transport.
func NewEngine(t Transport) *Engine {
	return &Engine{
		transport: t,
		pending:   make(map[int64]chan rpcResponse),
	}
}

// Start connects the transport and runs the MCP initialization handshake.
// It must be called before any other method.
func (e *Engine) Start(ctx context.Context) error {
	if err := e.transport.Start(ctx); err != nil {
		return fmt.Errorf("mcp engine: transport start: %w", err)
	}

	recvCh, err := e.transport.Receive()
	if err != nil {
		return fmt.Errorf("mcp engine: receive channel: %w", err)
	}

	go e.receiveLoop(recvCh)

	if err := e.initialize(ctx); err != nil {
		return fmt.Errorf("mcp engine: initialize: %w", err)
	}
	return nil
}

// ListTools returns the tools available on the connected MCP server.
func (e *Engine) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := e.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcp engine: tools/list: %w", err)
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp engine: decode tools/list: %w", err)
	}
	return result.Tools, nil
}

// ListResources returns the resources available on the connected MCP server.
func (e *Engine) ListResources(ctx context.Context) ([]Resource, error) {
	raw, err := e.call(ctx, "resources/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcp engine: resources/list: %w", err)
	}

	var result struct {
		Resources []Resource `json:"resources"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp engine: decode resources/list: %w", err)
	}
	return result.Resources, nil
}

// CallTool invokes a named tool on the MCP server with the provided arguments.
func (e *Engine) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	raw, err := e.call(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("mcp engine: tools/call %q: %w", name, err)
	}

	var result ToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp engine: decode tools/call result: %w", err)
	}
	return &result, nil
}

// Close terminates the connection to the MCP server.
func (e *Engine) Close() error {
	return e.transport.Close()
}

// initialize performs the MCP handshake: send initialize request then the
// notifications/initialized notification.
func (e *Engine) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]interface{}{
			"name":    "jandaira",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{},
	}

	if _, err := e.call(ctx, "initialize", params); err != nil {
		return err
	}

	// Fire-and-forget notification — no response expected.
	_ = e.notify(ctx, "notifications/initialized", nil)
	return nil
}

// call sends a JSON-RPC request and waits for the matching response.
func (e *Engine) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&e.idCounter, 1)

	req := rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respCh := make(chan rpcResponse, 1)
	e.mu.Lock()
	e.pending[id] = respCh
	e.mu.Unlock()

	if err := e.transport.Send(ctx, data); err != nil {
		e.mu.Lock()
		delete(e.pending, id)
		e.mu.Unlock()
		return nil, fmt.Errorf("send: %w", err)
	}

	select {
	case <-ctx.Done():
		e.mu.Lock()
		delete(e.pending, id)
		e.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("server error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// notify sends a JSON-RPC notification (no ID, no response expected).
func (e *Engine) notify(ctx context.Context, method string, params interface{}) error {
	notif := rpcNotification{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	return e.transport.Send(ctx, data)
}

// receiveLoop reads raw messages from the transport and dispatches each one to
// the waiting call goroutine via its pending channel.
func (e *Engine) receiveLoop(ch <-chan []byte) {
	for data := range ch {
		var resp rpcResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue // skip malformed frames
		}
		if resp.ID == 0 {
			continue // notification — ignored
		}

		e.mu.Lock()
		waiter, ok := e.pending[resp.ID]
		if ok {
			delete(e.pending, resp.ID)
		}
		e.mu.Unlock()

		if ok {
			waiter <- resp
		}
	}
}
