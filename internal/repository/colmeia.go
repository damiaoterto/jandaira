package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

var (
	ErrColmeiaNotFound       = errors.New("colmeia não encontrada")
	ErrAgenteColmeiaNotFound = errors.New("agente da colmeia não encontrado")
	ErrHistoricoNotFound     = errors.New("histórico de despacho não encontrado")
)

// ─── ColmeiaRepository ─────────────────────────────────────────────────────────

// ColmeiaRepository define as operações de banco de dados para Colmeia.
type ColmeiaRepository interface {
	Create(c *model.Colmeia) error
	FindByID(id string) (*model.Colmeia, error)
	FindByIDWithAgentes(id string) (*model.Colmeia, error)
	FindAll() ([]model.Colmeia, error)
	Update(c *model.Colmeia) error
	Delete(id string) error
}

type colmeiaRepository struct{ db *gorm.DB }

// NewColmeiaRepository cria um novo ColmeiaRepository.
func NewColmeiaRepository(db *gorm.DB) ColmeiaRepository {
	return &colmeiaRepository{db: db}
}

func (r *colmeiaRepository) Create(c *model.Colmeia) error {
	return r.db.Create(c).Error
}

func (r *colmeiaRepository) FindByID(id string) (*model.Colmeia, error) {
	var c model.Colmeia
	if err := r.db.First(&c, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *colmeiaRepository) FindByIDWithAgentes(id string) (*model.Colmeia, error) {
	var c model.Colmeia
	if err := r.db.
		Preload("Skills").
		Preload("Agentes", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("Agentes.Skills").
		First(&c, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *colmeiaRepository) FindAll() ([]model.Colmeia, error) {
	var colmeias []model.Colmeia
	if err := r.db.Select("id, name, description, queen_managed, created_at, updated_at").
		Order("created_at DESC").
		Find(&colmeias).Error; err != nil {
		return nil, err
	}
	return colmeias, nil
}

func (r *colmeiaRepository) Update(c *model.Colmeia) error {
	return r.db.Save(c).Error
}

func (r *colmeiaRepository) Delete(id string) error {
	return r.db.Delete(&model.Colmeia{}, "id = ?", id).Error
}

// ─── AgenteColmeiaRepository ───────────────────────────────────────────────────

// AgenteColmeiaRepository define as operações de banco de dados para AgenteColmeia.
type AgenteColmeiaRepository interface {
	Create(a *model.AgenteColmeia) error
	FindByID(id uint) (*model.AgenteColmeia, error)
	FindByColmeiaID(colmeiaID string) ([]model.AgenteColmeia, error)
	Update(a *model.AgenteColmeia) error
	Delete(id uint) error
	DeleteByColmeiaID(colmeiaID string) error
}

type agenteColmeiaRepository struct{ db *gorm.DB }

// NewAgenteColmeiaRepository cria um novo AgenteColmeiaRepository.
func NewAgenteColmeiaRepository(db *gorm.DB) AgenteColmeiaRepository {
	return &agenteColmeiaRepository{db: db}
}

func (r *agenteColmeiaRepository) Create(a *model.AgenteColmeia) error {
	return r.db.Create(a).Error
}

func (r *agenteColmeiaRepository) FindByID(id uint) (*model.AgenteColmeia, error) {
	var a model.AgenteColmeia
	if err := r.db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgenteColmeiaNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *agenteColmeiaRepository) FindByColmeiaID(colmeiaID string) ([]model.AgenteColmeia, error) {
	var agentes []model.AgenteColmeia
	if err := r.db.Where("colmeia_id = ?", colmeiaID).Order("id ASC").Find(&agentes).Error; err != nil {
		return nil, err
	}
	return agentes, nil
}

func (r *agenteColmeiaRepository) Update(a *model.AgenteColmeia) error {
	return r.db.Save(a).Error
}

func (r *agenteColmeiaRepository) Delete(id uint) error {
	return r.db.Delete(&model.AgenteColmeia{}, id).Error
}

func (r *agenteColmeiaRepository) DeleteByColmeiaID(colmeiaID string) error {
	return r.db.Where("colmeia_id = ?", colmeiaID).Delete(&model.AgenteColmeia{}).Error
}

// ─── HistoricoDespachoRepository ──────────────────────────────────────────────

// HistoricoDespachoRepository define as operações de banco de dados para HistoricoDespacho.
type HistoricoDespachoRepository interface {
	Create(h *model.HistoricoDespacho) error
	FindByID(id string) (*model.HistoricoDespacho, error)
	FindByColmeiaID(colmeiaID string) ([]model.HistoricoDespacho, error)
	FindRecentCompleted(colmeiaID string, limit int) ([]model.HistoricoDespacho, error)
	Update(h *model.HistoricoDespacho) error
}

type historicoDespachoRepository struct{ db *gorm.DB }

// NewHistoricoDespachoRepository cria um novo HistoricoDespachoRepository.
func NewHistoricoDespachoRepository(db *gorm.DB) HistoricoDespachoRepository {
	return &historicoDespachoRepository{db: db}
}

func (r *historicoDespachoRepository) Create(h *model.HistoricoDespacho) error {
	return r.db.Create(h).Error
}

func (r *historicoDespachoRepository) FindByID(id string) (*model.HistoricoDespacho, error) {
	var h model.HistoricoDespacho
	if err := r.db.First(&h, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoricoNotFound
		}
		return nil, err
	}
	return &h, nil
}

func (r *historicoDespachoRepository) FindByColmeiaID(colmeiaID string) ([]model.HistoricoDespacho, error) {
	var historico []model.HistoricoDespacho
	if err := r.db.Where("colmeia_id = ?", colmeiaID).Order("created_at DESC").Find(&historico).Error; err != nil {
		return nil, err
	}
	return historico, nil
}

// FindRecentCompleted retorna os N despachos concluídos mais recentes (do mais antigo ao mais novo).
func (r *historicoDespachoRepository) FindRecentCompleted(colmeiaID string, limit int) ([]model.HistoricoDespacho, error) {
	var historico []model.HistoricoDespacho
	if err := r.db.Where("colmeia_id = ? AND status = ?", colmeiaID, model.HistoricoStatusCompleted).
		Order("created_at DESC").
		Limit(limit).
		Find(&historico).Error; err != nil {
		return nil, err
	}
	// Inverte para ordem cronológica (mais antigo primeiro).
	for i, j := 0, len(historico)-1; i < j; i, j = i+1, j-1 {
		historico[i], historico[j] = historico[j], historico[i]
	}
	return historico, nil
}

func (r *historicoDespachoRepository) Update(h *model.HistoricoDespacho) error {
	return r.db.Save(h).Error
}
