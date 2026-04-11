package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleListTools returns all tools registered in the Queen.
//
//	GET /api/tools
func (s *Server) handleListTools(c *gin.Context) {
	toolsList := make([]map[string]any, 0)
	for name, tool := range s.Queen.Tools {
		toolsList = append(toolsList, map[string]any{
			"name":        name,
			"description": tool.Description(),
			"parameters":  tool.Parameters(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"tools": toolsList})
}

// handleListAgents returns all agents registered in the Queen.
//
//	GET /api/agents
func (s *Server) handleListAgents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"agents": []map[string]any{}})
}
