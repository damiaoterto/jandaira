package service

import (
	"errors"
	"fmt"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
)

var ErrSkillNotFound = errors.New("skill não encontrada")

// SkillService define a lógica de negócio para gerenciamento de Skills.
type SkillService interface {
	// CRUD global
	CreateSkill(name, description, instructions string, allowedTools []string) (*model.Skill, error)
	GetSkill(id uint) (*model.Skill, error)
	ListSkills() ([]model.Skill, error)
	UpdateSkill(id uint, name, description, instructions string, allowedTools []string) (*model.Skill, error)
	DeleteSkill(id uint) error

	// Associações com Colmeia
	AttachSkillToColmeia(skillID uint, colmeiaID string) error
	DetachSkillFromColmeia(skillID uint, colmeiaID string) error
	ListColmeiaSkills(colmeiaID string) ([]model.Skill, error)

	// Associações com AgenteColmeia
	AttachSkillToAgente(skillID uint, agenteID uint) error
	DetachSkillFromAgente(skillID uint, agenteID uint) error
	ListAgenteSkills(agenteID uint) ([]model.Skill, error)
}

type skillService struct {
	skills repository.SkillRepository
}

// NewSkillService cria um novo SkillService.
func NewSkillService(skills repository.SkillRepository) SkillService {
	return &skillService{skills: skills}
}

func (s *skillService) CreateSkill(name, description, instructions string, allowedTools []string) (*model.Skill, error) {
	skill := &model.Skill{
		Name:         name,
		Description:  description,
		Instructions: instructions,
	}
	if err := skill.SetAllowedTools(allowedTools); err != nil {
		return nil, fmt.Errorf("erro ao serializar ferramentas: %w", err)
	}
	if err := s.skills.Create(skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *skillService) GetSkill(id uint) (*model.Skill, error) {
	skill, err := s.skills.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrSkillNotFound) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return skill, nil
}

func (s *skillService) ListSkills() ([]model.Skill, error) {
	return s.skills.FindAll()
}

func (s *skillService) UpdateSkill(id uint, name, description, instructions string, allowedTools []string) (*model.Skill, error) {
	skill, err := s.skills.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrSkillNotFound) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	skill.Name = name
	skill.Description = description
	skill.Instructions = instructions
	if err := skill.SetAllowedTools(allowedTools); err != nil {
		return nil, fmt.Errorf("erro ao serializar ferramentas: %w", err)
	}
	if err := s.skills.Update(skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *skillService) DeleteSkill(id uint) error {
	if _, err := s.skills.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrSkillNotFound) {
			return ErrSkillNotFound
		}
		return err
	}
	return s.skills.Delete(id)
}

func (s *skillService) AttachSkillToColmeia(skillID uint, colmeiaID string) error {
	if _, err := s.skills.FindByID(skillID); err != nil {
		if errors.Is(err, repository.ErrSkillNotFound) {
			return ErrSkillNotFound
		}
		return err
	}
	return s.skills.AttachToColmeia(skillID, colmeiaID)
}

func (s *skillService) DetachSkillFromColmeia(skillID uint, colmeiaID string) error {
	if _, err := s.skills.FindByID(skillID); err != nil {
		if errors.Is(err, repository.ErrSkillNotFound) {
			return ErrSkillNotFound
		}
		return err
	}
	return s.skills.DetachFromColmeia(skillID, colmeiaID)
}

func (s *skillService) ListColmeiaSkills(colmeiaID string) ([]model.Skill, error) {
	return s.skills.FindByColmeia(colmeiaID)
}

func (s *skillService) AttachSkillToAgente(skillID uint, agenteID uint) error {
	if _, err := s.skills.FindByID(skillID); err != nil {
		if errors.Is(err, repository.ErrSkillNotFound) {
			return ErrSkillNotFound
		}
		return err
	}
	return s.skills.AttachToAgente(skillID, agenteID)
}

func (s *skillService) DetachSkillFromAgente(skillID uint, agenteID uint) error {
	if _, err := s.skills.FindByID(skillID); err != nil {
		if errors.Is(err, repository.ErrSkillNotFound) {
			return ErrSkillNotFound
		}
		return err
	}
	return s.skills.DetachFromAgente(skillID, agenteID)
}

func (s *skillService) ListAgenteSkills(agenteID uint) ([]model.Skill, error) {
	return s.skills.FindByAgente(agenteID)
}
