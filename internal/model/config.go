package model

type AppConfig struct {
	BaseModel
	Provider   string `gorm:"column:provider;type:varchar(20);default:'openai'" json:"provider"`
	Language   string `gorm:"column:language;type:varchar(10)"                  json:"language"`
	Model      string `gorm:"column:model;type:varchar(35)"                     json:"model"`
	SwarmName  string `gorm:"column:swarm_name;type:varchar(150)"               json:"swarm_name"`
	MaxNectar  int    `gorm:"column:max_nectar"                                 json:"max_nectar"`
	MaxAgents  int    `gorm:"column:max_agents"                                 json:"max_agents"`
	Supervised bool   `gorm:"column:supervised"                                 json:"supervised"`
	Isolated   bool   `gorm:"column:isolated"                                   json:"isolated"`
}

func (AppConfig) TableName() string {
	return "app_configs"
}
