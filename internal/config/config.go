package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrConfigNotFound = errors.New("arquivo de configuração não encontrado")

// Config representa as preferências do Apicultor
type Config struct {
	Language   string `json:"language"`
	Model      string `json:"model"`
	SwarmName  string `json:"swarm_name"`
	MaxNectar  int    `json:"max_nectar"`
	MaxAgents  int    `json:"max_agents"`
	Supervised bool   `json:"supervised"`
	Isolated   bool   `json:"isolated"`
}

// GetDefaultPath retorna o caminho padrão para salvar as configurações
// Usa diretórios padrão do OS: ~/.config/ no Linux/Mac, AppData\Roaming no Windows
func GetDefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		// Fallback caso não seja possível descobrir
		return "jandaira.config.json"
	}
	return filepath.Join(dir, "jandaira", "config.json")
}

// Load tenta ler o arquivo de configuração e retorna ErrConfigNotFound se ele não existir
func Load(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("erro lendo o arquivo de configuração: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("arquivo de configuração corrompido: %w", err)
	}

	return &cfg, nil
}

// Save escreve no arquivo JSON
func Save(filepathStr string, cfg *Config) error {
	dir := filepath.Dir(filepathStr)
	if err := os.MkdirAll(dir, 0755); err != nil { // Ensure directories exist
		return fmt.Errorf("erro ao criar o diretório da configuração: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao gerar JSON da configuração: %w", err)
	}

	if err := os.WriteFile(filepathStr, data, 0644); err != nil {
		return fmt.Errorf("erro ao salvar %s: %w", filepathStr, err)
	}

	return nil
}
