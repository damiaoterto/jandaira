package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/gin-gonic/gin"
)

// ─── Colmeia ───────────────────────────────────────────────────────────────────

// handleListColmeias retorna todas as colmeias.
//
//	GET /api/colmeias
func (s *Server) handleListColmeias(c *gin.Context) {
	colmeias, err := s.colmeiaService.ListColmeias()
	if err != nil {
		log.Printf("ERROR handleListColmeias: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar colmeias."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"colmeias": colmeias, "total": len(colmeias)})
}

// handleCreateColmeia cria uma nova colmeia persistente.
//
//	POST /api/colmeias
//	Body: { "name": "Minha Colmeia", "description": "...", "queen_managed": true }
func (s *Server) handleCreateColmeia(c *gin.Context) {
	var req struct {
		Name         string `json:"name"          binding:"required"`
		Description  string `json:"description"`
		QueenManaged *bool  `json:"queen_managed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'name' é obrigatório."})
		return
	}

	queenManaged := true
	if req.QueenManaged != nil {
		queenManaged = *req.QueenManaged
	}

	colmeia, err := s.colmeiaService.CreateColmeia(req.Name, req.Description, queenManaged)
	if err != nil {
		log.Printf("ERROR handleCreateColmeia: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar colmeia."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Colmeia criada com sucesso.", "colmeia": colmeia})
}

// handleGetColmeia retorna uma colmeia com seus agentes.
//
//	GET /api/colmeias/:id
func (s *Server) handleGetColmeia(c *gin.Context) {
	colmeia, err := s.colmeiaService.GetColmeia(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia não encontrada."})
			return
		}
		log.Printf("ERROR handleGetColmeia id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar colmeia."})
		return
	}
	c.JSON(http.StatusOK, colmeia)
}

// handleUpdateColmeia atualiza os dados de uma colmeia.
//
//	PUT /api/colmeias/:id
//	Body: { "name": "...", "description": "...", "queen_managed": false }
func (s *Server) handleUpdateColmeia(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name         string `json:"name"          binding:"required"`
		Description  string `json:"description"`
		QueenManaged *bool  `json:"queen_managed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'name' é obrigatório."})
		return
	}

	queenManaged := true
	if req.QueenManaged != nil {
		queenManaged = *req.QueenManaged
	}

	colmeia, err := s.colmeiaService.UpdateColmeia(id, req.Name, req.Description, queenManaged)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia não encontrada."})
			return
		}
		log.Printf("ERROR handleUpdateColmeia id=%s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar colmeia."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Colmeia atualizada.", "colmeia": colmeia})
}

// handleDeleteColmeia remove uma colmeia e todos os seus agentes e histórico.
//
//	DELETE /api/colmeias/:id
func (s *Server) handleDeleteColmeia(c *gin.Context) {
	id := c.Param("id")
	if err := s.colmeiaService.DeleteColmeia(id); err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia não encontrada."})
			return
		}
		log.Printf("ERROR handleDeleteColmeia id=%s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deletar colmeia."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Colmeia, agentes e histórico removidos com sucesso."})
}

// ─── Agentes da Colmeia ────────────────────────────────────────────────────────

