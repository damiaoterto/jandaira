package config

import (
	"os"
	"path/filepath"

	"github.com/damiaoterto/jandaira/internal/model"
)

// Config is a re-export of model.AppConfig so callers keep a stable import path.
type Config = model.AppConfig

// GetDefaultPath returns the default path for the SQLite database file.
// Uses the OS standard config directory: ~/.config/ on Linux/Mac,
// AppData\Roaming on Windows.
func GetDefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "jandaira.db"
	}
	return filepath.Join(dir, "jandaira", "jandaira.db")
}
