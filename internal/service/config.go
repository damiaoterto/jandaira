package service

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
)

// ErrNotConfigured is returned when the application has not been set up yet.
var ErrNotConfigured = errors.New("aplicação não configurada")

// ConfigService defines the business logic for application configuration.
type ConfigService interface {
	Load() (*model.AppConfig, error)
	Save(cfg *model.AppConfig) error
	IsConfigured() (bool, error)
}

type configService struct {
	repo repository.ConfigRepository
}

// NewConfigService creates a new ConfigService backed by the given repository.
func NewConfigService(repo repository.ConfigRepository) ConfigService {
	return &configService{repo: repo}
}

// Load retrieves the current configuration. Returns ErrNotConfigured when no
// configuration has been saved yet.
func (s *configService) Load() (*model.AppConfig, error) {
	cfg, err := s.repo.Find()
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotConfigured
		}
		return nil, err
	}
	return cfg, nil
}

// Save persists cfg. Creates a new record on the first call; updates on subsequent calls.
func (s *configService) Save(cfg *model.AppConfig) error {
	existing, err := s.repo.Find()
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return s.repo.Create(cfg)
		}
		return err
	}
	cfg.ID = existing.ID
	return s.repo.Update(cfg)
}

// IsConfigured reports whether the application has been configured.
func (s *configService) IsConfigured() (bool, error) {
	_, err := s.repo.Find()
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
