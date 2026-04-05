package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("erro ao ler diretorio '%s': %w", path, err)
	}

	result := fmt.Sprintf("Conteudo de '%s':\n", path)
	for _, e := range entries {
		result += fmt.Sprintf("- %s (IsDir: %t)\n", e.Name(), e.IsDir())
	}
	return result, nil
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
	var args struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	// Lendo o arquivo real do disco!
	content, err := os.ReadFile(args.Filename)
	if err != nil {
		return "", fmt.Errorf("erro ao ler arquivo '%s': %w", args.Filename, err)
	}

	return string(content), nil
}

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string {
	return "Escreve conteúdo num ficheiro. Se o ficheiro não existir, será criado. Se existir, será totalmente substituído."
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

	// Atenção: Num ambiente Sandbox (Wasm) real, verificaríamos se o path é seguro (ex: não sair do diretório permitido)
	// Como estamos a rodar no host para este teste, escrevemos diretamente.
	err := os.WriteFile(args.Filename, []byte(args.Content), 0644)
	if err != nil {
		return "", fmt.Errorf("erro ao escrever ficheiro '%s': %w", args.Filename, err)
	}

	return fmt.Sprintf("Ficheiro '%s' escrito com sucesso no disco.", args.Filename), nil
}
