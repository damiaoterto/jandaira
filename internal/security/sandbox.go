package security

import (
	"context"
	"fmt"
	"io"
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
func NewCell(ctx context.Context) (*Cell, error) {
	r := wazero.NewRuntime(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	config := wazero.NewModuleConfig().
		WithStdout(io.Discard).
		WithStderr(io.Discard).
		WithStdin(io.NopCloser(strings.NewReader("")))

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

// WithDirMount mounts a host directory at "/" inside the sandbox.
func (c *Cell) WithDirMount(hostDir string) *Cell {
	fs := wazero.NewFSConfig().WithDirMount(hostDir, "/")
	return c.WithVirtualFS(fs)
}

// WithOutput redirects the WASM stdout and stderr to the provided writers.
func (c *Cell) WithOutput(stdout, stderr io.Writer) *Cell {
	c.config = c.config.WithStdout(stdout).WithStderr(stderr)
	return c
}

// Execute runs a pre-compiled Wasm binary within the cell's isolation.
func (c *Cell) Execute(ctx context.Context, wasmBinary []byte, args []string) error {
	compiledModule, err := c.runtime.CompileModule(ctx, wasmBinary)
	if err != nil {
		return fmt.Errorf("failed to compile wasm module: %w", err)
	}
	defer compiledModule.Close(ctx)

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

// RegisterHostFunction registers a Go function as a host function in the sandbox.
func (c *Cell) RegisterHostFunction(ctx context.Context, moduleName, funcName string, function any) error {
	_, err := c.runtime.NewHostModuleBuilder(moduleName).
		NewFunctionBuilder().
		WithFunc(function).
		Export(funcName).
		Instantiate(ctx)
	return err
}
