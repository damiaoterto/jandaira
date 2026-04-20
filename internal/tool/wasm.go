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
	return `Compiles Go code to WebAssembly and executes it in an isolated sandbox. Returns stdout/stderr.

MANDATORY RULES:
1. Pass complete Go source in "code": must include "package main" and "func main()".
2. stdlib only — no external imports (CGO disabled in WASM).
3. Pass dynamic values via os.Args[1:] in "args" — never hardcode variable data.
4. File paths inside the code are relative to the mounted working directory (/app).
5. NEVER describe what the code would do — ALWAYS call this tool to execute it.

CANONICAL STRUCTURE (always use as base):

  package main

  import (
      "errors"
      "fmt"
      "os"
  )

  func main() {
      if err := run(os.Args[1:]); err != nil {
          fmt.Fprintf(os.Stderr, "error: %v\n", err)
          os.Exit(1)
      }
  }

  // run holds all logic — never call os.Exit here, return error instead.
  func run(args []string) error {
      if len(args) < 1 {
          return errors.New("usage: program <arg1>")
      }
      // implementation
      return nil
  }

ERROR HANDLING (mandatory):
- Handle ALL errors; never use _ on I/O or parsing errors.
- Propagate with context: fmt.Errorf("operation X: %w", err)
- Sentinel errors: var ErrNotFound = errors.New("not found")
- Check type: errors.Is(err, ErrNotFound) / errors.As(err, &target)
- os.Exit(1) only in main(); all other functions return error.

CONCURRENCY (when needed):
- Pass context.Context to every blocking operation; always respect ctx.Done().
- Every goroutine must have a defined lifetime — use sync.WaitGroup or an error channel.
- Buffered error channel: errCh := make(chan error, numWorkers)
- Worker pattern with cancellation:

  func worker(ctx context.Context, jobs <-chan Job, results chan<- Result, errCh chan<- error) {
      for {
          select {
          case <-ctx.Done():
              errCh <- fmt.Errorf("worker cancelled: %w", ctx.Err())
              return
          case job, ok := <-jobs:
              if !ok {
                  return // jobs channel closed, clean exit
              }
              res, err := process(ctx, job)
              if err != nil {
                  errCh <- fmt.Errorf("process job %v: %w", job.ID, err)
                  return
              }
              results <- res
          }
      }
  }

- Use sync.Mutex for shared state; sync.RWMutex when reads dominate.
- Prefer channels for communication; Mutex for state protection.
- Write as if -race is always active — no unsynchronised shared writes.

INTERFACES AND DESIGN:
- Define interfaces on the consumer side, not the producer.
- Keep interfaces small and focused (1-3 methods); compose via embedding.
- Accept interfaces, return concrete types.
- Functional options for structs with multiple optional parameters:

  type Option func(*Config)
  func WithTimeout(d time.Duration) Option { return func(c *Config) { c.timeout = d } }

COLLECTIONS AND STRINGS:
- Slices with known capacity: make([]T, 0, n)
- Maps: initialise with make(map[K]V); check existence with v, ok := m[k]
- String concatenation in loops: strings.Builder — never += in a loop
- Numeric parsing: strconv.Atoi / strconv.ParseFloat — not fmt.Sscanf
- Prefer switch over if-else chains with 3+ branches

I/O AND RESOURCES:
- defer to close resources immediately after opening them
- bufio.Scanner / bufio.NewReader for line-by-line input
- Output always ends with \n; use fmt.Println or fmt.Fprintf(os.Stdout, "...\n", ...)
- Check write errors on os.Stdout when output is critical`
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
	var input struct {
		Code string   `json:"code"`
		Args []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		return "", fmt.Errorf("ler argumentos: %w", err)
	}
	if strings.TrimSpace(input.Code) == "" {
		return "", fmt.Errorf("campo 'code' obrigatório")
	}

	tmpDir, err := os.MkdirTemp("", "wasm-*")
	if err != nil {
		return "", fmt.Errorf("criar dir temporário: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	wasmBinary, err := buildWasm(ctx, tmpDir, input.Code)
	if err != nil {
		return "", err
	}

	return runInSandbox(ctx, wasmBinary, input.Args)
}

func buildWasm(ctx context.Context, dir, code string) ([]byte, error) {
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(code), 0o644); err != nil {
		return nil, fmt.Errorf("gravar código: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sandbox\n\ngo 1.21\n"), 0o644); err != nil {
		return nil, fmt.Errorf("criar go.mod: %w", err)
	}

	wasmFile := filepath.Join(dir, "main.wasm")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("obter home dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-o", wasmFile, filepath.Join(dir, "main.go"))
	cmd.Env = append(os.Environ(),
		"GOOS=wasip1",
		"GOARCH=wasm",
		"GOCACHE="+filepath.Join(homeDir, ".cache", "go-build"),
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("compilação:\n%s", string(out))
	}

	data, err := os.ReadFile(wasmFile)
	if err != nil {
		return nil, fmt.Errorf("ler binário wasm: %w", err)
	}
	return data, nil
}

func runInSandbox(ctx context.Context, wasmBinary []byte, args []string) (string, error) {
	cell, err := security.NewCell(ctx)
	if err != nil {
		return "", fmt.Errorf("criar sandbox: %w", err)
	}
	defer cell.Close(ctx)

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("obter cwd: %w", err)
	}
	cell.WithDirMount(cwd)

	var stdout, stderr bytes.Buffer
	cell.WithOutput(&stdout, &stderr)

	execArgs := make([]string, 0, len(args)+1)
	execArgs = append(execArgs, "sandbox")
	execArgs = append(execArgs, args...)

	execErr := cell.Execute(ctx, wasmBinary, execArgs)

	var sb strings.Builder
	sb.WriteString(stdout.String())

	if execErr != nil && !strings.Contains(execErr.Error(), "invalid magic number") {
		fmt.Fprintf(&sb, "\n[Erro na Sandbox]: %v", execErr)
	}
	if stderr.Len() > 0 {
		fmt.Fprintf(&sb, "\n[Stderr]: %s", stderr.String())
	}

	result := sb.String()
	if result == "" {
		return "Executou com sucesso (sem output).", nil
	}
	return result, nil
}
