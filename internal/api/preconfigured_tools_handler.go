package api

import (
	"fmt"
	"net/http"

	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/gin-gonic/gin"
)

// preconfiguredTools is the registry of tools that require a stored API token.
// To add a new tool, insert its name and description here.
var preconfiguredTools = map[string]string{
	"firecrawl": "Web scraping and crawling via Firecrawl API",
}

func vaultKeyForTool(name string) string {
	return name + "_api_key"
}

// handleListPreconfiguredTools returns all supported tools with their configuration status.
//
//	GET /api/tools/preconfigured
func (s *Server) handleListPreconfiguredTools(c *gin.Context) {
	vault, err := security.InitVault(security.GetDefaultVaultDir())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to access vault: " + err.Error()})
		return
	}

	tools := make([]map[string]interface{}, 0, len(preconfiguredTools))
	for name, description := range preconfiguredTools {
		_, keyErr := vault.GetSecret(vaultKeyForTool(name))
		tools = append(tools, map[string]interface{}{
			"name":        name,
			"description": description,
			"configured":  keyErr == nil,
		})
	}

	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

// handleGetPreconfiguredToolStatus returns the configuration status of a specific tool.
//
//	GET /api/tools/preconfigured/:tool
func (s *Server) handleGetPreconfiguredToolStatus(c *gin.Context) {
	name := c.Param("tool")
	if _, ok := preconfiguredTools[name]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("unknown tool: %s", name)})
		return
	}

	vault, err := security.InitVault(security.GetDefaultVaultDir())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to access vault: " + err.Error()})
		return
	}

	_, err = vault.GetSecret(vaultKeyForTool(name))
	c.JSON(http.StatusOK, gin.H{
		"tool":        name,
		"description": preconfiguredTools[name],
		"configured":  err == nil,
	})
}

// handleConfigurePreconfiguredTool saves an API key for the given tool to the vault.
//
//	POST /api/tools/preconfigured/:tool
func (s *Server) handleConfigurePreconfiguredTool(c *gin.Context) {
	name := c.Param("tool")
	if _, ok := preconfiguredTools[name]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("unknown tool: %s", name)})
		return
	}

	var req struct {
		APIKey string `json:"api_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
		return
	}

	vault, err := security.InitVault(security.GetDefaultVaultDir())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to access vault: " + err.Error()})
		return
	}

	if err := vault.SaveSecret(vaultKeyForTool(name), req.APIKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save API key: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s API key configured successfully.", name)})
}

// handleDeletePreconfiguredToolConfig removes the API key for the given tool from the vault.
//
//	DELETE /api/tools/preconfigured/:tool
func (s *Server) handleDeletePreconfiguredToolConfig(c *gin.Context) {
	name := c.Param("tool")
	if _, ok := preconfiguredTools[name]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("unknown tool: %s", name)})
		return
	}

	vault, err := security.InitVault(security.GetDefaultVaultDir())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to access vault: " + err.Error()})
		return
	}

	if err := vault.DeleteSecret(vaultKeyForTool(name)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove API key: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s API key removed.", name)})
}
