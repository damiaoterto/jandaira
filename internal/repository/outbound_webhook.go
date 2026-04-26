package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

var ErrOutboundWebhookNotFound = errors.New("outbound webhook not found")

type OutboundWebhookRepository interface {
	Create(w *model.OutboundWebhook) error
	FindByID(id uint) (*model.OutboundWebhook, error)
	FindByColmeiaID(colmeiaID string) ([]model.OutboundWebhook, error)
	Update(w *model.OutboundWebhook) error
	Delete(id uint) error
}

type outboundWebhookRepository struct {
	db *gorm.DB
}

func NewOutboundWebhookRepository(db *gorm.DB) OutboundWebhookRepository {
	return &outboundWebhookRepository{db: db}
}

func (r *outboundWebhookRepository) Create(w *model.OutboundWebhook) error {
	return r.db.Create(w).Error
}

func (r *outboundWebhookRepository) FindByID(id uint) (*model.OutboundWebhook, error) {
	var w model.OutboundWebhook
	if err := r.db.First(&w, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOutboundWebhookNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *outboundWebhookRepository) FindByColmeiaID(colmeiaID string) ([]model.OutboundWebhook, error) {
	var list []model.OutboundWebhook
	if err := r.db.Where("colmeia_id = ?", colmeiaID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *outboundWebhookRepository) Update(w *model.OutboundWebhook) error {
	return r.db.Save(w).Error
}

func (r *outboundWebhookRepository) Delete(id uint) error {
	return r.db.Delete(&model.OutboundWebhook{}, id).Error
}
