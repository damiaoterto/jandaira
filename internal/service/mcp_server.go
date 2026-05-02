package service

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/damiaoterto/jandaira/internal/mcp"
	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/tool"
)

var ErrMCPServerNotFound = errors.New("MCP server not found")

// MCPServerService defines business logic for managing MCP server configurations
// scoped to a colmeia, and for starting live connections during dispatch.
type MCPServerService interface {
	Create(colmeiaID, name, transport string, command []string, url string, envVars map[string]string, active bool) (*model.MCPServer, error)
	GetByID(id string) (*model.MCPServer, error)
	ListForColmeia(colmeiaID string) ([]model.MCPServer, error)
	Update(id, name, transport string, command []string, url string, envVars map[string]string, active bool) (*model.MCPServer, error)
	Delete(id string) error

	// StartEnginesForColmeia connects to all active MCP servers of a colmeia,
	// discovers their tools, and returns adapters that implement tool.Tool plus
	// the live engines that must be closed when the dispatch finishes.
	StartEnginesForColmeia(ctx context.Context, colmeiaID string) ([]tool.Tool, []*mcp.Engine, error)
}

type mcpServerService struct {
	repo repository.MCPServerRepository
}

// NewMCPServerService creates a new MCPServerService.
func NewMCPServerService(repo repository.MCPServerRepository) MCPServerService {
	return &mcpServerService{repo: repo}
}

func (s *mcpServerService) Create(colmeiaID, name, transport string, command []string, url string, envVars map[string]string, active bool) (*model.MCPServer, error) {
	if err := validateTransport(transport); err != nil {
		return nil, err
	}
	if transport == model.MCPTransportStdio {
		command = wrapSandboxCommand(command)
		if err := validateCommand(command); err != nil {
			return nil, err
		}
	}
	if (transport == model.MCPTransportSSE || transport == model.MCPTransportHTTP) && url == "" {
		return nil, fmt.Errorf("transport %q requires a non-empty url", transport)
	}
	srv := &model.MCPServer{
		ColmeiaID: colmeiaID,
		Name:      name,
		Transport: transport,
		Command:   command,
		URL:       url,
		Active:    active,
	}
	if err := srv.SetEnvVars(envVars); err != nil {
		return nil, fmt.Errorf("failed to serialize env vars: %w", err)
	}
	if err := s.repo.Create(srv); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *mcpServerService) GetByID(id string) (*model.MCPServer, error) {
	srv, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrMCPServerNotFound) {
			return nil, ErrMCPServerNotFound
		}
		return nil, err
	}
	return srv, nil
}

func (s *mcpServerService) ListForColmeia(colmeiaID string) ([]model.MCPServer, error) {
	return s.repo.FindByColmeiaID(colmeiaID)
}

func (s *mcpServerService) Update(id, name, transport string, command []string, url string, envVars map[string]string, active bool) (*model.MCPServer, error) {
	if err := validateTransport(transport); err != nil {
		return nil, err
	}
	if transport == model.MCPTransportStdio {
		command = wrapSandboxCommand(command)
		if err := validateCommand(command); err != nil {
			return nil, err
		}
	}
	if (transport == model.MCPTransportSSE || transport == model.MCPTransportHTTP) && url == "" {
		return nil, fmt.Errorf("transport %q requires a non-empty url", transport)
	}
	srv, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrMCPServerNotFound) {
			return nil, ErrMCPServerNotFound
		}
		return nil, err
	}
	srv.Name = name
	srv.Transport = transport
	srv.Command = command
	srv.URL = url
	srv.Active = active
	if err := srv.SetEnvVars(envVars); err != nil {
		return nil, fmt.Errorf("failed to serialize env vars: %w", err)
	}
	if err := s.repo.Update(srv); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *mcpServerService) Delete(id string) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrMCPServerNotFound) {
			return ErrMCPServerNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

// StartEnginesForColmeia connects to each active MCP server belonging to the
// colmeia, discovers its tools, and returns adapters wrapping those tools plus
// the live engine handles. Callers must call Engine.Close() on every returned
// engine when the workflow finishes (even on error).
func (s *mcpServerService) StartEnginesForColmeia(ctx context.Context, colmeiaID string) ([]tool.Tool, []*mcp.Engine, error) {
	servers, err := s.repo.FindByColmeiaID(colmeiaID)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp service: list servers for colmeia %s: %w", colmeiaID, err)
	}

	var (
		allTools   []tool.Tool
		allEngines []*mcp.Engine
	)

	for _, srv := range servers {
		if !srv.Active {
			continue
		}

		transport, err := buildTransport(&srv)
		if err != nil {
			closeEngines(allEngines)
			return nil, nil, fmt.Errorf("mcp service: build transport for %q: %w", srv.Name, err)
		}

		engine := mcp.NewEngine(transport)
		if err := engine.Start(ctx); err != nil {
			closeEngines(allEngines)
			return nil, nil, fmt.Errorf("mcp service: start engine for %q: %w", srv.Name, err)
		}

		mcpTools, err := engine.ListTools(ctx)
		if err != nil {
			// Non-fatal: log and continue — the server may not support tools.
			_ = engine.Close()
			continue
		}

		for _, t := range mcpTools {
			allTools = append(allTools, mcp.NewMCPToolAdapter(engine, t, srv.Name))
		}
		allEngines = append(allEngines, engine)
	}

	return allTools, allEngines, nil
}

