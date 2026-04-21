package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/damiaoterto/jandaira/internal/swarm"
	"github.com/gin-gonic/gin"
)

// ─── Colmeia ───────────────────────────────────────────────────────────────────

// handleListColmeias returns all colmeias.
//
//	GET /api/colmeias
func (s *Server) handleListColmeias(c *gin.Context) {
	colmeias, err := s.colmeiaService.ListColmeias()
	if err != nil {
		log.Printf("ERROR handleListColmeias: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list hives."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"colmeias": colmeias, "total": len(colmeias)})
}

// handleCreateColmeia creates a new persistent colmeia.
//
//	POST /api/colmeias
//	Body: { "name": "My Hive", "description": "...", "queen_managed": true }
func (s *Server) handleCreateColmeia(c *gin.Context) {
	var req struct {
		Name         string `json:"name"          binding:"required"`
		Description  string `json:"description"`
		QueenManaged *bool  `json:"queen_managed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'name' is required."})
		return
	}

	queenManaged := true
	if req.QueenManaged != nil {
		queenManaged = *req.QueenManaged
	}

	colmeia, err := s.colmeiaService.CreateColmeia(req.Name, req.Description, queenManaged)
	if err != nil {
		log.Printf("ERROR handleCreateColmeia: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create hive."})
		return
	}

	// Eagerly create the colmeia's Qdrant collection so it exists before any
	// document is uploaded or store_memory is called.
	if s.Queen.Honeycomb != nil {
		ensureCtx, ensureCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ensureCancel()

		dim := 1536
		if vec, embedErr := s.Queen.Brain.Embed(ensureCtx, "init"); embedErr == nil {
			dim = len(vec)
		}

		groupID := "colmeia-" + sanitizeID(colmeia.ID)
		if ensErr := s.Queen.Honeycomb.EnsureCollection(ensureCtx, groupID, dim); ensErr != nil {
			log.Printf("WARN handleCreateColmeia EnsureCollection colmeia=%s: %v", colmeia.ID, ensErr)
		}
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Hive created successfully.", "colmeia": colmeia})
}

// handleGetColmeia returns a colmeia with its agents.
//
//	GET /api/colmeias/:id
func (s *Server) handleGetColmeia(c *gin.Context) {
	colmeia, err := s.colmeiaService.GetColmeia(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleGetColmeia id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch hive."})
		return
	}
	c.JSON(http.StatusOK, colmeia)
}

// handleUpdateColmeia updates colmeia fields.
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'name' is required."})
		return
	}

	queenManaged := true
	if req.QueenManaged != nil {
		queenManaged = *req.QueenManaged
	}

	colmeia, err := s.colmeiaService.UpdateColmeia(id, req.Name, req.Description, queenManaged)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleUpdateColmeia id=%s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update hive."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Hive updated.", "colmeia": colmeia})
}

// handleDeleteColmeia removes a colmeia along with all its agents and history.
//
//	DELETE /api/colmeias/:id
func (s *Server) handleDeleteColmeia(c *gin.Context) {
	id := c.Param("id")
	if err := s.colmeiaService.DeleteColmeia(id); err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleDeleteColmeia id=%s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete hive."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Hive, agents and history deleted successfully."})
}

// ─── Colmeia Agents ────────────────────────────────────────────────────────────

// handleListAgentesColmeia lists agents of a colmeia.
//
//	GET /api/colmeias/:id/agentes
func (s *Server) handleListAgentesColmeia(c *gin.Context) {
	agentes, err := s.colmeiaService.ListAgentes(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleListAgentesColmeia id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agentes": agentes, "total": len(agentes)})
}

// handleGetAgenteColmeia returns a single agent of a colmeia.
//
//	GET /api/colmeias/:id/agentes/:agentId
func (s *Server) handleGetAgenteColmeia(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID."})
		return
	}
	agente, err := s.colmeiaService.GetAgente(uint(agenteID))
	if err != nil {
		if errors.Is(err, service.ErrAgenteColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found."})
			return
		}
		log.Printf("ERROR handleGetAgenteColmeia agente=%d: %v", agenteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agent."})
		return
	}
	c.JSON(http.StatusOK, agente)
}

// handleAddAgenteColmeia adds a pre-defined agent to the colmeia.
// Returns 409 Conflict if the colmeia is queen_managed — agents are assembled
// automatically by the Queen in that mode and must not be pre-defined manually.
//
//	POST /api/colmeias/:id/agentes
//	Body: { "name": "Analyst", "system_prompt": "...", "allowed_tools": ["web_search"] }
func (s *Server) handleAddAgenteColmeia(c *gin.Context) {
	colmeiaID := c.Param("id")
	var req struct {
		Name         string   `json:"name"          binding:"required"`
		SystemPrompt string   `json:"system_prompt" binding:"required"`
		AllowedTools []string `json:"allowed_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fields 'name' and 'system_prompt' are required."})
		return
	}

	colmeia, err := s.colmeiaService.GetColmeia(colmeiaID)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleAddAgenteColmeia GetColmeia colmeia=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch hive."})
		return
	}

	if colmeia.QueenManaged {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Queen-managed hive does not accept pre-defined agents. Set queen_managed=false to use custom agents.",
		})
		return
	}

	agente, err := s.colmeiaService.AddAgente(colmeiaID, req.Name, req.SystemPrompt, req.AllowedTools)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleAddAgenteColmeia colmeia=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add agent."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Agent added to hive.", "agente": agente})
}

