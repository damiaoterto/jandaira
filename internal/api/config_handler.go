package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/model"
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

	if req.APIKey != "" {
		repoDir := security.GetDefaultVaultDir()
		switch cfg.Provider {
		case "anthropic":
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("ANTHROPIC_API_KEY", req.APIKey)
			}
			os.Setenv("ANTHROPIC_API_KEY", req.APIKey)
			ab := brain.NewAnthropicBrain(req.APIKey, cfg.Model)
			ab.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = ab
		case "openrouter":
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("OPENROUTER_API_KEY", req.APIKey)
			}
			os.Setenv("OPENROUTER_API_KEY", req.APIKey)
			rb := brain.NewOpenRouterBrain(req.APIKey, cfg.Model)
			rb.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = rb
		case "groq":
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("GROQ_API_KEY", req.APIKey)
			}
			os.Setenv("GROQ_API_KEY", req.APIKey)
			gb := brain.NewGroqBrain(req.APIKey, cfg.Model)
			gb.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = gb
		default:
			if v, err := security.InitVault(repoDir); err == nil {
				_ = v.SaveSecret("OPENAI_API_KEY", req.APIKey)
			}
			os.Setenv("OPENAI_API_KEY", req.APIKey)
			ob := brain.NewOpenAIBrain(req.APIKey, cfg.Model)
			ob.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = ob
		}
	} else if req.Provider != "" {
		// Provider changed but no new key — rebuild brain from vault/env with existing key.
		repoDir := security.GetDefaultVaultDir()
		vault, _ := security.InitVault(repoDir)
		switch cfg.Provider {
		case "anthropic":
			apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
			if apiKey == "" && vault != nil {
				if key, err := vault.GetSecret("ANTHROPIC_API_KEY"); err == nil {
					apiKey = strings.TrimSpace(key)
				}
			}
			ab := brain.NewAnthropicBrain(apiKey, cfg.Model)
			ab.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = ab
		case "openrouter":
			apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
			if apiKey == "" && vault != nil {
				if key, err := vault.GetSecret("OPENROUTER_API_KEY"); err == nil {
					apiKey = strings.TrimSpace(key)
				}
			}
			rb := brain.NewOpenRouterBrain(apiKey, cfg.Model)
			rb.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = rb
		case "groq":
			apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
			if apiKey == "" && vault != nil {
				if key, err := vault.GetSecret("GROQ_API_KEY"); err == nil {
					apiKey = strings.TrimSpace(key)
				}
			}
			gb := brain.NewGroqBrain(apiKey, cfg.Model)
			gb.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = gb
		default:
			apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
			if apiKey == "" && vault != nil {
				if key, err := vault.GetSecret("OPENAI_API_KEY"); err == nil {
					apiKey = strings.TrimSpace(key)
				}
			}
			ob := brain.NewOpenAIBrain(apiKey, cfg.Model)
			ob.MaxTokensFn = s.maxTokensFn()
			s.Queen.Brain = ob
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Configuration updated successfully.",
		"provider": cfg.Provider,
		"model":    cfg.Model,
		"brain":    s.Queen.Brain.GetProviderName(),
	})
}
