package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/gin-gonic/gin"
)

// ─── Public trigger ───────────────────────────────────────────────────────────

// handleWebhookTrigger is the public endpoint called by external systems.
//
//	POST /api/webhooks/:slug
//
// If the webhook has a Secret configured, the caller must send a valid
// X-Hub-Signature-256 header (format: "sha256=<hex-encoded-HMAC-SHA256>").
// The JSON body is rendered through the webhook's GoalTemplate and dispatched
// to the associated Colmeia asynchronously; progress is streamed via WebSocket.
func (s *Server) handleWebhookTrigger(c *gin.Context) {
	slug := c.Param("slug")

	webhook, err := s.webhookService.GetBySlug(slug)
	if err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found."})
			return
		}
		log.Printf("ERROR handleWebhookTrigger GetBySlug slug=%s: %v", slug, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch webhook."})
		return
	}

	if !webhook.Active {
		c.JSON(http.StatusGone, gin.H{"error": "Webhook is inactive."})
		return
	}

	// Read and validate body before parsing so we can verify HMAC on the raw bytes.
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body."})
		return
	}

	if webhook.Secret != "" {
		sig := c.GetHeader("X-Hub-Signature-256")
		if sig == "" {
			sig = c.GetHeader("X-Webhook-Signature")
		}
		if !validateWebhookHMAC([]byte(webhook.Secret), rawBody, sig) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature."})
			return
		}
	}

	var payload map[string]interface{}
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Request body must be valid JSON."})
			return
		}
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}

	goal, err := s.webhookService.ProcessPayload(webhook, payload)
	if err != nil {
		log.Printf("ERROR handleWebhookTrigger ProcessPayload slug=%s: %v", slug, err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("Goal template error: %v", err)})
		return
	}

	colmeia, err := s.colmeiaService.GetColmeia(webhook.ColmeiaID)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia linked to this webhook was not found."})
			return
		}
		log.Printf("ERROR handleWebhookTrigger GetColmeia colmeia=%s: %v", webhook.ColmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch colmeia."})
		return
	}

	enrichedGoal, err := s.colmeiaService.BuildGoalWithHistory(colmeia, goal)
	if err != nil {
		log.Printf("WARN handleWebhookTrigger BuildGoalWithHistory colmeia=%s: %v", colmeia.ID, err)
		enrichedGoal = goal
	}

	if skillsCtx := s.colmeiaService.BuildSkillsContext(colmeia); skillsCtx != "" {
		enrichedGoal = skillsCtx + "\n\n" + enrichedGoal
	}

	historico, err := s.colmeiaService.CreateHistorico(colmeia.ID, goal)
	if err != nil {
		log.Printf("ERROR handleWebhookTrigger CreateHistorico colmeia=%s: %v", colmeia.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register dispatch."})
		return
	}

	cfg, _ := s.configService.Load()
	maxWorkers := 3
	if cfg != nil && cfg.MaxAgents > 0 {
		maxWorkers = cfg.MaxAgents
	}

	groupID := "colmeia-" + sanitizeID(colmeia.ID)

	if !s.Queen.IsSwarmRegistered(groupID) {
		maxNectar := 20000
		if cfg != nil && cfg.MaxNectar > 0 {
			maxNectar = cfg.MaxNectar
		}
		s.Queen.RegisterSwarm(groupID, swarm.Policy{
			MaxNectar:        maxNectar,
			Isolate:          cfg != nil && cfg.Isolated,
			RequiresApproval: cfg != nil && cfg.Supervised,
		})
	}

	enrichedGoal = fmt.Sprintf("[HIVE MEMORY COLLECTION: %s]\n\n%s", groupID, enrichedGoal)

	s.Broadcast(WsMessage{
		Type:    "status",
		Message: fmt.Sprintf("🪝 Webhook '%s' triggered for hive '%s'.", webhook.Name, colmeia.Name),
	})

	if colmeia.QueenManaged {
		assembleCtx, assembleCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer assembleCancel()

		specialists, err := s.Queen.AssembleSwarm(assembleCtx, enrichedGoal, maxWorkers)
		if err != nil {
			_ = s.colmeiaService.FailHistorico(historico.ID)
			log.Printf("ERROR handleWebhookTrigger AssembleSwarm colmeia=%s: %v", colmeia.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to plan the swarm: %v", err)})
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			resultChan, errChan := s.Queen.DispatchWorkflow(ctx, groupID, enrichedGoal, specialists)

			select {
			case result := <-resultChan:
				_ = s.colmeiaService.CompleteHistorico(historico.ID, result)
				s.outboundWebhookService.Enqueue(colmeia.ID, map[string]interface{}{
					"result":       result,
					"goal":         goal,
					"colmeia_id":   colmeia.ID,
					"historico_id": historico.ID,
					"payload":      payload,
				})
				s.Broadcast(WsMessage{Type: "result", Message: result})
			case dispatchErr := <-errChan:
				_ = s.colmeiaService.FailHistorico(historico.ID)
				s.Broadcast(WsMessage{Type: "error", Message: dispatchErr.Error()})
			case <-ctx.Done():
				_ = s.colmeiaService.FailHistorico(historico.ID)
				s.Broadcast(WsMessage{Type: "error", Message: "Webhook mission timeout reached."})
			}
		}()

		c.JSON(http.StatusAccepted, gin.H{
			"message":      "Webhook received. Mission dispatched. Follow progress via WebSocket.",
			"webhook_slug": slug,
			"colmeia_id":   colmeia.ID,
			"historico_id": historico.ID,
			"agents":       len(specialists),
			"mode":         "queen_managed",
		})
		return
	}

	// User-defined agents.
	if len(colmeia.Agentes) == 0 {
		_ = s.colmeiaService.FailHistorico(historico.ID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Colmeia has no defined agents. Add agents or enable queen_managed.",
		})
		return
	}

	predefinedSpecialists := s.colmeiaService.BuildSpecialists(colmeia)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		resultChan, errChan := s.Queen.DispatchWorkflow(ctx, groupID, enrichedGoal, predefinedSpecialists)

		select {
		case result := <-resultChan:
			_ = s.colmeiaService.CompleteHistorico(historico.ID, result)
			s.outboundWebhookService.Enqueue(colmeia.ID, map[string]interface{}{
				"result":       result,
				"goal":         goal,
				"colmeia_id":   colmeia.ID,
				"historico_id": historico.ID,
				"payload":      payload,
			})
			s.Broadcast(WsMessage{Type: "result", Message: result})
		case dispatchErr := <-errChan:
			_ = s.colmeiaService.FailHistorico(historico.ID)
			s.Broadcast(WsMessage{Type: "error", Message: dispatchErr.Error()})
		case <-ctx.Done():
			_ = s.colmeiaService.FailHistorico(historico.ID)
			s.Broadcast(WsMessage{Type: "error", Message: "Webhook mission timeout reached."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":      "Webhook received. Mission dispatched. Follow progress via WebSocket.",
		"webhook_slug": slug,
		"colmeia_id":   colmeia.ID,
		"historico_id": historico.ID,
		"agents":       len(predefinedSpecialists),
		"mode":         "user_defined",
	})
}

// validateWebhookHMAC verifies a GitHub-style HMAC-SHA256 signature.
// signatureHeader must be in the format "sha256=<hex>".
func validateWebhookHMAC(secret, body []byte, signatureHeader string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	gotBytes, err := hex.DecodeString(signatureHeader[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hmac.Equal(gotBytes, mac.Sum(nil))
}

// ─── Management CRUD ──────────────────────────────────────────────────────────

// handleListWebhooks returns all configured webhooks.
//
//	GET /api/webhooks
func (s *Server) handleListWebhooks(c *gin.Context) {
	list, err := s.webhookService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list webhooks."})
		return
	}
	c.JSON(http.StatusOK, list)
}

// handleCreateWebhook creates a new webhook.
//
//	POST /api/webhooks
func (s *Server) handleCreateWebhook(c *gin.Context) {
	var req struct {
		Name         string `json:"name"          binding:"required"`
		Slug         string `json:"slug"          binding:"required"`
		ColmeiaID    string `json:"colmeia_id"    binding:"required"`
		Secret       string `json:"secret"`
		Active       *bool  `json:"active"`
		GoalTemplate string `json:"goal_template" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	w, err := s.webhookService.Create(req.Name, req.Slug, req.ColmeiaID, req.Secret, req.GoalTemplate, active)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create webhook: %v", err)})
		return
	}
	c.JSON(http.StatusCreated, w)
}

// handleGetWebhook returns a single webhook by ID.
//
//	GET /api/webhooks/:id
func (s *Server) handleGetWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID."})
		return
	}

	w, err := s.webhookService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch webhook."})
		return
	}
	c.JSON(http.StatusOK, w)
}

// handleUpdateWebhook updates an existing webhook.
//
//	PUT /api/webhooks/:id
func (s *Server) handleUpdateWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID."})
		return
	}

	var req struct {
		Name         string `json:"name"          binding:"required"`
		Slug         string `json:"slug"          binding:"required"`
		ColmeiaID    string `json:"colmeia_id"    binding:"required"`
		Secret       string `json:"secret"`
		Active       *bool  `json:"active"`
		GoalTemplate string `json:"goal_template" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	w, err := s.webhookService.Update(uint(id), req.Name, req.Slug, req.ColmeiaID, req.Secret, req.GoalTemplate, active)
	if err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update webhook: %v", err)})
		return
	}
	c.JSON(http.StatusOK, w)
}

// handleDeleteWebhook removes a webhook.
//
//	DELETE /api/webhooks/:id
func (s *Server) handleDeleteWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID."})
		return
	}

	if err := s.webhookService.Delete(uint(id)); err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete webhook."})
		return
	}
	c.Status(http.StatusNoContent)
}
