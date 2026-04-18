package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/damiaoterto/jandaira/internal/service"
	"github.com/gin-gonic/gin"
)

// ─── Skills (CRUD global) ──────────────────────────────────────────────────────

// handleListSkills returns all registered skills.
//
//	GET /api/skills
func (s *Server) handleListSkills(c *gin.Context) {
	skills, err := s.skillService.ListSkills()
	if err != nil {
		log.Printf("ERROR handleListSkills: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar skills."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills, "total": len(skills)})
}

// handleCreateSkill creates a new skill.
//
//	POST /api/skills
//	Body: { "name": "...", "description": "...", "instructions": "...", "allowed_tools": [...] }
func (s *Server) handleCreateSkill(c *gin.Context) {
	var req struct {
		Name         string   `json:"name"          binding:"required"`
		Description  string   `json:"description"`
		Instructions string   `json:"instructions"`
		AllowedTools []string `json:"allowed_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'name' é obrigatório."})
		return
	}

	skill, err := s.skillService.CreateSkill(req.Name, req.Description, req.Instructions, req.AllowedTools)
	if err != nil {
		log.Printf("ERROR handleCreateSkill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar skill."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Skill criada com sucesso.", "skill": skill})
}

// handleGetSkill returns a single skill by ID.
//
//	GET /api/skills/:id
func (s *Server) handleGetSkill(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de skill inválido."})
		return
	}

	skill, err := s.skillService.GetSkill(uint(id))
	if err != nil {
		if errors.Is(err, service.ErrSkillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill não encontrada."})
			return
		}
		log.Printf("ERROR handleGetSkill id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar skill."})
		return
	}
	c.JSON(http.StatusOK, skill)
}

// handleUpdateSkill updates a skill.
//
//	PUT /api/skills/:id
func (s *Server) handleUpdateSkill(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de skill inválido."})
		return
	}
	var req struct {
		Name         string   `json:"name"          binding:"required"`
		Description  string   `json:"description"`
		Instructions string   `json:"instructions"`
		AllowedTools []string `json:"allowed_tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'name' é obrigatório."})
		return
	}

	skill, err := s.skillService.UpdateSkill(uint(id), req.Name, req.Description, req.Instructions, req.AllowedTools)
	if err != nil {
		if errors.Is(err, service.ErrSkillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill não encontrada."})
			return
		}
		log.Printf("ERROR handleUpdateSkill id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar skill."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Skill atualizada.", "skill": skill})
}

// handleDeleteSkill deletes a skill.
//
//	DELETE /api/skills/:id
func (s *Server) handleDeleteSkill(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de skill inválido."})
		return
	}
	if err := s.skillService.DeleteSkill(uint(id)); err != nil {
		if errors.Is(err, service.ErrSkillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill não encontrada."})
			return
		}
		log.Printf("ERROR handleDeleteSkill id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deletar skill."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Skill removida com sucesso."})
}

// ─── Colmeia ↔ Skills ─────────────────────────────────────────────────────────

// handleListColmeiaSkills returns all skills attached to a colmeia.
//
//	GET /api/colmeias/:id/skills
func (s *Server) handleListColmeiaSkills(c *gin.Context) {
	skills, err := s.skillService.ListColmeiaSkills(c.Param("id"))
	if err != nil {
		log.Printf("ERROR handleListColmeiaSkills id=%s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar skills da colmeia."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills, "total": len(skills)})
}

// handleAttachSkillToColmeia attaches an existing skill to a colmeia.
//
//	POST /api/colmeias/:id/skills
//	Body: { "skill_id": 1 }
func (s *Server) handleAttachSkillToColmeia(c *gin.Context) {
	colmeiaID := c.Param("id")
	var req struct {
		SkillID uint `json:"skill_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'skill_id' é obrigatório."})
		return
	}

	if err := s.skillService.AttachSkillToColmeia(req.SkillID, colmeiaID); err != nil {
		if errors.Is(err, service.ErrSkillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill não encontrada."})
			return
		}
		log.Printf("ERROR handleAttachSkillToColmeia colmeia=%s skill=%d: %v", colmeiaID, req.SkillID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao associar skill."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Skill associada à colmeia."})
}

// handleDetachSkillFromColmeia removes a skill from a colmeia.
//
//	DELETE /api/colmeias/:id/skills/:skillId
func (s *Server) handleDetachSkillFromColmeia(c *gin.Context) {
	colmeiaID := c.Param("id")
	skillID, err := strconv.ParseUint(c.Param("skillId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de skill inválido."})
		return
	}

	if err := s.skillService.DetachSkillFromColmeia(uint(skillID), colmeiaID); err != nil {
		if errors.Is(err, service.ErrSkillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill não encontrada."})
			return
		}
		log.Printf("ERROR handleDetachSkillFromColmeia colmeia=%s skill=%d: %v", colmeiaID, skillID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao remover skill da colmeia."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Skill removida da colmeia."})
}

// ─── AgenteColmeia ↔ Skills ───────────────────────────────────────────────────

// handleListAgenteSkills returns all skills attached to an agent.
//
//	GET /api/colmeias/:id/agentes/:agentId/skills
func (s *Server) handleListAgenteSkills(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de agente inválido."})
		return
	}

	skills, err := s.skillService.ListAgenteSkills(uint(agenteID))
	if err != nil {
		log.Printf("ERROR handleListAgenteSkills agente=%d: %v", agenteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao listar skills do agente."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills, "total": len(skills)})
}

// handleAttachSkillToAgente attaches a skill to a pre-defined agent.
//
//	POST /api/colmeias/:id/agentes/:agentId/skills
//	Body: { "skill_id": 1 }
func (s *Server) handleAttachSkillToAgente(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de agente inválido."})
		return
	}
	var req struct {
		SkillID uint `json:"skill_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O campo 'skill_id' é obrigatório."})
		return
	}

	if err := s.skillService.AttachSkillToAgente(req.SkillID, uint(agenteID)); err != nil {
		if errors.Is(err, service.ErrSkillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill não encontrada."})
			return
		}
		log.Printf("ERROR handleAttachSkillToAgente agente=%d skill=%d: %v", agenteID, req.SkillID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao associar skill ao agente."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Skill associada ao agente."})
}

// handleDetachSkillFromAgente removes a skill from a pre-defined agent.
//
//	DELETE /api/colmeias/:id/agentes/:agentId/skills/:skillId
func (s *Server) handleDetachSkillFromAgente(c *gin.Context) {
	agenteID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de agente inválido."})
		return
	}
	skillID, err := strconv.ParseUint(c.Param("skillId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de skill inválido."})
		return
	}

	if err := s.skillService.DetachSkillFromAgente(uint(skillID), uint(agenteID)); err != nil {
		if errors.Is(err, service.ErrSkillNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill não encontrada."})
			return
		}
		log.Printf("ERROR handleDetachSkillFromAgente agente=%d skill=%d: %v", agenteID, skillID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao remover skill do agente."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Skill removida do agente."})
}
