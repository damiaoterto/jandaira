package api

import (
	"net/http"
	"os"

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao verificar configuração."})
		return
	}
	if configured {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "A colmeia já foi configurada. Não é possível alterar a estrutura principal pelo setup base.",
		})
		return
	}

	var rawReq struct {
		model.AppConfig
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetros inválidos para configuração."})
		return
	}

	cfg := rawReq.AppConfig

	if cfg.MaxNectar == 0 {
		cfg.MaxNectar = 20000
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.SwarmName == "" {
		cfg.SwarmName = "enxame-alfa"
	}

	if err := s.configService.Save(&cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao gravar configuração no banco de dados."})
		return
	}

	if rawReq.APIKey != "" {
		repoDir := security.GetDefaultVaultDir()
		if v, err := security.InitVault(repoDir); err == nil {
			_ = v.SaveSecret("OPENAI_API_KEY", rawReq.APIKey)
		}
		os.Setenv("OPENAI_API_KEY", rawReq.APIKey)
		if b, ok := s.Queen.Brain.(*brain.OpenAIBrain); ok {
			b.APIKey = rawReq.APIKey
		}
	}

	s.Queen.RegisterSwarm(cfg.SwarmName, swarm.Policy{
		MaxNectar:        cfg.MaxNectar,
		Isolate:          cfg.Isolated,
		RequiresApproval: cfg.Supervised,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuração salva com sucesso! O ecossistema está pronto e as operárias acordaram.",
	})
}
