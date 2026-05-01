package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/gin-gonic/gin"
)

// ─── MCP Server CRUD ───────────────────────────────────────────────────────────

// handleListMCPServers returns all configured MCP servers.
//
//	GET /api/mcp-servers
func (s *Server) handleListMCPServers(c *gin.Context) {
	servers, err := s.mcpService.List()
	if err != nil {
		log.Printf("ERROR handleListMCPServers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list MCP servers."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mcp_servers": servers, "total": len(servers)})
}

// handleCreateMCPServer creates a new MCP server configuration.
//
//	POST /api/mcp-servers
//	Body: { "name": "postgres", "transport": "stdio", "command": "npx -y @mcp/server-postgres postgres://...", "env_vars": {}, "active": true }
func (s *Server) handleCreateMCPServer(c *gin.Context) {
	var req struct {
		Name      string            `json:"name"      binding:"required"`
		Transport string            `json:"transport" binding:"required"`
		Command   string            `json:"command"`
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

	srv, err := s.mcpService.Create(req.Name, req.Transport, req.Command, req.URL, req.EnvVars, active)
	if err != nil {
		log.Printf("ERROR handleCreateMCPServer: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "MCP server created.", "mcp_server": srv})
}

// handleGetMCPServer returns a single MCP server by ID.
//
//	GET /api/mcp-servers/:id
func (s *Server) handleGetMCPServer(c *gin.Context) {
	srv, err := s.mcpService.GetByID(c.Param("id"))
	if err != nil {
		if errors.Is(err, service.ErrMCPServerNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found."})
			return
		}
		log.Printf("ERROR handleGetMCPServer id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch MCP server."})
		return
	}
	c.JSON(http.StatusOK, srv)
}

// handleUpdateMCPServer updates an existing MCP server configuration.
//
//	PUT /api/mcp-servers/:id
func (s *Server) handleUpdateMCPServer(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name      string            `json:"name"      binding:"required"`
		Transport string            `json:"transport" binding:"required"`
		Command   string            `json:"command"`
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
		log.Printf("ERROR handleUpdateMCPServer id=%s: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server updated.", "mcp_server": srv})
}

// handleDeleteMCPServer removes an MCP server configuration.
//
//	DELETE /api/mcp-servers/:id
func (s *Server) handleDeleteMCPServer(c *gin.Context) {
	if err := s.mcpService.Delete(c.Param("id")); err != nil {
		if errors.Is(err, service.ErrMCPServerNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found."})
			return
		}
		log.Printf("ERROR handleDeleteMCPServer id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete MCP server."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server deleted."})
}

// ─── Colmeia ↔ MCP Server association ─────────────────────────────────────────

// handleListColmeiaMCPServers lists the MCP servers linked to a colmeia.
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

// handleAttachMCPServerToColmeia links an MCP server to a colmeia.
//
//	POST /api/colmeias/:id/mcp-servers
//	Body: { "mcp_server_id": "uuid" }
func (s *Server) handleAttachMCPServerToColmeia(c *gin.Context) {
	colmeiaID := c.Param("id")
	var req struct {
		MCPServerID string `json:"mcp_server_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'mcp_server_id' is required."})
		return
	}

	if err := s.mcpService.AttachToColmeia(req.MCPServerID, colmeiaID); err != nil {
		if errors.Is(err, service.ErrMCPServerNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found."})
			return
		}
		log.Printf("ERROR handleAttachMCPServerToColmeia colmeia=%s server=%s: %v", colmeiaID, req.MCPServerID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to attach MCP server to hive."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server attached to hive."})
}

// handleDetachMCPServerFromColmeia removes the link between an MCP server and a colmeia.
//
//	DELETE /api/colmeias/:id/mcp-servers/:serverId
func (s *Server) handleDetachMCPServerFromColmeia(c *gin.Context) {
	colmeiaID := c.Param("id")
	serverID := c.Param("serverId")

	if err := s.mcpService.DetachFromColmeia(serverID, colmeiaID); err != nil {
		log.Printf("ERROR handleDetachMCPServerFromColmeia colmeia=%s server=%s: %v", colmeiaID, serverID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detach MCP server from hive."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP server detached from hive."})
}
