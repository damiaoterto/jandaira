package security

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Cell represents an isolated execution environment (a Wasm instance).
type Cell struct {
	runtime wazero.Runtime
	config  wazero.ModuleConfig
}

// NewCell initializes a new wazero runtime and default configuration.
// envVars is an explicit allowlist of env var names to forward into the sandbox.
// Only vars present in this list and set in the host environment are forwarded.
func NewCell(ctx context.Context, envVars []string) (*Cell, error) {
	// Create a new wazero runtime.
	r := wazero.NewRuntime(ctx)

	// Instantiate WASI (WebAssembly System Interface) to provide standard OS-like functions.
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// Default configuration: no access to host FS, no network, restricted env vars.
	// stdin is an empty reader — never wraps nil to avoid panics on wasm reads.
	config := wazero.NewModuleConfig().
		WithStdout(io.Discard).
		WithStderr(io.Discard).
		WithStdin(io.NopCloser(strings.NewReader("")))

	// Forward only explicitly requested env vars from the host environment.
	for _, envName := range envVars {
		if val := os.Getenv(envName); val != "" {
			config = config.WithEnv(envName, val)
		}
	}

	return &Cell{
		runtime: r,
		config:  config,
	}, nil
}

// WithVirtualFS attaches a virtual filesystem to the sandbox cell.
func (c *Cell) WithVirtualFS(fs wazero.FSConfig) *Cell {
	c.config = c.config.WithFSConfig(fs)
	return c
}

// WithOutput redirects the WASM stdout and stderr to the provided writers.
func (c *Cell) WithOutput(stdout, stderr io.Writer) *Cell {
	c.config = c.config.WithStdout(stdout).WithStderr(stderr)
	return c
}

// Execute runs a pre-compiled Wasm binary within the cell's isolation.
func (c *Cell) Execute(ctx context.Context, wasmBinary []byte, args []string) error {
	// Compile the Wasm binary.
	compiledModule, err := c.runtime.CompileModule(ctx, wasmBinary)
	if err != nil {
		return fmt.Errorf("failed to compile wasm module: %w", err)
	}
	defer compiledModule.Close(ctx)

	// Instantiate the module with the provided configuration.
	mod, err := c.runtime.InstantiateModule(ctx, compiledModule, c.config.WithArgs(args...))
	if err != nil {
		return fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	return nil
}

// Close cleans up the wazero runtime and all instantiated modules.
func (c *Cell) Close(ctx context.Context) error {
	return c.runtime.Close(ctx)
}

// Example usage showing host function registration (The "Nectar" API)
func (c *Cell) RegisterHostFunction(ctx context.Context, moduleName, funcName string, function any) error {
	_, err := c.runtime.NewHostModuleBuilder(moduleName).
		NewFunctionBuilder().
		WithFunc(function).
		Export(funcName).
		Instantiate(ctx)
	return err
}
