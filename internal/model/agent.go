package model

import "time"

const (
	AgentStatusIdle    = "idle"
	AgentStatusWorking = "working"
	AgentStatusDone    = "done"
	AgentStatusFailed  = "failed"
)

// Agent represents an AI specialist spawned within a Session.
// Agents are created automatically when a swarm is assembled and are
// always tied to exactly one Session.
type Agent struct {
	ID        uint      `gorm:"primaryKey"                       json:"id"`
	SessionID string    `gorm:"type:varchar(36);index;not null"  json:"session_id"`
	Name      string    `gorm:"type:varchar(150)"                json:"name"`
	Role      string    `gorm:"type:varchar(100)"                json:"role"`
	Status    string    `gorm:"type:varchar(20);default:'idle'"  json:"status"`
	CreatedAt time.Time `                                        json:"created_at"`
	UpdatedAt time.Time `                                        json:"updated_at"`
}

func (Agent) TableName() string { return "agents" }
