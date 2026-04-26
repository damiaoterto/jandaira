package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

var ErrWebhookNotFound = errors.New("webhook não encontrado")

// WebhookRepository defines database operations for Webhook.
type WebhookRepository interface {
	Create(w *model.Webhook) error
	FindByID(id uint) (*model.Webhook, error)
	FindBySlug(slug string) (*model.Webhook, error)
	FindAll() ([]model.Webhook, error)
	Update(w *model.Webhook) error
	Delete(id uint) error
}

type webhookRepository struct{ db *gorm.DB }

// NewWebhookRepository creates a new WebhookRepository.
func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &webhookRepository{db: db}
}

func (r *webhookRepository) Create(w *model.Webhook) error {
	return r.db.Create(w).Error
}

func (r *webhookRepository) FindByID(id uint) (*model.Webhook, error) {
	var w model.Webhook
	if err := r.db.First(&w, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *webhookRepository) FindBySlug(slug string) (*model.Webhook, error) {
	var w model.Webhook
	if err := r.db.Where("slug = ?", slug).First(&w).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *webhookRepository) FindAll() ([]model.Webhook, error) {
	var webhooks []model.Webhook
	if err := r.db.Order("created_at DESC").Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

func (r *webhookRepository) Update(w *model.Webhook) error {
	return r.db.Save(w).Error
}

func (r *webhookRepository) Delete(id uint) error {
	return r.db.Delete(&model.Webhook{}, id).Error
}
