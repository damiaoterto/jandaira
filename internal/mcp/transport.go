package mcp

import "context"

// Transport abstracts the communication channel to an MCP server.
// Implementations handle Stdio (subprocess) and SSE (remote HTTP) connections.
type Transport interface {
	// Start initialises the underlying connection or process.
	Start(ctx context.Context) error

	// Send writes a JSON-RPC message to the server.
	Send(ctx context.Context, msg []byte) error

	// Receive returns a channel that delivers raw JSON-RPC messages from the server.
	// The channel is closed when the connection terminates.
	Receive() (<-chan []byte, error)

	// Close terminates the connection and releases resources.
	Close() error
}
