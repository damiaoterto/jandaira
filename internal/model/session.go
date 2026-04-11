package model

import (
	"crypto/rand"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	SessionStatusActive    = "active"
	SessionStatusCompleted = "completed"
	SessionStatusFailed    = "failed"
)

// Session represents a single execution context.
// A session is created for each user goal and holds all agents spawned for it.
// Deleting a session cascades to its agents.
type Session struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)"      json:"id"`
	Name      string    `gorm:"type:varchar(150)"                json:"name,omitempty"`
	Goal      string    `gorm:"type:text"                        json:"goal"`
	Status    string    `gorm:"type:varchar(20);default:'active'" json:"status"`
	Result    string    `gorm:"type:text"                        json:"result,omitempty"`
	CreatedAt time.Time `                                        json:"created_at"`
	UpdatedAt time.Time `                                        json:"updated_at"`
	Agents    []Agent   `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"agents,omitempty"`
}

func (s *Session) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = newUUID()
	}
	if s.Status == "" {
		s.Status = SessionStatusActive
	}
	return nil
}

func (Session) TableName() string { return "sessions" }

// newUUID generates a random UUID v4.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
