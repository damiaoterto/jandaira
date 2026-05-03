package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// MCPToolAdapter wraps a single MCP tool as a tool.Tool so it can be equipped
// on the Queen's tool registry and called by specialists during a workflow.
type MCPToolAdapter struct {
	engine     *Engine
	mcpTool    Tool
	serverName string
}

// NewMCPToolAdapter wraps the given MCP tool using the supplied engine.
// serverName is used to namespace the tool name: "{serverName}_{toolName}".
func NewMCPToolAdapter(engine *Engine, t Tool, serverName string) *MCPToolAdapter {
	return &MCPToolAdapter{engine: engine, mcpTool: t, serverName: serverName}
}

// Name returns the qualified tool name in the form "{serverName}_{toolName}".
// Both parts are fully sanitized (hyphens → underscores) so LLMs can reproduce
// the name reliably in structured outputs.
func (a *MCPToolAdapter) Name() string {
	return sanitizeName(a.serverName) + "_" + sanitizeName(a.mcpTool.Name)
}

// Description exposes the MCP tool description with a server prefix so
// specialists know which integration the tool belongs to.
func (a *MCPToolAdapter) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", a.serverName, a.mcpTool.Description)
}

// Parameters returns the MCP tool's JSON Schema input definition verbatim.
// If the server returned no schema, an empty object schema is used as fallback.
func (a *MCPToolAdapter) Parameters() map[string]interface{} {
	if a.mcpTool.InputSchema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		}
	}
	return a.mcpTool.InputSchema
}

// Execute deserialises the JSON arguments, calls the MCP server tool, and
// returns the concatenated text content of the result.
func (a *MCPToolAdapter) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args map[string]any
	if argsJSON != "" && argsJSON != "{}" && argsJSON != "null" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("mcp adapter %q: invalid args JSON: %w", a.Name(), err)
		}
	}

	log.Printf("[mcp-adapter] calling tool %q args=%s", a.Name(), argsJSON)
	result, err := a.engine.CallTool(ctx, a.mcpTool.Name, args)
	if err != nil {
		log.Printf("[mcp-adapter] tool %q failed: %v", a.Name(), err)
		return "", fmt.Errorf("mcp adapter %q: call failed: %w", a.Name(), err)
	}

	text := extractText(result.Content)
	log.Printf("[mcp-adapter] tool %q returned %d bytes (isError=%v)", a.Name(), len(text), result.IsError)

	if result.IsError {
		return "", fmt.Errorf("mcp tool %q returned error: %s", a.Name(), text)
	}
	return text, nil
}

// extractText joins the text fields of all text-type content blocks.
func extractText(blocks []ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// sanitizeName replaces any character that is not a letter or digit with an
// underscore so the resulting string is safe as part of an LLM tool name.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
