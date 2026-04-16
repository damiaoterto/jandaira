package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/gin-gonic/gin"
)

// handleListSessions returns all sessions ordered by creation date (newest first).
// Agents are not included to keep the payload small — use GET /:id for the full graph.
//
//	GET /api/sessions
func (s *Server) handleListSessions(c *gin.Context) {
	sessions, err := s.sessionService.ListSessions()
	if err != nil {
		log.Printf("ERROR handleListSessions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar sessões."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
}

// handleCreateSession starts a new session.
//
//	POST /api/sessions
//	Body: { "name": "opcional", "goal": "objetivo da missão" }
func (s *Server) handleCreateSession(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		Goal string `json:"goal" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'goal' é obrigatório."})
		return
	}

	session, err := s.sessionService.Create(req.Name, req.Goal)
	if err != nil {
		log.Printf("ERROR handleCreateSession: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar sessão."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Sessão criada. Use POST /api/sessions/:id/dispatch para iniciar o enxame.",
		"session": session,
	})
}

// handleGetSession returns a session with all its agents.
//
//	GET /api/sessions/:id
func (s *Server) handleGetSession(c *gin.Context) {
	session, err := s.sessionService.GetSession(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sessão não encontrada."})
			return
		}
		log.Printf("ERROR handleGetSession id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar sessão."})
		return
	}
	c.JSON(http.StatusOK, session)
}

// handleDeleteSession removes a session and all its agents.
//
//	DELETE /api/sessions/:id
func (s *Server) handleDeleteSession(c *gin.Context) {
	id := c.Param("id")
	if err := s.sessionService.DeleteSession(id); err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sessão não encontrada."})
			return
		}
		log.Printf("ERROR handleDeleteSession id=%s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deletar sessão."})
		return
	}

	// Remove workspace files created during document upload.
	sessionWorkspace := filepath.Join(workspaceDir, id)
	if err := os.RemoveAll(sessionWorkspace); err != nil {
		log.Printf("WARN handleDeleteSession cleanup workspace id=%s: %v", id, err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sessão e agentes removidos com sucesso."})
}

// handleListSessionAgents returns all agents belonging to a session.
//
//	GET /api/sessions/:id/agents
func (s *Server) handleListSessionAgents(c *gin.Context) {
	agents, err := s.sessionService.ListAgents(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sessão não encontrada."})
			return
		}
		log.Printf("ERROR handleListSessionAgents id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar agentes."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents, "total": len(agents)})
}

// handleSessionDispatch assembles the swarm for a session, persists each
// specialist as an Agent, and dispatches the workflow asynchronously.
// Progress is streamed to connected WebSocket clients.
//
//	POST /api/sessions/:id/dispatch
//	Body: { "goal": "opcional — substitui o goal da sessão" }
func (s *Server) handleSessionDispatch(c *gin.Context) {
	sessionID := c.Param("id")

	session, err := s.sessionService.GetSession(sessionID)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sessão não encontrada."})
			return
		}
		log.Printf("ERROR handleSessionDispatch GetSession id=%s: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar sessão."})
		return
	}

	if session.Status == model.SessionStatusCompleted || session.Status == model.SessionStatusFailed {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Sessão já finalizada com status '%s'. Crie uma nova sessão.", session.Status),
		})
		return
	}

	var req struct {
		Goal string `json:"goal"`
	}
	_ = c.ShouldBindJSON(&req)

	goal := req.Goal
	if goal == "" {
		goal = session.Goal
	}
	if goal == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nenhum objetivo definido. Forneça 'goal' no body ou ao criar a sessão."})
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

	// Inject uploaded document paths into the goal so the Queen instructs
	// agents to read the correct files from the workspace.
	docDir := filepath.Join(workspaceDir, sessionID)
	if entries, err := os.ReadDir(docDir); err == nil && len(entries) > 0 {
		var paths []string
		for _, e := range entries {
			if !e.IsDir() {
				paths = append(paths, filepath.Join(docDir, e.Name()))
			}
		}
		if len(paths) > 0 {
			goal = fmt.Sprintf(
				"%s\n\nDocumentos enviados pelo usuário (use read_file com estes caminhos exatos):\n%s",
				goal,
				strings.Join(paths, "\n"),
			)
		}
	}

	// Assemble swarm synchronously so we can register agents before returning.
	assembleCtx, assembleCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer assembleCancel()

	specialists, err := s.Queen.AssembleSwarm(assembleCtx, goal, maxWorkers)
	if err != nil {
		log.Printf("ERROR handleSessionDispatch AssembleSwarm session=%s: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Falha ao planejar o enxame: %v", err)})
		return
	}

	// Persist each specialist as an Agent record and notify clients.
	for _, spec := range specialists {
		agent, err := s.sessionService.AddAgent(sessionID, spec.Name, "specialist")
		if err != nil {
			// Non-fatal: log and continue.
			s.Broadcast(WsMessage{Type: "status", Message: fmt.Sprintf("⚠️ Falha ao registrar agente '%s': %v", spec.Name, err)})
			continue
		}
		s.Broadcast(WsMessage{Type: "agent_created", AgentData: agent})
	}

	// Wire AgentChangeFunc to track which agent is actively working.
	// Note: this overwrites any previous callback, so concurrent dispatches
	// from different sessions should not be issued simultaneously.
	s.Queen.AgentChangeFunc = func(agentName string) {
		_ = s.sessionService.UpdateAgentStatusByName(sessionID, agentName, model.AgentStatusWorking)
		s.Broadcast(WsMessage{Type: "agent_change", Agent: agentName})
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		s.Broadcast(WsMessage{
			Type:    "status",
			Message: fmt.Sprintf("🚀 Sessão %s iniciada com %d agente(s). Acompanhe pelo WebSocket.", sessionID, len(specialists)),
		})

		resultChan, errChan := s.Queen.DispatchWorkflow(ctx, groupID, goal, specialists)

		select {
		case result := <-resultChan:
			_ = s.sessionService.CompleteSession(sessionID, result)
			s.Broadcast(WsMessage{Type: "result", Message: result})
		case dispatchErr := <-errChan:
			_ = s.sessionService.FailSession(sessionID)
			s.Broadcast(WsMessage{Type: "error", Message: dispatchErr.Error()})
		case <-ctx.Done():
			_ = s.sessionService.FailSession(sessionID)
			s.Broadcast(WsMessage{Type: "error", Message: "Tempo limite da missão atingido."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Missão despachada. Acompanhe o progresso via WebSocket.",
		"session_id": sessionID,
		"agents":     len(specialists),
	})
}
