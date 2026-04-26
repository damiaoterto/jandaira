package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/gin-gonic/gin"
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

	if cfg.Model == "" {
		switch cfg.Provider {
		case "anthropic":
			cfg.Model = "claude-sonnet-4-6"
		case "openrouter":
			cfg.Model = "openai/gpt-4o-mini"
		case "groq":
			cfg.Model = "llama-3.3-70b-versatile"
		default:
			cfg.Model = "gpt-4o-mini"
		}
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
		switch cfg.Provider {
		case "anthropic":
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("ANTHROPIC_API_KEY", rawReq.APIKey)
			}
			os.Setenv("ANTHROPIC_API_KEY", rawReq.APIKey)
			ab := brain.NewAnthropicBrain(rawReq.APIKey, cfg.Model)
			ab.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = ab
		case "openrouter":
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("OPENROUTER_API_KEY", rawReq.APIKey)
			}
			os.Setenv("OPENROUTER_API_KEY", rawReq.APIKey)
			rb := brain.NewOpenRouterBrain(rawReq.APIKey, cfg.Model)
			rb.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = rb
		case "groq":
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("GROQ_API_KEY", rawReq.APIKey)
			}
			os.Setenv("GROQ_API_KEY", rawReq.APIKey)
			gb := brain.NewGroqBrain(rawReq.APIKey, cfg.Model)
			gb.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = gb
		default:
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("OPENAI_API_KEY", rawReq.APIKey)
			}
			os.Setenv("OPENAI_API_KEY", rawReq.APIKey)
			ob := brain.NewOpenAIBrain(rawReq.APIKey, cfg.Model)
			ob.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = ob
		}
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
