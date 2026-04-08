package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, argsJSON string) (string, error)
}

func resolvePath(inputPath string) (string, error) {
	if strings.HasPrefix(inputPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			inputPath = strings.Replace(inputPath, "~", home, 1)
		}
	}
	
	if filepath.IsAbs(inputPath) {
		return inputPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("não foi possível obter o diretório atual (CWD): %w", err)
	}
	return filepath.Join(cwd, inputPath), nil
}

type ListDirectoryTool struct{}

func (t *ListDirectoryTool) Name() string { return "list_directory" }

func (t *ListDirectoryTool) Description() string {
	return "Lista os ficheiros e pastas de um diretório específico, sempre preenchendo o caminho de forma resolvida a partir do diretório atual."
}

func (t *ListDirectoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "O caminho do diretório a listar (ex: '.', './pkg')",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ListDirectoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	path := args.Path
	if path == "" {
		path = "."
	}

	absPath, err := resolvePath(path)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return "", fmt.Errorf("erro ao ler diretorio '%s': %w", absPath, err)
	}

	result := fmt.Sprintf("Conteudo de '%s':\n", absPath)
	for _, e := range entries {
		result += fmt.Sprintf("- %s (IsDir: %t)\n", e.Name(), e.IsDir())
	}
	return result, nil
}

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Lê o conteúdo de um ficheiro de texto, resolvendo-o a partir do CWD."
}

func (t *ReadFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filename": map[string]any{
				"type":        "string",
				"description": "O nome do ficheiro a ler",
			},
		},
		"required": []string{"filename"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	absPath, err := resolvePath(args.Filename)
	if err != nil {
		return "", err
	}

	// Lendo o arquivo real do disco!
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("erro ao ler arquivo '%s': %w", absPath, err)
	}

	return string(content), nil
}

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string {
	return "Escreve conteúdo num ficheiro, resolvendo-o a partir do CWD. Se o ficheiro não existir, será criado. Se existir, será totalmente substituído."
}

func (t *WriteFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "O caminho completo do ficheiro a criar ou modificar (ex: 'hello.go', 'docs/readme.md')",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "O conteúdo completo que será escrito no ficheiro.",
			},
		},
		"required": []string{"filename", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	absPath, err := resolvePath(args.Filename)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(absPath, []byte(args.Content), 0644)
	if err != nil {
		return "", fmt.Errorf("erro ao escrever ficheiro '%s': %w", absPath, err)
	}

	return fmt.Sprintf("Ficheiro '%s' escrito com sucesso no disco.", absPath), nil
}

type CreateDirectoryTool struct{}

func (t *CreateDirectoryTool) Name() string { return "create_directory" }

func (t *CreateDirectoryTool) Description() string {
	return "Cria um novo diretório (e diretórios pai se não existirem), resolvendo o caminho a partir do CWD."
}

func (t *CreateDirectoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "O caminho completo do diretório a criar (ex: 'docs/images')",
			},
		},
		"required": []string{"path"},
	}
}

func (t *CreateDirectoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	absPath, err := resolvePath(args.Path)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(absPath, 0755)
	if err != nil {
		return "", fmt.Errorf("erro ao criar diretório '%s': %w", absPath, err)
	}

	return fmt.Sprintf("Diretório '%s' criado com sucesso.", absPath), nil
}
