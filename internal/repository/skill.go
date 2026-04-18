package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

var ErrSkillNotFound = errors.New("skill não encontrada")

// SkillRepository define as operações de banco de dados para Skills.
type SkillRepository interface {
	Create(s *model.Skill) error
	FindByID(id uint) (*model.Skill, error)
	FindAll() ([]model.Skill, error)
	Update(s *model.Skill) error
	Delete(id uint) error

	// Associações com Colmeia
	AttachToColmeia(skillID uint, colmeiaID string) error
	DetachFromColmeia(skillID uint, colmeiaID string) error
	FindByColmeia(colmeiaID string) ([]model.Skill, error)

	// Associações com AgenteColmeia
	AttachToAgente(skillID uint, agenteID uint) error
	DetachFromAgente(skillID uint, agenteID uint) error
	FindByAgente(agenteID uint) ([]model.Skill, error)
}

type skillRepository struct{ db *gorm.DB }

// NewSkillRepository cria um novo SkillRepository.
func NewSkillRepository(db *gorm.DB) SkillRepository {
	return &skillRepository{db: db}
}

func (r *skillRepository) Create(s *model.Skill) error {
	return r.db.Create(s).Error
}

func (r *skillRepository) FindByID(id uint) (*model.Skill, error) {
	var s model.Skill
	if err := r.db.First(&s, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *skillRepository) FindAll() ([]model.Skill, error) {
	var skills []model.Skill
	if err := r.db.Order("id ASC").Find(&skills).Error; err != nil {
		return nil, err
	}
	return skills, nil
}

func (r *skillRepository) Update(s *model.Skill) error {
	return r.db.Save(s).Error
}

func (r *skillRepository) Delete(id uint) error {
	return r.db.Delete(&model.Skill{}, id).Error
}

func (r *skillRepository) AttachToColmeia(skillID uint, colmeiaID string) error {
	skill, err := r.FindByID(skillID)
	if err != nil {
		return err
	}
	colmeia := &model.Colmeia{ID: colmeiaID}
	return r.db.Model(colmeia).Association("Skills").Append(skill)
}

func (r *skillRepository) DetachFromColmeia(skillID uint, colmeiaID string) error {
	skill, err := r.FindByID(skillID)
	if err != nil {
		return err
	}
	colmeia := &model.Colmeia{ID: colmeiaID}
	return r.db.Model(colmeia).Association("Skills").Delete(skill)
}

func (r *skillRepository) FindByColmeia(colmeiaID string) ([]model.Skill, error) {
	var skills []model.Skill
	colmeia := &model.Colmeia{ID: colmeiaID}
	if err := r.db.Model(colmeia).Association("Skills").Find(&skills); err != nil {
		return nil, err
	}
	return skills, nil
}

func (r *skillRepository) AttachToAgente(skillID uint, agenteID uint) error {
	skill, err := r.FindByID(skillID)
	if err != nil {
		return err
	}
	agente := &model.AgenteColmeia{ID: agenteID}
	return r.db.Model(agente).Association("Skills").Append(skill)
}

func (r *skillRepository) DetachFromAgente(skillID uint, agenteID uint) error {
	skill, err := r.FindByID(skillID)
	if err != nil {
		return err
	}
	agente := &model.AgenteColmeia{ID: agenteID}
	return r.db.Model(agente).Association("Skills").Delete(skill)
}

func (r *skillRepository) FindByAgente(agenteID uint) ([]model.Skill, error) {
	var skills []model.Skill
	agente := &model.AgenteColmeia{ID: agenteID}
	if err := r.db.Model(agente).Association("Skills").Find(&skills); err != nil {
		return nil, err
	}
	return skills, nil
}
