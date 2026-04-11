package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

// ErrAgentNotFound is returned when no agent with the given ID exists.
var ErrAgentNotFound = errors.New("agente não encontrado")

// AgentRepository defines the database operations for Agent.
type AgentRepository interface {
	Create(a *model.Agent) error
	FindByID(id uint) (*model.Agent, error)
	FindBySessionID(sessionID string) ([]model.Agent, error)
	FindBySessionAndName(sessionID, name string) (*model.Agent, error)
	UpdateStatus(id uint, status string) error
	UpdateStatusByName(sessionID, name, status string) error
	UpdateAllInSession(sessionID, status string) error
	Delete(id uint) error
	DeleteBySessionID(sessionID string) error
}

type agentRepository struct {
	db *gorm.DB
}

// NewAgentRepository creates a new AgentRepository backed by the given GORM database.
func NewAgentRepository(db *gorm.DB) AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) Create(a *model.Agent) error {
	return r.db.Create(a).Error
}

func (r *agentRepository) FindByID(id uint) (*model.Agent, error) {
	var a model.Agent
	if err := r.db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *agentRepository) FindBySessionID(sessionID string) ([]model.Agent, error) {
	var agents []model.Agent
	if err := r.db.Where("session_id = ?", sessionID).Order("id ASC").Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}

func (r *agentRepository) FindBySessionAndName(sessionID, name string) (*model.Agent, error) {
	var a model.Agent
	if err := r.db.Where("session_id = ? AND name = ?", sessionID, name).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *agentRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&model.Agent{}).Where("id = ?", id).Update("status", status).Error
}

func (r *agentRepository) UpdateStatusByName(sessionID, name, status string) error {
	return r.db.Model(&model.Agent{}).
		Where("session_id = ? AND name = ?", sessionID, name).
		Update("status", status).Error
}

func (r *agentRepository) UpdateAllInSession(sessionID, status string) error {
	return r.db.Model(&model.Agent{}).
		Where("session_id = ?", sessionID).
		Update("status", status).Error
}

func (r *agentRepository) Delete(id uint) error {
	return r.db.Delete(&model.Agent{}, id).Error
}

func (r *agentRepository) DeleteBySessionID(sessionID string) error {
	return r.db.Where("session_id = ?", sessionID).Delete(&model.Agent{}).Error
}