// buildTransport constructs the correct Transport implementation for a server.
func buildTransport(srv *model.MCPServer) (mcp.Transport, error) {
	switch srv.Transport {
	case model.MCPTransportStdio:
		if len(srv.Command) == 0 {
			return nil, fmt.Errorf("stdio transport requires a non-empty command")
		}
		return mcp.NewStdioTransport(srv.Command, srv.EnvSlice()), nil

	case model.MCPTransportSSE:
		if srv.URL == "" {
			return nil, fmt.Errorf("sse transport requires a non-empty URL")
		}
		headers := srv.GetEnvVars()
		return mcp.NewSSETransport(srv.URL, headers), nil

	case model.MCPTransportHTTP:
		if srv.URL == "" {
			return nil, fmt.Errorf("http transport requires a non-empty URL")
		}
		headers := srv.GetEnvVars()
		return mcp.NewStreamableHTTPTransport(srv.URL, headers), nil

	default:
		return nil, fmt.Errorf("unknown transport %q", srv.Transport)
	}
}

// closeEngines terminates all engines in the slice, ignoring errors.
func closeEngines(engines []*mcp.Engine) {
	for _, e := range engines {
		_ = e.Close()
	}
}

// validateTransport returns an error if transport is not a recognised value.
func validateTransport(t string) error {
	switch t {
	case model.MCPTransportStdio, model.MCPTransportSSE, model.MCPTransportHTTP:
		return nil
	}
	return fmt.Errorf("invalid transport %q: must be \"stdio\", \"sse\", or \"http\"", t)
}

// validateCommand delegates to security.ValidateSbxCommand, enforcing that all
// stdio MCP servers run inside an isolated sandbox.
func validateCommand(command []string) error {
	return security.ValidateSbxCommand(command)
}

// runtimeImage maps a well-known CLI runtime to a minimal Docker image.
var runtimeImage = map[string]string{
	"npx":     "node:22-alpine",
	"node":    "node:22-alpine",
	"tsx":     "node:22-alpine",
	"ts-node": "node:22-alpine",
	"python":  "ghcr.io/astral-sh/uv:python3.12-alpine",
	"python3": "ghcr.io/astral-sh/uv:python3.12-alpine",
	"uvx":     "ghcr.io/astral-sh/uv:python3.12-alpine",
	"uv":      "ghcr.io/astral-sh/uv:python3.12-alpine",
}

// wrapSandboxCommand transparently wraps a raw MCP command inside the configured
// sandbox backend. If the command already starts with "sbx" or "docker" it is
// returned unchanged (backward-compatible).
//
// Backend selection via MCP_SANDBOX env var:
//   - "docker" → docker run -i --rm <image> <cmd...>
//   - default  → sbx exec <cmd...>
func wrapSandboxCommand(raw []string) []string {
	if len(raw) == 0 {
		return raw
	}
	if raw[0] == "sbx" || raw[0] == "docker" {
		return raw // already wrapped
	}

	if os.Getenv("MCP_SANDBOX") == "docker" {
		image, ok := runtimeImage[raw[0]]
		if !ok {
			image = "alpine"
		}
		wrapped := []string{"docker", "run", "-i", "--rm", image}
		return append(wrapped, raw...)
	}

	// default: sbx
	return append([]string{"sbx", "exec"}, raw...)
}
