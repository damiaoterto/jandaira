package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, argsJSON string) (string, error)
}

type ListDirectoryTool struct{}

func (t *ListDirectoryTool) Name() string { return "list_directory" }

func (t *ListDirectoryTool) Description() string {
	return "Lista os ficheiros e pastas de um diretório específico."
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
	// Simplificação: Num ambiente real, fariamos o parse do argsJSON para extrair o "path"
	// e validaríamos contra o Sandbox/VFS para garantir segurança.
	path := "." // Hardcoded para o diretório atual neste protótipo seguro

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("erro ao ler diretório: %w", err)
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Conteúdo de %s:\n", path)
	for _, e := range entries {
		fmt.Fprintf(&result, "- %s (IsDir: %t)\n", e.Name(), e.IsDir())
	}
	return result.String(), nil
}

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Lê o conteúdo de um ficheiro de texto."
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
	// Para simplificar o protótipo, vamos apenas ler o ficheiro (requer parsing de JSON na versão final)
	// Aqui a Rainha validaria se o ficheiro está no Sandbox!
	return "Conteúdo simulado ou lógica de leitura real", nil
}