// handleListAgentesColmeia lista os agentes de uma colmeia.
//
//	GET /api/colmeias/:id/agentes
func (s *Server) handleListAgentesColmeia(c *gin.Context) {
	agentes, err := s.colmeiaService.ListAgentes(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia não encontrada."})
			return
		}
		log.Printf("ERROR handleListAgentesColmeia id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar agentes."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agentes": agentes, "total": len(agentes)})
}

// handleAddAgenteColmeia adiciona um agente pré-definido à colmeia.
//
//	POST /api/colmeias/:id/agentes
//	Body: { "name": "Analista", "system_prompt": "...", "allowed_tools": ["web_search"] }
func (s *Server) handleAddAgenteColmeia(c *gin.Context) {
	colmeiaID := c.Param("id")
	var req struct {
		Name         string   `json:"name"          binding:"required"`
		SystemPrompt string   `json:"system_prompt" binding:"required"`
		AllowedTools []string `json:"allowed_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Os campos 'name' e 'system_prompt' são obrigatórios."})
		return
	}

	agente, err := s.colmeiaService.AddAgente(colmeiaID, req.Name, req.SystemPrompt, req.AllowedTools)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia não encontrada."})
			return
		}
		log.Printf("ERROR handleAddAgenteColmeia colmeia=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao adicionar agente."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Agente adicionado à colmeia.", "agente": agente})
}

// handleUpdateAgenteColmeia edita o nome, prompt e ferramentas de um agente.
//
//	PUT /api/colmeias/:id/agentes/:agentId
//	Body: { "name": "...", "system_prompt": "...", "allowed_tools": [...] }
func (s *Server) handleUpdateAgenteColmeia(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de agente inválido."})
		return
	}
	var req struct {
		Name         string   `json:"name"          binding:"required"`
		SystemPrompt string   `json:"system_prompt" binding:"required"`
		AllowedTools []string `json:"allowed_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Os campos 'name' e 'system_prompt' são obrigatórios."})
		return
	}

	agente, err := s.colmeiaService.UpdateAgente(uint(agenteID), req.Name, req.SystemPrompt, req.AllowedTools)
	if err != nil {
		if errors.Is(err, service.ErrAgenteColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agente não encontrado."})
			return
		}
		log.Printf("ERROR handleUpdateAgenteColmeia agente=%d: %v", agenteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar agente."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Agente atualizado.", "agente": agente})
}

// handleRemoveAgenteColmeia remove um agente da colmeia.
//
//	DELETE /api/colmeias/:id/agentes/:agentId
func (s *Server) handleRemoveAgenteColmeia(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de agente inválido."})
		return
	}
	if err := s.colmeiaService.RemoveAgente(uint(agenteID)); err != nil {
		if errors.Is(err, service.ErrAgenteColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agente não encontrado."})
			return
		}
		log.Printf("ERROR handleRemoveAgenteColmeia agente=%d: %v", agenteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao remover agente."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Agente removido da colmeia."})
}

// ─── Histórico ─────────────────────────────────────────────────────────────────

// handleListHistoricoColmeia retorna o histórico de despachos de uma colmeia.
//
//	GET /api/colmeias/:id/historico
func (s *Server) handleListHistoricoColmeia(c *gin.Context) {
	historico, err := s.colmeiaService.ListHistorico(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia não encontrada."})
			return
		}
		log.Printf("ERROR handleListHistoricoColmeia id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar histórico."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"historico": historico, "total": len(historico)})
}

// ─── Despacho ──────────────────────────────────────────────────────────────────

