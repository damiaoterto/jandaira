package model

import (
	"time"

	"gorm.io/gorm"
)

// Document tracks a file uploaded to a session or colmeia workspace.
// Metadata here mirrors what is stored in Qdrant so chunks can be deleted
// without a separate Qdrant search.
type Document struct {
	ID            string    `gorm:"primaryKey;type:varchar(36)"  json:"id"`
	Filename      string    `gorm:"type:varchar(500)"            json:"filename"`
	WorkspacePath string    `gorm:"type:varchar(1000)"           json:"workspace_path"`
	Collection    string    `gorm:"type:varchar(255)"            json:"collection"`
	ScopeKey      string    `gorm:"type:varchar(50)"             json:"scope_key"`
	ScopeVal      string    `gorm:"type:varchar(36)"             json:"scope_val"`
	Chunks        int       `gorm:"type:integer;default:0"       json:"chunks"`
	CreatedAt     time.Time `                                    json:"created_at"`
	UpdatedAt     time.Time `                                    json:"updated_at"`
}

func (d *Document) BeforeCreate(_ *gorm.DB) error {
	if d.ID == "" {
		d.ID = newUUID()
	}
	return nil
}

func (Document) TableName() string { return "documents" }
