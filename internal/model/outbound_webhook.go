package model

// OutboundWebhook is an HTTP endpoint that receives mission results when a
// Colmeia completes a dispatch. BodyTemplate uses Go text/template syntax;
// the template dot (.) exposes: result, goal, colmeia_id, historico_id, payload.
type OutboundWebhook struct {
	BaseModel
	ColmeiaID    string `gorm:"type:varchar(36);index;not null"          json:"colmeia_id"`
	Name         string `gorm:"type:varchar(150);not null"               json:"name"`
	URL          string `gorm:"type:varchar(2048);not null"              json:"url"`
	Method       string `gorm:"type:varchar(10);not null;default:'POST'" json:"method"`
	Headers      string `gorm:"type:text;default:'{}'"                   json:"headers"`
	BodyTemplate string `gorm:"type:text;not null"                       json:"body_template"`
	Secret       string `gorm:"type:varchar(256)"                        json:"secret,omitempty"`
	Active       bool   `gorm:"not null;default:true"                    json:"active"`
}

func (OutboundWebhook) TableName() string { return "outbound_webhooks" }
