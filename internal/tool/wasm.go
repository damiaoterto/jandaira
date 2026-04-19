package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/damiaoterto/jandaira/internal/security"
)

type ExecuteCodeTool struct{}

func (t *ExecuteCodeTool) Name() string { return "execute_code" }

func (t *ExecuteCodeTool) Description() string {
	return `Compila código Go para WebAssembly e executa numa Sandbox isolada. Retorna stdout/stderr.

REGRAS:
1. Passe o código Go completo no campo "code" (package main + func main).
2. Use apenas stdlib Go — sem imports externos (CGO desabilitado em WASM).
3. Passe valores dinâmicos via os.Args[1:] no campo "args" — nunca hardcode valores.
4. Paths de arquivo dentro do código são relativos ao diretório de trabalho (/app).
5. NUNCA descreva o que o código faria — sempre chame esta tool para executar.`
}

func (t *ExecuteCodeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "Código fonte Go completo (package main com func main).",
			},
			"args": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Argumentos passados via os.Args[1:].",
			},
		},
		"required": []string{"code"},
	}
}

func (t *ExecuteCodeTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Code string   `json:"code"`
		Args []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos: %w", err)
	}
	if strings.TrimSpace(args.Code) == "" {
		return "", fmt.Errorf("campo 'code' obrigatório")
	}

	tmpDir, err := os.MkdirTemp("", "wasm-*")
	if err != nil {
		return "", fmt.Errorf("erro ao criar dir temporário: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(args.Code), 0644); err != nil {
		return "", fmt.Errorf("erro ao gravar código: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module sandbox\n\ngo 1.21\n"), 0644); err != nil {
		return "", fmt.Errorf("erro ao criar go.mod: %w", err)
	}

	wasmFile := filepath.Join(tmpDir, "main.wasm")
	homeDir, _ := os.UserHomeDir()

	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", wasmFile, filepath.Join(tmpDir, "main.go"))
	buildCmd.Env = append(os.Environ(),
		"GOOS=wasip1",
		"GOARCH=wasm",
		"GOCACHE="+filepath.Join(homeDir, ".cache", "go-build"),
	)
	out, err := buildCmd.CombinedOutput()
	fmt.Printf("[execute_code] go build output:\n%s\n", string(out))
	if err != nil {
		return "", fmt.Errorf("erro de compilação:\n%s", string(out))
	}

	wasmBinary, err := os.ReadFile(wasmFile)
	if err != nil {
		return "", fmt.Errorf("erro ao ler binário wasm: %w", err)
	}

	cell, err := security.NewCell(ctx)
	if err != nil {
		return "", fmt.Errorf("erro ao criar sandbox: %w", err)
	}
	defer cell.Close(ctx)

	cwd, _ := os.Getwd()
	cell.WithDirMount(cwd)

	var stdout, stderr bytes.Buffer
	cell.WithOutput(&stdout, &stderr)

	execErr := cell.Execute(ctx, wasmBinary, args.Args)

	result := stdout.String()
	if execErr != nil && !strings.Contains(execErr.Error(), "invalid magic number") {
		result += fmt.Sprintf("\n[Erro na Sandbox]: %v", execErr)
	}
	if stderr.Len() > 0 {
		result += fmt.Sprintf("\n[Stderr]: %s", stderr.String())
	}
	if result == "" {
		result = "Executou com sucesso (sem output)."
	}

	return result, nil
}
