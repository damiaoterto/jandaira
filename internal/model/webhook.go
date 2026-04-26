package model

// Webhook defines a public HTTP trigger that dispatches a goal to a Colmeia.
// The GoalTemplate field uses Go's text/template syntax; placeholders reference
// fields from the incoming JSON payload (e.g. {{.repository.name}}).
type Webhook struct {
	BaseModel
	Name         string `gorm:"type:varchar(150);not null"             json:"name"`
	Slug         string `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	ColmeiaID    string `gorm:"type:varchar(36);index;not null"        json:"colmeia_id"`
	Secret       string `gorm:"type:varchar(256)"                      json:"secret,omitempty"`
	Active       bool   `gorm:"not null;default:true"                  json:"active"`
	GoalTemplate string `gorm:"type:text;not null"                     json:"goal_template"`
}

func (Webhook) TableName() string { return "webhooks" }
