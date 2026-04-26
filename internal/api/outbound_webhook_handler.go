package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/gin-gonic/gin"
)

// handleListOutboundWebhooks lists all outbound webhooks for a colmeia.
//
//	GET /api/colmeias/:id/outbound-webhooks
func (s *Server) handleListOutboundWebhooks(c *gin.Context) {
	colmeiaID := c.Param("id")
	webhooks, err := s.outboundWebhookService.ListByColmeia(colmeiaID)
	if err != nil {
		log.Printf("ERROR handleListOutboundWebhooks colmeia=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list outbound webhooks."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks, "total": len(webhooks)})
}

// handleGetOutboundWebhook returns a single outbound webhook.
//
//	GET /api/colmeias/:id/outbound-webhooks/:webhookId
func (s *Server) handleGetOutboundWebhook(c *gin.Context) {
	webhookID, err := strconv.ParseUint(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID."})
		return
	}
	webhook, err := s.outboundWebhookService.GetByID(uint(webhookID))
	if err != nil {
		if errors.Is(err, service.ErrOutboundWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Outbound webhook not found."})
			return
		}
		log.Printf("ERROR handleGetOutboundWebhook id=%d: %v", webhookID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch outbound webhook."})
		return
	}
	c.JSON(http.StatusOK, webhook)
}

// handleCreateOutboundWebhook creates a new outbound webhook for the colmeia.
//
//	POST /api/colmeias/:id/outbound-webhooks
func (s *Server) handleCreateOutboundWebhook(c *gin.Context) {
	colmeiaID := c.Param("id")
	var req struct {
		Name         string `json:"name" binding:"required"`
		URL          string `json:"url" binding:"required"`
		Method       string `json:"method"`
		Headers      string `json:"headers"`
		BodyTemplate string `json:"body_template" binding:"required"`
		Secret       string `json:"secret"`
		Active       *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fields 'name', 'url', and 'body_template' are required."})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	webhook, err := s.outboundWebhookService.Create(
		colmeiaID,
		req.Name,
		req.URL,
		req.Method,
		req.Headers,
		req.BodyTemplate,
		req.Secret,
		active,
	)
	if err != nil {
		log.Printf("ERROR handleCreateOutboundWebhook colmeia=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create outbound webhook."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Outbound webhook created successfully.", "webhook": webhook})
}

// handleUpdateOutboundWebhook updates an existing outbound webhook.
//
//	PUT /api/colmeias/:id/outbound-webhooks/:webhookId
func (s *Server) handleUpdateOutboundWebhook(c *gin.Context) {
	webhookID, err := strconv.ParseUint(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID."})
		return
	}

	var req struct {
		Name         string `json:"name" binding:"required"`
		URL          string `json:"url" binding:"required"`
		Method       string `json:"method"`
		Headers      string `json:"headers"`
		BodyTemplate string `json:"body_template" binding:"required"`
		Secret       string `json:"secret"`
		Active       *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fields 'name', 'url', and 'body_template' are required."})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	webhook, err := s.outboundWebhookService.Update(
		uint(webhookID),
		req.Name,
		req.URL,
		req.Method,
		req.Headers,
		req.BodyTemplate,
		req.Secret,
		active,
	)
	if err != nil {
		if errors.Is(err, service.ErrOutboundWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Outbound webhook not found."})
			return
		}
		log.Printf("ERROR handleUpdateOutboundWebhook id=%d: %v", webhookID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update outbound webhook."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Outbound webhook updated.", "webhook": webhook})
}

// handleDeleteOutboundWebhook removes an outbound webhook.
//
//	DELETE /api/colmeias/:id/outbound-webhooks/:webhookId
func (s *Server) handleDeleteOutboundWebhook(c *gin.Context) {
	webhookID, err := strconv.ParseUint(c.Param("webhookId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID."})
		return
	}

	if err := s.outboundWebhookService.Delete(uint(webhookID)); err != nil {
		if errors.Is(err, service.ErrOutboundWebhookNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Outbound webhook not found."})
			return
		}
		log.Printf("ERROR handleDeleteOutboundWebhook id=%d: %v", webhookID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete outbound webhook."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Outbound webhook deleted successfully."})
}
