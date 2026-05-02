package repository

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/gorm"
)

var ErrMCPServerNotFound = errors.New("MCP server not found")

// MCPServerRepository defines database operations for MCPServer.
type MCPServerRepository interface {
	Create(s *model.MCPServer) error
	FindByID(id string) (*model.MCPServer, error)
	FindByColmeiaID(colmeiaID string) ([]model.MCPServer, error)
	Update(s *model.MCPServer) error
	Delete(id string) error
}

type mcpServerRepository struct{ db *gorm.DB }

// NewMCPServerRepository creates a new MCPServerRepository backed by db.
func NewMCPServerRepository(db *gorm.DB) MCPServerRepository {
	return &mcpServerRepository{db: db}
}

func (r *mcpServerRepository) Create(s *model.MCPServer) error {
	return r.db.Create(s).Error
}

func (r *mcpServerRepository) FindByID(id string) (*model.MCPServer, error) {
	var s model.MCPServer
	if err := r.db.First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMCPServerNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *mcpServerRepository) FindByColmeiaID(colmeiaID string) ([]model.MCPServer, error) {
	var servers []model.MCPServer
	if err := r.db.Where("colmeia_id = ?", colmeiaID).Order("name ASC").Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *mcpServerRepository) Update(s *model.MCPServer) error {
	return r.db.Save(s).Error
}

func (r *mcpServerRepository) Delete(id string) error {
	return r.db.Delete(&model.MCPServer{ID: id}).Error
}
