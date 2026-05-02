package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	MCPTransportStdio = "stdio"
	MCPTransportSSE   = "sse"
	MCPTransportHTTP  = "http" // Streamable HTTP — MCP spec 2025-03-26 (Context7, etc.)
)

// MCPServer stores the connection configuration for an external MCP server.
// Each MCPServer belongs to exactly one Colmeia (one-to-many).
type MCPServer struct {
	ID        string   `gorm:"primaryKey;type:varchar(36)"                                    json:"id"`
	ColmeiaID string   `gorm:"type:varchar(36);index;not null;uniqueIndex:idx_mcp_col_name"   json:"colmeia_id"`
	Name      string   `gorm:"type:varchar(150);not null;uniqueIndex:idx_mcp_col_name"        json:"name"`
	Transport string   `gorm:"type:varchar(10);not null"                                      json:"transport"` // stdio | sse
	// Command holds the argv for stdio transport: ["sbx","exec","npx","-y","@mcp/server-sqlite"].
	// Stored as a JSON array in the database via GORM's json serializer.
	Command   []string `gorm:"type:text;serializer:json"  json:"command,omitempty"`
	// URL is the base URL of the remote SSE server.
	URL       string   `gorm:"type:varchar(500)"          json:"url,omitempty"`
	// EnvVars stores additional environment variables as a JSON object {"KEY":"VALUE"}.
	EnvVars   string   `gorm:"type:text;default:'{}'"     json:"env_vars,omitempty"`
	Active    bool     `gorm:"not null;default:true"      json:"active"`
	CreatedAt time.Time `                                  json:"created_at"`
	UpdatedAt time.Time `                                  json:"updated_at"`
}

func (m *MCPServer) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = newUUID()
	}
	return nil
}

func (MCPServer) TableName() string { return "mcp_servers" }

// GetEnvVars deserialises the EnvVars JSON field into a map.
func (m *MCPServer) GetEnvVars() map[string]string {
	out := make(map[string]string)
	if m.EnvVars == "" || m.EnvVars == "null" || m.EnvVars == "{}" {
		return out
	}
	_ = json.Unmarshal([]byte(m.EnvVars), &out)
	return out
}

// SetEnvVars serialises env vars into the EnvVars JSON field.
func (m *MCPServer) SetEnvVars(vars map[string]string) error {
	if vars == nil {
		vars = map[string]string{}
	}
	b, err := json.Marshal(vars)
	if err != nil {
		return err
	}
	m.EnvVars = string(b)
	return nil
}

// EnvSlice converts GetEnvVars into ["KEY=VALUE", ...] format for exec.Cmd.Env.
func (m *MCPServer) EnvSlice() []string {
	vars := m.GetEnvVars()
	out := make([]string, 0, len(vars))
	for k, v := range vars {
		out = append(out, k+"="+v)
	}
	return out
}
