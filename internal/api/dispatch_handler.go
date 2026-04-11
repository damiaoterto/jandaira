package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleDispatch assembles a swarm and runs the workflow asynchronously.
// Progress is streamed to connected WebSocket clients.
//
//	POST /api/dispatch
//	Body: { "goal": "...", "group_id": "opcional" }
func (s *Server) handleDispatch(c *gin.Context) {
	var req struct {
		Goal    string `json:"goal" binding:"required"`
		GroupID string `json:"group_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'goal' is required."})
		return
	}

	if req.GroupID == "" {
		if cfg, err := s.configService.Load(); err == nil {
			req.GroupID = cfg.SwarmName
		} else {
			req.GroupID = "enxame-alfa"
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		s.Broadcast(WsMessage{Type: "status", Message: "🚀 Queen received the goal and is starting the swarm..."})

		maxWorkers := 3
		if cfg, err := s.configService.Load(); err == nil && cfg.MaxAgents > 0 {
			maxWorkers = cfg.MaxAgents
		}

		dynamicPipeline, err := s.Queen.AssembleSwarm(ctx, req.Goal, maxWorkers)
		if err != nil {
			s.Broadcast(WsMessage{Type: "error", Message: fmt.Sprintf("Error planning the swarm: %v", err)})
			return
		}

		resultChan, errChan := s.Queen.DispatchWorkflow(ctx, req.GroupID, req.Goal, dynamicPipeline)

		select {
		case res := <-resultChan:
			s.Broadcast(WsMessage{Type: "result", Message: res})
		case err := <-errChan:
			s.Broadcast(WsMessage{Type: "error", Message: err.Error()})
		case <-ctx.Done():
			s.Broadcast(WsMessage{Type: "error", Message: "Mission timeout reached."})
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "Mission dispatched to the swarm. Follow progress via WebSocket."})
}
