package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

// ErrSessionNotFound is returned when no session with the given ID exists.
var ErrSessionNotFound = errors.New("sessão não encontrada")

// SessionRepository defines the database operations for Session.
type SessionRepository interface {
	Create(s *model.Session) error
	FindByID(id string) (*model.Session, error)
	FindByIDWithAgents(id string) (*model.Session, error)
	FindAll() ([]model.Session, error)
	Update(s *model.Session) error
	Delete(id string) error
}

type sessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new SessionRepository backed by the given GORM database.
func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(s *model.Session) error {
	return r.db.Create(s).Error
}

func (r *sessionRepository) FindByID(id string) (*model.Session, error) {
	var s model.Session
	if err := r.db.First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *sessionRepository) FindByIDWithAgents(id string) (*model.Session, error) {
	var s model.Session
	if err := r.db.Preload("Agents", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *sessionRepository) FindAll() ([]model.Session, error) {
	var sessions []model.Session
	if err := r.db.Select("id, name, goal, status, created_at, updated_at").
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *sessionRepository) Update(s *model.Session) error {
	return r.db.Save(s).Error
}

func (r *sessionRepository) Delete(id string) error {
	return r.db.Delete(&model.Session{}, "id = ?", id).Error
}