// handleColmeiaDispatch envia um objetivo para a colmeia e executa o workflow.
// Se queen_managed=true, a rainha monta o enxame automaticamente.
// Se queen_managed=false, usa os agentes pré-definidos pelo usuário.
// O histórico de conversas anteriores é injetado como contexto para continuidade.
//
//	POST /api/colmeias/:id/dispatch
//	Body: { "goal": "O que você quer que a colmeia faça" }
func (s *Server) handleColmeiaDispatch(c *gin.Context) {
	colmeiaID := c.Param("id")

	colmeia, err := s.colmeiaService.GetColmeia(colmeiaID)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Colmeia não encontrada."})
			return
		}
		log.Printf("ERROR handleColmeiaDispatch GetColmeia id=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar colmeia."})
		return
	}

	var req struct {
		Goal string `json:"goal" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'goal' é obrigatório."})
		return
	}

	// Enriquecer o objetivo com o histórico anterior da colmeia.
	enrichedGoal, err := s.colmeiaService.BuildGoalWithHistory(colmeia, req.Goal)
	if err != nil {
		log.Printf("WARN handleColmeiaDispatch BuildGoalWithHistory id=%s: %v", colmeiaID, err)
		enrichedGoal = req.Goal
	}

	// Registrar o despacho no histórico (goal original, sem contexto injetado).
	historico, err := s.colmeiaService.CreateHistorico(colmeiaID, req.Goal)
	if err != nil {
		log.Printf("ERROR handleColmeiaDispatch CreateHistorico id=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao registrar despacho."})
		return
	}

	cfg, _ := s.configService.Load()
	maxWorkers := 3
	if cfg != nil && cfg.MaxAgents > 0 {
		maxWorkers = cfg.MaxAgents
	}
	groupID := "enxame-alfa"
	if cfg != nil && cfg.SwarmName != "" {
		groupID = cfg.SwarmName
	}

	if colmeia.QueenManaged {
		// Rainha monta o enxame automaticamente a partir do objetivo.
		assembleCtx, assembleCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer assembleCancel()

		swarmSpecialists, err := s.Queen.AssembleSwarm(assembleCtx, enrichedGoal, maxWorkers)
		if err != nil {
			_ = s.colmeiaService.FailHistorico(historico.ID)
			log.Printf("ERROR handleColmeiaDispatch AssembleSwarm colmeia=%s: %v", colmeiaID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Falha ao planejar o enxame: %v", err)})
			return
		}

		s.Broadcast(WsMessage{
			Type:    "status",
			Message: fmt.Sprintf("👑 Rainha montou %d agente(s) para '%s'.", len(swarmSpecialists), colmeia.Name),
		})

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			s.Queen.AgentChangeFunc = func(agentName string) {
				s.Broadcast(WsMessage{Type: "agent_change", Agent: agentName})
			}

			resultChan, errChan := s.Queen.DispatchWorkflow(ctx, groupID, enrichedGoal, swarmSpecialists)

			select {
			case result := <-resultChan:
				_ = s.colmeiaService.CompleteHistorico(historico.ID, result)
				s.Broadcast(WsMessage{Type: "result", Message: result})
			case dispatchErr := <-errChan:
				_ = s.colmeiaService.FailHistorico(historico.ID)
				s.Broadcast(WsMessage{Type: "error", Message: dispatchErr.Error()})
			case <-ctx.Done():
				_ = s.colmeiaService.FailHistorico(historico.ID)
				s.Broadcast(WsMessage{Type: "error", Message: "Tempo limite da missão atingido."})
			}
		}()

		c.JSON(http.StatusAccepted, gin.H{
			"message":      "Missão despachada. Acompanhe o progresso via WebSocket.",
			"colmeia_id":   colmeiaID,
			"historico_id": historico.ID,
			"agents":       len(swarmSpecialists),
			"mode":         "queen_managed",
		})
		return
	}

	// Agentes pré-definidos pelo usuário.
	if len(colmeia.Agentes) == 0 {
		_ = s.colmeiaService.FailHistorico(historico.ID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "A colmeia não possui agentes definidos. Adicione agentes via POST /api/colmeias/:id/agentes ou ative queen_managed.",
		})
		return
	}

	predefinedSpecialists := s.colmeiaService.BuildSpecialists(colmeia)

	s.Broadcast(WsMessage{
		Type:    "status",
		Message: fmt.Sprintf("🐝 Colmeia '%s' usando %d agente(s) pré-definido(s).", colmeia.Name, len(predefinedSpecialists)),
	})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		s.Queen.AgentChangeFunc = func(agentName string) {
			s.Broadcast(WsMessage{Type: "agent_change", Agent: agentName})
		}

		resultChan, errChan := s.Queen.DispatchWorkflow(ctx, groupID, enrichedGoal, predefinedSpecialists)

		select {
		case result := <-resultChan:
			_ = s.colmeiaService.CompleteHistorico(historico.ID, result)
			s.Broadcast(WsMessage{Type: "result", Message: result})
		case dispatchErr := <-errChan:
			_ = s.colmeiaService.FailHistorico(historico.ID)
			s.Broadcast(WsMessage{Type: "error", Message: dispatchErr.Error()})
		case <-ctx.Done():
			_ = s.colmeiaService.FailHistorico(historico.ID)
			s.Broadcast(WsMessage{Type: "error", Message: "Tempo limite da missão atingido."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":      "Missão despachada. Acompanhe o progresso via WebSocket.",
		"colmeia_id":   colmeiaID,
		"historico_id": historico.ID,
		"agents":       len(predefinedSpecialists),
		"mode":         "user_defined",
	})
}
