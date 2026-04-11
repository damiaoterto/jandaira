package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/damiaoterto/jandaira/internal/security"
)

type ExecuteCodeTool struct{}

func (t *ExecuteCodeTool) Name() string { return "execute_code" }

func (t *ExecuteCodeTool) Description() string {
	return "Compila um ficheiro Go para WebAssembly e executa-o numa Sandbox totalmente isolada. Retorna a saída (stdout/stderr)."
}

func (t *ExecuteCodeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filename": map[string]any{
				"type":        "string",
				"description": "O caminho do ficheiro Go a ser executado (ex: 'calculadora.go').",
			},
		},
		"required": []string{"filename"},
	}
}

func (t *ExecuteCodeTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("erro ao ler argumentos JSON: %w", err)
	}

	absFilename, err := resolvePath(args.Filename)
	if err != nil {
		return "", err
	}

	// 1. Compilar para WASI/WASM (Seguro, apenas usa o compilador)
	wasmFilename := strings.TrimSuffix(absFilename, ".go") + ".wasm"

	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", wasmFilename, absFilename)
	buildCmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm") // Força a compilação para WebAssembly

	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("erro de compilação:\n%s\nErro: %w", string(buildOutput), err)
	}
	// Limpa o binário .wasm quando a função terminar
	defer os.Remove(wasmFilename)

	// 2. Ler o binário compilado
	wasmBinary, err := os.ReadFile(wasmFilename)
	if err != nil {
		return "", fmt.Errorf("erro ao ler binário Wasm: %w", err)
	}

	// 3. Criar a Célula Sandbox (Isolamento total com wazero)
	cell, err := security.NewCell(ctx, []string{"OPENAI_API_KEY"})
	if err != nil {
		return "", fmt.Errorf("erro ao criar célula Wasm: %w", err)
	}
	defer cell.Close(ctx)

	// 4. Capturar Stdout e Stderr da Sandbox
	var stdoutBuf, stderrBuf bytes.Buffer
	cell.WithOutput(&stdoutBuf, &stderrBuf)

	// 5. Executar o código isolado
	err = cell.Execute(ctx, wasmBinary, nil)

	// 6. Formatar o resultado para a IA
	result := stdoutBuf.String()
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid magic number") {
			result += "\n[Sucesso de Compilação]: O código fonte compilou perfeitamente e não tem erros de sintaxe!\n"
			result += "Contudo, não pôde ser executado pela Sandbox pois o ficheiro não é um programa final (não tem 'package main' e 'func main()'). Se isto for uma biblioteca, podes considerar a alteração válida!"
		} else {
			result += fmt.Sprintf("\n[Erro de Execução na Sandbox]: %v", err)
		}
	}
	if stderrBuf.Len() > 0 {
		result += fmt.Sprintf("\n[Stderr]:\n%s", stderrBuf.String())
	}

	// Se não imprimiu nada, devolvemos uma mensagem para a IA saber que rodou.
	if result == "" {
		result = "O programa executou com sucesso na Sandbox, mas não imprimiu nada."
	}

	return result, nil
}
