package api

import (
	"net/http"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/provider"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/gin-gonic/gin"
	"strings"
)

// handleSetup configures the hive on first run.
//
//	POST /api/setup
func (s *Server) handleSetup(c *gin.Context) {
	configured, err := s.configService.IsConfigured()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify configuration."})
		return
	}
	if configured {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Hive already configured. Cannot change the main structure via the setup endpoint.",
		})
		return
	}

	var rawReq struct {
		model.AppConfig
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid configuration parameters."})
		return
	}

	cfg := rawReq.AppConfig

	if cfg.MaxNectar == 0 {
		cfg.MaxNectar = 20000
	}
	if cfg.Provider == "" {
		cfg.Provider = "openai"
	}
	cfg.Provider = strings.ToLower(cfg.Provider)

	if !provider.IsValid(cfg.Provider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown provider: " + cfg.Provider})
		return
	}

	if cfg.Model == "" {
		cfg.Model = provider.DefaultModel(cfg.Provider)
	}
	if cfg.SwarmName == "" {
		cfg.SwarmName = "enxame-alfa"
	}

	if err := s.configService.Save(&cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration to database."})
		return
	}

	if rawReq.APIKey != "" {
		repoDir := security.GetDefaultVaultDir()
		vault, _ := security.InitVault(repoDir)
		activeBrain, _, err := provider.BuildBrainsWithKey(cfg.Provider, rawReq.APIKey, cfg.Model, vault, s.maxTokensFn())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize brain: " + err.Error()})
			return
		}
		s.Queen.Brain = activeBrain
	}

	s.Queen.RegisterSwarm(cfg.SwarmName, swarm.Policy{
		MaxNectar:        cfg.MaxNectar,
		Isolate:          cfg.Isolated,
		RequiresApproval: cfg.Supervised,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration saved successfully! The ecosystem is ready.",
	})
}
