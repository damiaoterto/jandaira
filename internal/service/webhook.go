package service

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
)

var ErrWebhookNotFound = errors.New("webhook não encontrado")

// WebhookService defines business logic for Webhook management.
type WebhookService interface {
	Create(name, slug, colmeiaID, secret, goalTemplate string, active bool) (*model.Webhook, error)
	GetByID(id uint) (*model.Webhook, error)
	GetBySlug(slug string) (*model.Webhook, error)
	List() ([]model.Webhook, error)
	Update(id uint, name, slug, colmeiaID, secret, goalTemplate string, active bool) (*model.Webhook, error)
	Delete(id uint) error

	// ProcessPayload renders the webhook's GoalTemplate against the incoming
	// JSON payload, producing the final goal string for the Queen.
	ProcessPayload(webhook *model.Webhook, payload map[string]interface{}) (string, error)
}

type webhookService struct {
	webhooks repository.WebhookRepository
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(webhooks repository.WebhookRepository) WebhookService {
	return &webhookService{webhooks: webhooks}
}

func (s *webhookService) Create(name, slug, colmeiaID, secret, goalTemplate string, active bool) (*model.Webhook, error) {
	w := &model.Webhook{
		Name:         name,
		Slug:         slug,
		ColmeiaID:    colmeiaID,
		Secret:       secret,
		Active:       active,
		GoalTemplate: goalTemplate,
	}
	if err := s.webhooks.Create(w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *webhookService) GetByID(id uint) (*model.Webhook, error) {
	w, err := s.webhooks.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	return w, nil
}

func (s *webhookService) GetBySlug(slug string) (*model.Webhook, error) {
	w, err := s.webhooks.FindBySlug(slug)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	return w, nil
}

func (s *webhookService) List() ([]model.Webhook, error) {
	return s.webhooks.FindAll()
}

func (s *webhookService) Update(id uint, name, slug, colmeiaID, secret, goalTemplate string, active bool) (*model.Webhook, error) {
	w, err := s.webhooks.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	w.Name = name
	w.Slug = slug
	w.ColmeiaID = colmeiaID
	w.Secret = secret
	w.GoalTemplate = goalTemplate
	w.Active = active
	if err := s.webhooks.Update(w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *webhookService) Delete(id uint) error {
	if _, err := s.webhooks.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return ErrWebhookNotFound
		}
		return err
	}
	return s.webhooks.Delete(id)
}

// ProcessPayload renders GoalTemplate with the incoming JSON payload as data.
// Placeholders follow Go text/template syntax: {{.repository.name}}, {{.env}}, etc.
// The payload map is passed as the template dot (.) so nested keys work via map
// traversal — callers must ensure nested objects are also map[string]interface{}.
func (s *webhookService) ProcessPayload(webhook *model.Webhook, payload map[string]interface{}) (string, error) {
	tmpl, err := template.New("goal").Parse(webhook.GoalTemplate)
	if err != nil {
		return "", fmt.Errorf("invalid goal template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, payload); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}
	return buf.String(), nil
}
