package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	HistoricoStatusActive    = "active"
	HistoricoStatusCompleted = "completed"
	HistoricoStatusFailed    = "failed"
)

// Colmeia representa uma colmeia persistente de agentes AI.
// Uma colmeia pode ter agentes pré-definidos pelo usuário (QueenManaged=false)
// ou deixar a rainha criar os agentes automaticamente (QueenManaged=true).
type Colmeia struct {
	ID           string              `gorm:"primaryKey;type:varchar(36)"                          json:"id"`
	Name         string              `gorm:"type:varchar(150);not null"                           json:"name"`
	Description  string              `gorm:"type:text"                                            json:"description,omitempty"`
	QueenManaged bool                `gorm:"not null"                                             json:"queen_managed"`
	CreatedAt    time.Time           `                                                            json:"created_at"`
	UpdatedAt    time.Time           `                                                            json:"updated_at"`
	Agentes          []AgenteColmeia     `gorm:"foreignKey:ColmeiaID;constraint:OnDelete:CASCADE"     json:"agentes,omitempty"`
	Historico        []HistoricoDespacho `gorm:"foreignKey:ColmeiaID;constraint:OnDelete:CASCADE"     json:"historico,omitempty"`
	OutboundWebhooks []OutboundWebhook   `gorm:"foreignKey:ColmeiaID;constraint:OnDelete:CASCADE"     json:"outbound_webhooks,omitempty"`
	Skills           []Skill             `gorm:"many2many:colmeia_skills;"                            json:"skills,omitempty"`
	MCPServers       []MCPServer         `gorm:"many2many:colmeia_mcp_servers;"                       json:"mcp_servers,omitempty"`
}

func (c *Colmeia) BeforeCreate(_ *gorm.DB) error {
	if c.ID == "" {
		c.ID = newUUID()
	}
	return nil
}

func (Colmeia) TableName() string { return "colmeias" }

// AgenteColmeia representa um agente persistente dentro de uma Colmeia.
// O usuário pode definir o nome, prompt do sistema e ferramentas permitidas.
type AgenteColmeia struct {
	ID           uint      `gorm:"primaryKey"                       json:"id"`
	ColmeiaID    string    `gorm:"type:varchar(36);index;not null"  json:"colmeia_id"`
	Name         string    `gorm:"type:varchar(150)"                json:"name"`
	SystemPrompt string    `gorm:"type:text"                        json:"system_prompt"`
	AllowedTools string    `gorm:"type:text;default:'[]'"           json:"allowed_tools"` // JSON array
	CreatedAt    time.Time `                                         json:"created_at"`
	UpdatedAt    time.Time `                                         json:"updated_at"`
	Skills       []Skill   `gorm:"many2many:agente_colmeia_skills;" json:"skills,omitempty"`
}

func (AgenteColmeia) TableName() string { return "agentes_colmeia" }

// GetAllowedTools retorna a lista de ferramentas permitidas para o agente.
func (a *AgenteColmeia) GetAllowedTools() []string {
	var tools []string
	if a.AllowedTools == "" || a.AllowedTools == "null" {
		return tools
	}
	_ = json.Unmarshal([]byte(a.AllowedTools), &tools)
	return tools
}

// SetAllowedTools serializa a lista de ferramentas permitidas para armazenamento.
func (a *AgenteColmeia) SetAllowedTools(tools []string) error {
	if tools == nil {
		tools = []string{}
	}
	b, err := json.Marshal(tools)
	if err != nil {
		return err
	}
	a.AllowedTools = string(b)
	return nil
}

// HistoricoDespacho registra cada envio de objetivo para a colmeia,
// permitindo que conversas anteriores sejam usadas como contexto.
type HistoricoDespacho struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)"       json:"id"`
	ColmeiaID string    `gorm:"type:varchar(36);index;not null"   json:"colmeia_id"`
	Goal      string    `gorm:"type:text"                         json:"goal"`
	Result    string    `gorm:"type:text"                         json:"result,omitempty"`
	Status    string    `gorm:"type:varchar(20);default:'active'" json:"status"`
	CreatedAt time.Time `                                          json:"created_at"`
	UpdatedAt time.Time `                                          json:"updated_at"`
}

func (h *HistoricoDespacho) BeforeCreate(_ *gorm.DB) error {
	if h.ID == "" {
		h.ID = newUUID()
	}
	if h.Status == "" {
		h.Status = HistoricoStatusActive
	}
	return nil
}

func (HistoricoDespacho) TableName() string { return "historico_despachos" }
