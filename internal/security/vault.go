package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Vault struct {
	secretsFile string
	masterKey   []byte
}

func InitVault(dir string) (*Vault, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	keyFile := filepath.Join(dir, "master.key")
	secretsFile := filepath.Join(dir, "vault.enc")

	var masterKey []byte

	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		key, err := GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("erro ao gerar chave mestra: %w", err)
		}
		if err := os.WriteFile(keyFile, key, 0600); err != nil {
			return nil, fmt.Errorf("erro ao salvar chave mestra: %w", err)
		}
		masterKey = key
	} else {
		masterKey, err = os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("erro ao ler chave mestra: %w", err)
		}
	}

	return &Vault{
		secretsFile: secretsFile,
		masterKey:   masterKey,
	}, nil
}

func (v *Vault) SaveSecret(key, value string) error {
	secrets := make(map[string]string)

	if data, err := os.ReadFile(v.secretsFile); err == nil {
		decrypted, err := Open(v.masterKey, string(data))
		if err == nil {
			_ = json.Unmarshal([]byte(decrypted), &secrets)
		}
	}

	secrets[key] = value

	jsonBytes, _ := json.Marshal(secrets)
	encrypted, err := Seal(v.masterKey, string(jsonBytes))
	if err != nil {
		return err
	}

	return os.WriteFile(v.secretsFile, []byte(encrypted), 0600)
}

func (v *Vault) GetSecret(key string) (string, error) {
	data, err := os.ReadFile(v.secretsFile)
	if err != nil {
		return "", fmt.Errorf("cofre não encontrado ou vazio")
	}

	decrypted, err := Open(v.masterKey, string(data))
	if err != nil {
		return "", fmt.Errorf("falha ao descriptografar o cofre")
	}

	secrets := make(map[string]string)
	if err := json.Unmarshal([]byte(decrypted), &secrets); err != nil {
		return "", err
	}

	val, exists := secrets[key]
	if !exists {
		return "", fmt.Errorf("segredo '%s' não encontrado", key)
	}

	return val, nil
}

func GetDefaultVaultDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, "jandaira", ".secrets")
}
