package database

import (
	"os"
	"path/filepath"

	"github.com/damiaoterto/jandaira/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open creates (or opens) the SQLite database at dbPath, ensures the directory
// exists, and runs AutoMigrate for all registered models.
// Add new models to the AutoMigrate call as they are created.
func Open(dbPath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&model.AppConfig{}); err != nil {
		return nil, err
	}

	return db, nil
}
