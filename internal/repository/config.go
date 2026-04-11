package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

// ErrNotFound is returned when no configuration record exists in the database.
var ErrNotFound = errors.New("configuração não encontrada")

// ConfigRepository defines the database operations for AppConfig.
type ConfigRepository interface {
	Find() (*model.AppConfig, error)
	Create(cfg *model.AppConfig) error
	Update(cfg *model.AppConfig) error
}

type configRepository struct {
	db *gorm.DB
}

// NewConfigRepository creates a new ConfigRepository backed by the given GORM database.
func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) Find() (*model.AppConfig, error) {
	var cfg model.AppConfig
	if err := r.db.First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *configRepository) Create(cfg *model.AppConfig) error {
	return r.db.Create(cfg).Error
}

func (r *configRepository) Update(cfg *model.AppConfig) error {
	return r.db.Save(cfg).Error
}
