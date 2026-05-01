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
	FindAll() ([]model.MCPServer, error)
	Update(s *model.MCPServer) error
	Delete(id string) error

	// Many-to-many helpers for Colmeia ↔ MCPServer.
	AttachToColmeia(serverID, colmeiaID string) error
	DetachFromColmeia(serverID, colmeiaID string) error
	FindByColmeiaID(colmeiaID string) ([]model.MCPServer, error)
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

func (r *mcpServerRepository) FindAll() ([]model.MCPServer, error) {
	var servers []model.MCPServer
	if err := r.db.Order("name ASC").Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *mcpServerRepository) Update(s *model.MCPServer) error {
	return r.db.Save(s).Error
}

func (r *mcpServerRepository) Delete(id string) error {
	server := model.MCPServer{ID: id}
	// Remove associações many-to-many antes de deletar para evitar erro de Foreign Key
	if err := r.db.Model(&server).Association("Colmeias").Clear(); err != nil {
		return err
	}
	return r.db.Delete(&server).Error
}

func (r *mcpServerRepository) AttachToColmeia(serverID, colmeiaID string) error {
	colmeia := model.Colmeia{ID: colmeiaID}
	server := model.MCPServer{ID: serverID}
	return r.db.Model(&colmeia).Association("MCPServers").Append(&server)
}

func (r *mcpServerRepository) DetachFromColmeia(serverID, colmeiaID string) error {
	colmeia := model.Colmeia{ID: colmeiaID}
	server := model.MCPServer{ID: serverID}
	return r.db.Model(&colmeia).Association("MCPServers").Delete(&server)
}

func (r *mcpServerRepository) FindByColmeiaID(colmeiaID string) ([]model.MCPServer, error) {
	var colmeia model.Colmeia
	if err := r.db.Preload("MCPServers").First(&colmeia, "id = ?", colmeiaID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	return colmeia.MCPServers, nil
}
