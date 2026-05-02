package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/gin-gonic/gin"
)

// ─── Colmeia-scoped MCP Server CRUD ───────────────────────────────────────────

// handleListColmeiaMCPServers returns all MCP servers belonging to a hive.
//
//	GET /api/colmeias/:id/mcp-servers
func (s *Server) handleListColmeiaMCPServers(c *gin.Context) {
	servers, err := s.mcpService.ListForColmeia(c.Param("id"))
	if err != nil {
		log.Printf("ERROR handleListColmeiaMCPServers colmeia=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list MCP servers for hive."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mcp_servers": servers, "total": len(servers)})
}

// handleCreateColmeiaMCPServer creates a new MCP server configuration inside a hive.
//
//	POST /api/colmeias/:id/mcp-servers
func (s *Server) handleCreateColmeiaMCPServer(c *gin.Context) {
	colmeiaID := c.Param("id")
	var req struct {
		Name      string            `json:"name"      binding:"required"`
		Transport string            `json:"transport" binding:"required"`
		Command   []string          `json:"command"`
		URL       string            `json:"url"`
		EnvVars   map[string]string `json:"env_vars"`
		Active    *bool             `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fields 'name' and 'transport' are required."})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	srv, err := s.mcpService.Create(colmeiaID, req.Name, req.Transport, req.Command, req.URL, req.EnvVars, active)
	if err != nil {
		log.Printf("ERROR handleCreateColmeiaMCPServer colmeia=%s: %v", colmeiaID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "MCP server created.", "mcp_server": srv})
}

// handleGetColmeiaMCPServer returns a single MCP server by ID.
//
//	GET /api/colmeias/:id/mcp-servers/:serverId
func (s *Server) handleGetColmeiaMCPServer(c *gin.Context) {
	srv, err := s.mcpService.GetByID(c.Param("serverId"))
	if err != nil {
		if errors.Is(err, service.ErrMCPServerNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found."})
			return
		}
		log.Printf("ERROR handleGetColmeiaMCPServer id=%s: %v", c.Param("serverId"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch MCP server."})
		return
	}
	c.JSON(http.StatusOK, srv)
}

// handleUpdateColmeiaMCPServer updates an existing MCP server configuration.
//
//	PUT /api/colmeias/:id/mcp-servers/:serverId
func (s *Server) handleUpdateColmeiaMCPServer(c *gin.Context) {
	id := c.Param("serverId")
	var req struct {
		Name      string            `json:"name"      binding:"required"`
		Transport string            `json:"transport" binding:"required"`
		Command   []string          `json:"command"`
		URL       string            `json:"url"`
		EnvVars   map[string]string `json:"env_vars"`
		Active    *bool             `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fields 'name' and 'transport' are required."})
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	srv, err := s.mcpService.Update(id, req.Name, req.Transport, req.Command, req.URL, req.EnvVars, active)
	if err != nil {
		if errors.Is(err, service.ErrMCPServerNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found."})
			return
		}
		log.Printf("ERROR handleUpdateColmeiaMCPServer id=%s: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server updated.", "mcp_server": srv})
}

// handleDeleteColmeiaMCPServer removes an MCP server configuration.
//
//	DELETE /api/colmeias/:id/mcp-servers/:serverId
func (s *Server) handleDeleteColmeiaMCPServer(c *gin.Context) {
	if err := s.mcpService.Delete(c.Param("serverId")); err != nil {
		if errors.Is(err, service.ErrMCPServerNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found."})
			return
		}
		log.Printf("ERROR handleDeleteColmeiaMCPServer id=%s: %v", c.Param("serverId"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete MCP server."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server deleted."})
}
