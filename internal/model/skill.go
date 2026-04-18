package model

import (
	"encoding/json"
	"time"
)

// Skill é uma capacidade reutilizável que pode ser atribuída a colmeias ou agentes.
// Encapsula instruções específicas e ferramentas necessárias para um domínio de atuação.
type Skill struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"              json:"id"`
	Name         string    `gorm:"type:varchar(150);not null;uniqueIndex" json:"name"`
	Description  string    `gorm:"type:text"                             json:"description,omitempty"`
	Instructions string    `gorm:"type:text"                             json:"instructions,omitempty"`
	AllowedTools string    `gorm:"type:text;default:'[]'"                json:"allowed_tools"`
	CreatedAt    time.Time `                                              json:"created_at"`
	UpdatedAt    time.Time `                                              json:"updated_at"`
}

func (Skill) TableName() string { return "skills" }

// GetAllowedTools retorna a lista de ferramentas permitidas para a skill.
func (s *Skill) GetAllowedTools() []string {
	var tools []string
	if s.AllowedTools == "" || s.AllowedTools == "null" {
		return tools
	}
	_ = json.Unmarshal([]byte(s.AllowedTools), &tools)
	return tools
}

// SetAllowedTools serializa a lista de ferramentas para armazenamento.
func (s *Skill) SetAllowedTools(tools []string) error {
	if tools == nil {
		tools = []string{}
	}
	b, err := json.Marshal(tools)
	if err != nil {
		return err
	}
	s.AllowedTools = string(b)
	return nil
}
