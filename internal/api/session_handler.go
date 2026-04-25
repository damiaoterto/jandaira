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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
}

// handleCreateSession starts a new session.
//
//	POST /api/sessions
//	Body: { "name": "optional", "goal": "mission objective" }
func (s *Server) handleCreateSession(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		Goal string `json:"goal" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'goal' is required."})
		return
	}

	session, err := s.sessionService.Create(req.Name, req.Goal)
	if err != nil {
		log.Printf("ERROR handleCreateSession: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Session created. Use POST /api/sessions/:id/dispatch to start the swarm.",
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found."})
			return
		}
		log.Printf("ERROR handleGetSession id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch session."})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found."})
			return
		}
		log.Printf("ERROR handleDeleteSession id=%s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session."})
		return
	}

	// Remove workspace files created during document upload.
	sessionWorkspace := filepath.Join(workspaceRoot(), "sessions",id)
	if err := os.RemoveAll(sessionWorkspace); err != nil {
		log.Printf("WARN handleDeleteSession cleanup workspace id=%s: %v", id, err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Session and agents deleted successfully."})
}

// handleListSessionAgents returns all agents belonging to a session.
//
//	GET /api/sessions/:id/agents
func (s *Server) handleListSessionAgents(c *gin.Context) {
	agents, err := s.sessionService.ListAgents(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found."})
			return
		}
		log.Printf("ERROR handleListSessionAgents id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents, "total": len(agents)})
}

// handleSessionDispatch assembles the swarm for a session, persists each
// specialist as an Agent, and dispatches the workflow asynchronously.
// Progress is streamed to connected WebSocket clients.
//
//	POST /api/sessions/:id/dispatch
//	Body: { "goal": "optional — overrides the session goal" }
func (s *Server) handleSessionDispatch(c *gin.Context) {
	sessionID := c.Param("id")

	session, err := s.sessionService.GetSession(sessionID)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found."})
			return
		}
		log.Printf("ERROR handleSessionDispatch GetSession id=%s: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch session."})
		return
	}

	if session.Status == model.SessionStatusCompleted || session.Status == model.SessionStatusFailed {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Session already finished with status '%s'. Create a new session.", session.Status),
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "No goal defined. Provide 'goal' in the request body or when creating the session."})
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
	docDir := filepath.Join(workspaceRoot(), "sessions",sessionID)
	if entries, err := os.ReadDir(docDir); err == nil && len(entries) > 0 {
		var paths []string
		for _, e := range entries {
			if !e.IsDir() {
				paths = append(paths, filepath.Join(docDir, e.Name()))
			}
		}
		if len(paths) > 0 {
			goal = fmt.Sprintf(
				"%s\n\nDocuments uploaded by user (use read_file with these exact paths):\n%s",
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to plan the swarm: %v", err)})
		return
	}

	// Persist each specialist as an Agent record and notify clients.
	for _, spec := range specialists {
		agent, err := s.sessionService.AddAgent(sessionID, spec.Name, "specialist")
		if err != nil {
			// Non-fatal: log and continue.
			s.Broadcast(WsMessage{Type: "status", Message: fmt.Sprintf("⚠️ Failed to register agent '%s': %v", spec.Name, err)})
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
			Message: fmt.Sprintf("🚀 Session %s started with %d agent(s). Follow progress via WebSocket.", sessionID, len(specialists)),
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
			s.Broadcast(WsMessage{Type: "error", Message: "Mission timeout reached."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Mission dispatched. Follow progress via WebSocket.",
		"session_id": sessionID,
		"agents":     len(specialists),
	})
}