// handleUpdateAgenteColmeia updates an agent's name, system prompt, and allowed tools.
//
//	PUT /api/colmeias/:id/agentes/:agentId
//	Body: { "name": "...", "system_prompt": "...", "allowed_tools": [...] }
func (s *Server) handleUpdateAgenteColmeia(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID."})
		return
	}
	var req struct {
		Name         string   `json:"name"          binding:"required"`
		SystemPrompt string   `json:"system_prompt" binding:"required"`
		AllowedTools []string `json:"allowed_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fields 'name' and 'system_prompt' are required."})
		return
	}

	agente, err := s.colmeiaService.UpdateAgente(uint(agenteID), req.Name, req.SystemPrompt, req.AllowedTools)
	if err != nil {
		if errors.Is(err, service.ErrAgenteColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found."})
			return
		}
		log.Printf("ERROR handleUpdateAgenteColmeia agente=%d: %v", agenteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Agent updated.", "agente": agente})
}

// handleRemoveAgenteColmeia removes an agent from the colmeia.
//
//	DELETE /api/colmeias/:id/agentes/:agentId
func (s *Server) handleRemoveAgenteColmeia(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID."})
		return
	}
	if err := s.colmeiaService.RemoveAgente(uint(agenteID)); err != nil {
		if errors.Is(err, service.ErrAgenteColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found."})
			return
		}
		log.Printf("ERROR handleRemoveAgenteColmeia agente=%d: %v", agenteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove agent."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Agent removed from hive."})
}

// ─── History ───────────────────────────────────────────────────────────────────

// handleListHistoricoColmeia returns the dispatch history of a colmeia.
//
//	GET /api/colmeias/:id/historico
func (s *Server) handleListHistoricoColmeia(c *gin.Context) {
	historico, err := s.colmeiaService.ListHistorico(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleListHistoricoColmeia id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch history."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"historico": historico, "total": len(historico)})
}

// ─── Dispatch ──────────────────────────────────────────────────────────────────

// handleColmeiaDispatch sends a goal to the colmeia and executes the workflow.
// If queen_managed=true, the queen assembles the swarm automatically.
// If queen_managed=false, uses the agents pre-defined by the user.
// Previous conversation history and semantic memory are injected as context.
//
//	POST /api/colmeias/:id/dispatch
//	Body: { "goal": "What you want the colmeia to do" }
func (s *Server) handleColmeiaDispatch(c *gin.Context) {
	colmeiaID := c.Param("id")

	colmeia, err := s.colmeiaService.GetColmeia(colmeiaID)
	if err != nil {
		if errors.Is(err, service.ErrColmeiaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Hive not found."})
			return
		}
		log.Printf("ERROR handleColmeiaDispatch GetColmeia id=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch hive."})
		return
	}

	var req struct {
		Goal string `json:"goal" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'goal' is required."})
		return
	}

	// Enrich the goal with the colmeia's previous DB history.
	enrichedGoal, err := s.colmeiaService.BuildGoalWithHistory(colmeia, req.Goal)
	if err != nil {
		log.Printf("WARN handleColmeiaDispatch BuildGoalWithHistory id=%s: %v", colmeiaID, err)
		enrichedGoal = req.Goal
	}

	// Prepend skills context so the Queen (or pre-defined agents) are aware of
	// available skills for this mission.
	if skillsCtx := s.colmeiaService.BuildSkillsContext(colmeia); skillsCtx != "" {
		enrichedGoal = skillsCtx + "\n\n" + enrichedGoal
	}

	// Record the dispatch in history using the original goal (without injected context).
	historico, err := s.colmeiaService.CreateHistorico(colmeiaID, req.Goal)
	if err != nil {
		log.Printf("ERROR handleColmeiaDispatch CreateHistorico id=%s: %v", colmeiaID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register dispatch."})
		return
	}

	cfg, _ := s.configService.Load()
	maxWorkers := 3
	if cfg != nil && cfg.MaxAgents > 0 {
		maxWorkers = cfg.MaxAgents
	}
	// Each colmeia has its own vector memory collection scoped by its ID.
	// Must match the collection name used in handleColmeiaUploadDocument.
	groupID := "colmeia-" + sanitizeID(colmeiaID)

	// Ensure the colmeia's group is registered in the hive before dispatching.
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

	// Ensure the colmeia's Qdrant collection exists before any search or store.
	// Covers colmeias created before the eager-EnsureCollection fix.
	if s.Queen.Honeycomb != nil {
		ensureCtx, ensureCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ensureCancel()
		dim := 1536
		if vec, embedErr := s.Queen.Brain.Embed(ensureCtx, "init"); embedErr == nil {
			dim = len(vec)
		}
		if ensErr := s.Queen.Honeycomb.EnsureCollection(ensureCtx, groupID, dim); ensErr != nil {
			log.Printf("WARN handleColmeiaDispatch EnsureCollection colmeia=%s: %v", colmeiaID, ensErr)
		}
	}

	// Enrich with semantic memory from Honeycomb (vector DB) if available.
	if s.Queen.Honeycomb != nil {
		memCtx, memCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer memCancel()

		queryVec, err := s.Queen.Brain.Embed(memCtx, req.Goal)
		if err == nil {
			results, err := s.Queen.Honeycomb.Search(memCtx, groupID, queryVec, 3)
			if err == nil && len(results) > 0 {
				var sb strings.Builder
				sb.WriteString(enrichedGoal)
				sb.WriteString("\n\n--- Relevant semantic memory for this colmeia ---\n")
				for _, r := range results {
					if content, ok := r.Metadata["content"]; ok {
						preview := content
						if len(preview) > 400 {
							preview = preview[:400] + "..."
						}
						sb.WriteString(fmt.Sprintf("[score: %.2f]\n%s\n\n", r.Score, preview))
					}
				}
				enrichedGoal = sb.String()
			}
		}
	}

	// Inject collection name so agents can pass it explicitly to search_memory.
	enrichedGoal = fmt.Sprintf("[HIVE MEMORY COLLECTION: %s]\n\n%s", groupID, enrichedGoal)

	if colmeia.QueenManaged {
		// Queen assembles the swarm automatically from the goal.
		assembleCtx, assembleCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer assembleCancel()

		swarmSpecialists, err := s.Queen.AssembleSwarm(assembleCtx, enrichedGoal, maxWorkers)
		if err != nil {
			_ = s.colmeiaService.FailHistorico(historico.ID)
			log.Printf("ERROR handleColmeiaDispatch AssembleSwarm colmeia=%s: %v", colmeiaID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to plan the swarm: %v", err)})
			return
		}

		s.Broadcast(WsMessage{
			Type:    "status",
			Message: fmt.Sprintf("👑 Queen assembled %d agent(s) for '%s'.", len(swarmSpecialists), colmeia.Name),
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
				s.Broadcast(WsMessage{Type: "error", Message: "Mission timeout reached."})
			}
		}()

		c.JSON(http.StatusAccepted, gin.H{
			"message":      "Mission dispatched. Follow progress via WebSocket.",
			"colmeia_id":   colmeiaID,
			"historico_id": historico.ID,
			"agents":       len(swarmSpecialists),
			"mode":         "queen_managed",
		})
		return
	}

	// User-defined agents.
	if len(colmeia.Agentes) == 0 {
		_ = s.colmeiaService.FailHistorico(historico.ID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Hive has no defined agents. Add agents via POST /api/colmeias/:id/agentes or enable queen_managed.",
		})
		return
	}

	predefinedSpecialists := s.colmeiaService.BuildSpecialists(colmeia)

	s.Broadcast(WsMessage{
		Type:    "status",
		Message: fmt.Sprintf("🐝 Hive '%s' using %d pre-defined agent(s).", colmeia.Name, len(predefinedSpecialists)),
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
			s.Broadcast(WsMessage{Type: "error", Message: "Mission timeout reached."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":      "Mission dispatched. Follow progress via WebSocket.",
		"colmeia_id":   colmeiaID,
		"historico_id": historico.ID,
		"agents":       len(predefinedSpecialists),
		"mode":         "user_defined",
	})
}
