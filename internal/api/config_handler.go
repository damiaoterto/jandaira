package api

import (
	"net/http"
	"strings"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/provider"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/gin-gonic/gin"
)

// handleGetConfig returns the current application configuration.
//
//	GET /api/config
func (s *Server) handleGetConfig(c *gin.Context) {
	cfg, err := s.configService.Load()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration not found."})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"provider":   cfg.Provider,
		"model":      cfg.Model,
		"swarm_name": cfg.SwarmName,
		"max_nectar": cfg.MaxNectar,
		"max_agents": cfg.MaxAgents,
		"supervised": cfg.Supervised,
		"isolated":   cfg.Isolated,
		"brain":      s.Queen.Brain.GetProviderName(),
	})
}

// handleUpdateConfig updates provider, model and/or API key on the running server.
// Unlike /api/setup, this endpoint can be called multiple times.
//
//	PUT /api/config
func (s *Server) handleUpdateConfig(c *gin.Context) {
	var req struct {
		model.AppConfig
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parameters."})
		return
	}

	cfg, err := s.configService.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configuration."})
		return
	}

	if req.Provider != "" {
		cfg.Provider = strings.ToLower(req.Provider)
	}
	if req.Model != "" {
		cfg.Model = req.Model
	}
	if req.MaxNectar > 0 {
		cfg.MaxNectar = req.MaxNectar
	}
	if req.MaxAgents > 0 {
		cfg.MaxAgents = req.MaxAgents
	}

	if err := s.configService.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration."})
		return
	}

	repoDir := security.GetDefaultVaultDir()
	vault, _ := security.InitVault(repoDir)

	var activeBrain = s.Queen.Brain
	if req.APIKey != "" {
		activeBrain, _, err = provider.BuildBrainsWithKey(cfg.Provider, req.APIKey, cfg.Model, vault, s.maxTokensFn())
	} else if req.Provider != "" {
		activeBrain, _, err = provider.BuildBrains(cfg.Provider, cfg.Model, vault, s.maxTokensFn())
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize brain: " + err.Error()})
		return
	}
	s.Queen.Brain = activeBrain

	c.JSON(http.StatusOK, gin.H{
		"message":  "Configuration updated successfully.",
		"provider": cfg.Provider,
		"model":    cfg.Model,
		"brain":    s.Queen.Brain.GetProviderName(),
	})
}
