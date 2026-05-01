package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	MCPTransportStdio = "stdio"
	MCPTransportSSE   = "sse"
)

// MCPServer stores the connection configuration for an external MCP server.
// It has a many-to-many relationship with Colmeia: one colmeia can use many
// MCP servers, and one MCP server can be shared across many colmeias.
type MCPServer struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)"           json:"id"`
	Name      string    `gorm:"type:varchar(150);not null;uniqueIndex" json:"name"`
	Transport string    `gorm:"type:varchar(10);not null"             json:"transport"` // stdio | sse
	// Command is the shell command used for stdio transport, e.g. "npx -y @mcp/server-postgres".
	Command   string    `gorm:"type:text"                             json:"command,omitempty"`
	// URL is the base URL of the remote SSE server.
	URL       string    `gorm:"type:varchar(500)"                     json:"url,omitempty"`
	// EnvVars stores additional environment variables as a JSON object {"KEY":"VALUE"}.
	EnvVars   string    `gorm:"type:text;default:'{}'"                json:"env_vars,omitempty"`
	Active    bool      `gorm:"not null;default:true"                 json:"active"`
	CreatedAt time.Time `                                              json:"created_at"`
	UpdatedAt time.Time `                                              json:"updated_at"`

	Colmeias []Colmeia `gorm:"many2many:colmeia_mcp_servers;" json:"colmeias,omitempty"`
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

// CommandTokens splits the Command string into tokens for exec.Command.
// Simple space-split — sufficient for standard MCP server commands.
func (m *MCPServer) CommandTokens() []string {
	if m.Command == "" {
		return nil
	}
	var tokens []string
	current := ""
	inQuote := false
	for _, r := range m.Command {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
		default:
			current += string(r)
		}
	}
	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
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
