package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
)

var ErrOutboundWebhookNotFound = errors.New("outbound webhook not found")

// OutboundDispatchJob is the unit of work sent to the dispatch channel.
type OutboundDispatchJob struct {
	ColmeiaID string
	Data      map[string]interface{}
}

// OutboundWebhookService manages outbound webhook configuration and operates a
// channel-based worker pool that fans out HTTP calls after a mission completes.
type OutboundWebhookService interface {
	Create(colmeiaID, name, url, method, headers, bodyTemplate, secret string, active bool) (*model.OutboundWebhook, error)
	GetByID(id uint) (*model.OutboundWebhook, error)
	ListByColmeia(colmeiaID string) ([]model.OutboundWebhook, error)
	Update(id uint, name, url, method, headers, bodyTemplate, secret string, active bool) (*model.OutboundWebhook, error)
	Delete(id uint) error

	// Start launches `workers` goroutines that consume the dispatch channel.
	// Call once at application startup.
	Start(workers int)

	// Enqueue sends a dispatch job to the channel without blocking.
	// If the buffer is full, the job is dropped and a warning is logged.
	Enqueue(colmeiaID string, data map[string]interface{})
}

type outboundWebhookService struct {
	repo   repository.OutboundWebhookRepository
	client *http.Client
	jobs   chan OutboundDispatchJob
}

func NewOutboundWebhookService(repo repository.OutboundWebhookRepository) OutboundWebhookService {
	return &outboundWebhookService{
		repo:   repo,
		client: &http.Client{Timeout: 30 * time.Second},
		jobs:   make(chan OutboundDispatchJob, 64),
	}
}

func (s *outboundWebhookService) Start(workers int) {
	for i := 0; i < workers; i++ {
		go s.worker()
	}
}

func (s *outboundWebhookService) worker() {
	for job := range s.jobs {
		s.dispatchAll(job.ColmeiaID, job.Data)
	}
}

func (s *outboundWebhookService) Enqueue(colmeiaID string, data map[string]interface{}) {
	select {
	case s.jobs <- OutboundDispatchJob{ColmeiaID: colmeiaID, Data: data}:
	default:
		log.Printf("WARN OutboundWebhookService: queue full, dropping dispatch for colmeia=%s", colmeiaID)
	}
}

// dispatchAll fetches all active outbound webhooks for the colmeia and fires
// each one concurrently, waiting for all calls to finish before returning.
func (s *outboundWebhookService) dispatchAll(colmeiaID string, data map[string]interface{}) {
	list, err := s.repo.FindByColmeiaID(colmeiaID)
	if err != nil {
		log.Printf("ERROR OutboundWebhookService.dispatchAll colmeia=%s: %v", colmeiaID, err)
		return
	}

	var wg sync.WaitGroup
	for _, w := range list {
		if !w.Active {
			continue
		}
		wg.Add(1)
		go func(wh model.OutboundWebhook) {
			defer wg.Done()
			if err := s.dispatch(&wh, data); err != nil {
				log.Printf("ERROR outbound webhook id=%d url=%s: %v", wh.ID, wh.URL, err)
			}
		}(w)
	}
	wg.Wait()
}

func (s *outboundWebhookService) dispatch(w *model.OutboundWebhook, data map[string]interface{}) error {
	body, err := renderOutboundTemplate(w.BodyTemplate, data)
	if err != nil {
		return fmt.Errorf("body template: %w", err)
	}

	req, err := http.NewRequest(w.Method, w.URL, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	var headers map[string]string
	if w.Headers != "" && w.Headers != "{}" {
		if jsonErr := json.Unmarshal([]byte(w.Headers), &headers); jsonErr != nil {
			log.Printf("WARN outbound webhook %d invalid headers JSON: %v", w.ID, jsonErr)
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if w.Secret != "" {
		mac := hmac.New(sha256.New, []byte(w.Secret))
		mac.Write([]byte(body))
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote returned %d: %s", resp.StatusCode, string(respBody))
	}

	io.Copy(io.Discard, resp.Body)
	return nil
}

// ─── CRUD ─────────────────────────────────────────────────────────────────────

func (s *outboundWebhookService) Create(colmeiaID, name, url, method, headers, bodyTemplate, secret string, active bool) (*model.OutboundWebhook, error) {
	if method == "" {
		method = "POST"
	}
	if headers == "" {
		headers = "{}"
	}
	w := &model.OutboundWebhook{
		ColmeiaID:    colmeiaID,
		Name:         name,
		URL:          url,
		Method:       strings.ToUpper(method),
		Headers:      headers,
		BodyTemplate: bodyTemplate,
		Secret:       secret,
		Active:       active,
	}
	if err := s.repo.Create(w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *outboundWebhookService) GetByID(id uint) (*model.OutboundWebhook, error) {
	w, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrOutboundWebhookNotFound) {
			return nil, ErrOutboundWebhookNotFound
		}
		return nil, err
	}
	return w, nil
}

func (s *outboundWebhookService) ListByColmeia(colmeiaID string) ([]model.OutboundWebhook, error) {
	return s.repo.FindByColmeiaID(colmeiaID)
}

func (s *outboundWebhookService) Update(id uint, name, url, method, headers, bodyTemplate, secret string, active bool) (*model.OutboundWebhook, error) {
	w, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrOutboundWebhookNotFound) {
			return nil, ErrOutboundWebhookNotFound
		}
		return nil, err
	}
	if method == "" {
		method = "POST"
	}
	if headers == "" {
		headers = "{}"
	}
	w.Name = name
	w.URL = url
	w.Method = strings.ToUpper(method)
	w.Headers = headers
	w.BodyTemplate = bodyTemplate
	w.Secret = secret
	w.Active = active
	if err := s.repo.Update(w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *outboundWebhookService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrOutboundWebhookNotFound) {
			return ErrOutboundWebhookNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

func renderOutboundTemplate(tmplStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("body").Funcs(template.FuncMap{
		"json": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"truncate": func(length int, v interface{}) string {
			s := fmt.Sprintf("%v", v)
			runes := []rune(s)
			if len(runes) > length {
				return string(runes[:length-3]) + "..."
			}
			return s
		},
		"normalize": func(v interface{}) string {
			s := fmt.Sprintf("%v", v)
			if idx := strings.LastIndex(s, "--- Nova mensagem do usuário ---"); idx != -1 {
				s = s[idx:]
			}
			if idx := strings.LastIndex(s, "--- Report from "); idx != -1 {
				s = s[idx:]
				if nlIdx := strings.Index(s, "\n"); nlIdx != -1 {
					s = s[nlIdx+1:]
				}
			} else if idx := strings.LastIndex(s, "--- Agent "); idx != -1 {
				s = s[idx:]
				if nlIdx := strings.Index(s, "\n"); nlIdx != -1 {
					s = s[nlIdx+1:]
				}
			}
			return strings.TrimSpace(s)
		},
	}).Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
