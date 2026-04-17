package database

import (
	"os"
	"path/filepath"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open creates (or opens) the SQLite database at dbPath, ensures the parent
// directory exists, enables foreign-key enforcement, and runs AutoMigrate for
// all registered models.
//
// To register a new model, add it to the AutoMigrate call below.
func Open(dbPath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Enable SQLite foreign-key constraints so ON DELETE CASCADE works.
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&model.AppConfig{},
		&model.Session{},
		&model.Agent{},
		&model.Colmeia{},
		&model.AgenteColmeia{},
		&model.HistoricoDespacho{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
