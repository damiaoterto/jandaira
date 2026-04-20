package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

var ErrDocumentNotFound = errors.New("document not found")

type DocumentRepository interface {
	Create(d *model.Document) error
	FindByID(id string) (*model.Document, error)
	FindByScopeVal(scopeKey, scopeVal string) ([]model.Document, error)
	Delete(id string) error
}

type documentRepository struct {
	db *gorm.DB
}

func NewDocumentRepository(db *gorm.DB) DocumentRepository {
	return &documentRepository{db: db}
}

func (r *documentRepository) Create(d *model.Document) error {
	return r.db.Create(d).Error
}

func (r *documentRepository) FindByID(id string) (*model.Document, error) {
	var d model.Document
	if err := r.db.First(&d, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *documentRepository) FindByScopeVal(scopeKey, scopeVal string) ([]model.Document, error) {
	var docs []model.Document
	if err := r.db.Where("scope_key = ? AND scope_val = ?", scopeKey, scopeVal).
		Order("created_at DESC").
		Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}

func (r *documentRepository) Delete(id string) error {
	return r.db.Delete(&model.Document{}, "id = ?", id).Error
}
