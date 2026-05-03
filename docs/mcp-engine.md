# MCP Engine — Implementation Guide

> Model Context Protocol (MCP) integration for Jandaira Swarm OS.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Transport Layer](#transport-layer)
   - [Stdio Transport](#stdio-transport)
   - [SSE Transport](#sse-transport)
   - [Streamable HTTP Transport](#streamable-http-transport)
4. [Engine & JSON-RPC Client](#engine--json-rpc-client)
5. [Tool Adapter](#tool-adapter)
6. [Database Model](#database-model)
7. [Repository & Service](#repository--service)
8. [Queen Integration — GroupTools](#queen-integration--grouptools)
9. [Dispatch Lifecycle](#dispatch-lifecycle)
10. [REST API Reference](#rest-api-reference)
11. [Sandbox Requirements](#sandbox-requirements)
12. [Usage Examples](#usage-examples)
13. [File Map](#file-map)

---

## Overview

The MCP Engine allows each Jandaira **Colmeia** (hive) to connect to one or more external [Model Context Protocol](https://modelcontextprotocol.io/) servers. The relationship is **one-to-many**: one colmeia owns many MCP servers, and each MCP server belongs to exactly one colmeia.

When a dispatch is triggered, the engine:

1. Starts live connections to every active MCP server belonging to the colmeia.
2. Calls `tools/list` on each server and wraps the results as native `tool.Tool` implementations.
3. Registers those tools as **group-scoped** tools on the Queen, visible only during that colmeia's dispatch.
4. After the workflow finishes (or fails), closes the connections and deregisters the tools.

The LLM never knows it is talking to an external process — MCP tools appear in the tool listing alongside built-in tools such as `web_search` and `execute_code`.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Colmeia Dispatch                                            │
│                                                              │
│  ┌──────────────┐   tools/list   ┌─────────────────────┐    │
│  │  MCPEngine   │◄──────────────►│  MCP Server (stdio) │    │
│  │  (Go client) │   tools/call   │  (sbx sandbox)      │    │
│  └──────┬───────┘                └─────────────────────┘    │
│         │ MCPToolAdapter                                      │
│         │ implements tool.Tool                               │
│         ▼                                                    │
│  ┌──────────────┐   EquipGroupTool   ┌──────────────────┐   │
│  │   Queen      │◄──────────────────►│  Specialist      │   │
│  │  GroupTools  │   AllowedTools     │  (LLM agent)     │   │
│  └──────────────┘                    └──────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

```mermaid
sequenceDiagram
    participant H as Handler
    participant S as MCPServerService
    participant E as Engine
    participant Q as Queen
    participant MCP as MCP Server

    H->>S: StartEnginesForColmeia(ctx, colmeiaID)
    S->>E: NewEngine(StdioTransport)
    E->>MCP: initialize (JSON-RPC)
    MCP-->>E: initialized
    E->>MCP: tools/list
    MCP-->>E: [{name, description, inputSchema}]
    S-->>H: []tool.Tool (MCPToolAdapters), []*Engine

    H->>Q: EquipGroupTool(groupID, tool) × N
    H->>Q: AssembleSwarmForGroup(ctx, goal, n, groupID)
    Q-->>H: []Specialist (with MCP tool names in AllowedTools)
    H->>Q: DispatchWorkflow(ctx, groupID, goal, specialists)

    Note over Q: Specialist calls MCP tool
    Q->>E: MCPToolAdapter.Execute(ctx, argsJSON)
    E->>MCP: tools/call {name, arguments}
    MCP-->>E: {content: [{type:"text", text:"..."}]}
    E-->>Q: result string

    H->>Q: UnequipGroupTool(groupID, name) × N
    H->>E: Close()
```

---

## Transport Layer

`internal/mcp/transport.go` defines the `Transport` interface:

```go
type Transport interface {
    Start(ctx context.Context) error
    Send(ctx context.Context, msg []byte) error
    Receive() (<-chan []byte, error)
    Close() error
}
```

Both transports produce and consume **newline-delimited JSON-RPC 2.0** messages.

### Stdio Transport

`internal/mcp/transport_stdio.go`

Used when the MCP server runs as a **local subprocess** (most common case).

**How it works:**

1. Go calls `exec.Command` with the command tokens stored in the `Command []string` field (e.g. `["sbx","exec","mcp-base","npx","-y","@mcp/server-postgres","postgres://..."]`).
2. Go "hijacks" `stdin` and `stdout` of the child process.
3. `Send()` writes a JSON line to the child's `stdin`.
4. A background goroutine scans the child's `stdout` line-by-line and forwards each JSON line to an internal channel.
5. MCP server stderr is redirected to the Go logger for debugging.

```
Go process                          sbx sandbox (Docker MicroVM)
─────────────────────────────────   ──────────────────────────────────
stdin.Write(jsonRequest + "\n") ──► MCP server reads from own stdin
                                    processes request
stdout scanner receives line    ◄── MCP server writes to own stdout
```

**Key details:**
- Buffer is 1 MiB per line to handle large tool schemas.
- Non-JSON lines (sbx startup banners, tool init logs) are filtered in `readLoop` before reaching the engine's JSON-RPC parser — prevents timeout during `ListTools`.
- Extra environment variables (e.g. `DATABASE_URL`) are passed via `env_vars` field on the model.
- The process is killed (`Process.Kill`) on `Close()`, not gracefully terminated — suitable for short-lived dispatch connections.

**Auto-wrapping (`wrapSandboxCommand`):**

When a raw command like `["npx", "-y", "@mcp/server-postgres"]` is submitted via the API, the service transparently wraps it before validation and storage:

| `MCP_SANDBOX` env var | Resulting command |
|---|---|
| unset (default) | `["sbx", "exec", "mcp-base", "npx", "-y", "@mcp/server-postgres"]` |
| `docker` | `["docker", "run", "-i", "--rm", "node:22-alpine", "npx", "-y", "@mcp/server-postgres"]` |

Commands already starting with `sbx` or `docker` are returned unchanged. The `mcp-base` sandbox must be provisioned beforehand — see `scripts/setup-sandbox.sh`.

**Security model:**
- Only `sbx` and `docker` are accepted as entry-point executables — raw `npx`, `node`, `python` calls are auto-wrapped, never executed directly on the host.
- For `sbx`: only `exec` and `run` subcommands are permitted. Third argument is the sandbox name (`mcp-base`).
- For `docker`: only `run` subcommand is permitted.
- Dangerous flags (`--privileged`, `--network=host`, `--pid=host`, `--ipc=host`, `--cap-add=*`, `--device=*`, `--security-opt=*`) are rejected at registration time.
- Volume mounts (`--mount`, `-v`) are validated: path traversal and sensitive host directories (`/etc`, `/proc`, `/sys`, `/dev`, `/root`, `/var`, …) are blocked.

### SSE Transport

`internal/mcp/transport_sse.go`

Used for **remote HTTP MCP servers** (cloud-hosted integrations).

**Protocol:**

```
Client                          Remote MCP Server
──────────────────────────────  ──────────────────────────────
GET /sse                   ───► opens SSE stream
                           ◄─── event: endpoint
                                data: /messages?sessionId=abc

POST /messages?sessionId=abc ─► JSON-RPC request body
                           ◄─── event: message
                                data: JSON-RPC response
```

**Key details:**
- `Start()` blocks until the server sends the `endpoint` event (30-second timeout).
- Each `Send()` call is an independent `POST` request.
- The SSE loop runs in a background goroutine; closing cancels the context and terminates the stream.
- HTTP headers (`Headers` map on the transport) can carry `Authorization` tokens for authenticated remote servers.

---

## Engine & JSON-RPC Client

`internal/mcp/engine.go`

The `Engine` wraps a `Transport` and implements the MCP client protocol.

### Initialization Handshake

```go
engine := mcp.NewEngine(transport)
err := engine.Start(ctx) // connects + runs MCP handshake
```

`Start()` performs:
1. `transport.Start()` — starts child process or SSE connection.
2. Launches `receiveLoop` goroutine — reads incoming messages and routes them to pending callers.
3. Sends `initialize` request with `protocolVersion: "2024-11-05"` and client info.
4. Sends `notifications/initialized` notification (fire-and-forget).

### Request/Response Model

Each call gets an atomic int64 ID. A `chan rpcResponse` is stored in `pending[id]` before sending; the receive loop delivers the response to that channel. Context cancellation removes the pending entry and returns `ctx.Err()`.

```go
// Internal flow for every RPC call:
id := atomic.AddInt64(&e.idCounter, 1)
pending[id] = make(chan rpcResponse, 1)
transport.Send(marshal(request))
select {
case resp := <-pending[id]: return resp.Result
case <-ctx.Done():           return ctx.Err()
}
```

### Public Methods

| Method | MCP RPC | Description |
|---|---|---|
| `ListTools(ctx)` | `tools/list` | Returns all tools the server offers. |
| `ListResources(ctx)` | `resources/list` | Returns all resources the server offers. |
| `CallTool(ctx, name, args)` | `tools/call` | Invokes a tool with JSON arguments. |
| `Close()` | — | Terminates the transport. |

---

## Tool Adapter

`internal/mcp/adapter.go`

`MCPToolAdapter` implements `tool.Tool`, bridging the MCP world to the Queen's tool registry.

```go
type MCPToolAdapter struct {
    engine     *Engine
    mcpTool    Tool    // as returned by tools/list
    serverName string
}
```

**Name mangling:** Tool names are qualified as `{serverName}_{toolName}` with all non-alphanumeric characters replaced by underscores. This prevents collisions between different MCP servers and makes it clear to the LLM which integration a tool belongs to.

Example: server `postgres-prod`, tool `query` → registered as `postgres_prod_query`.

**Description:** Prefixed with `[MCP:{serverName}]` so the Queen can explain the tool's origin in the system prompt.

**Execute:** Deserialises `argsJSON` → calls `engine.CallTool()` → joins all `text`-type content blocks into a single string result. Returns an error if the server responds with `isError: true`.

---

## Database Model

`internal/model/mcp_server.go`

```go
type MCPServer struct {
    ID        string    // UUID v4
    ColmeiaID string    // FK → colmeias.id (ON DELETE CASCADE)
    Name      string    // unique within the colmeia
    Transport string    // "stdio" | "sse" | "http"
    Command   []string  // stdio: wrapped argv tokens, e.g. ["sbx","exec","mcp-base","npx","-y","@mcp/server-postgres","postgres://..."]
    URL       string    // sse/http: endpoint URL, e.g. "https://mcp.context7.com/mcp"
    EnvVars   string    // JSON object {"KEY":"VALUE"} — env vars for stdio, headers for sse/http
    Active    bool      // false = excluded from dispatch without deletion
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Each `MCPServer` belongs to exactly one `Colmeia`. Deleting a colmeia cascade-deletes all its MCP servers. The composite unique index `(colmeia_id, name)` prevents duplicate server names within the same hive while allowing reuse of the same name across different hives.

**Helper methods:**

| Method | Description |
|---|---|
| `GetEnvVars() map[string]string` | Deserialises `EnvVars` JSON field. |
| `SetEnvVars(map[string]string) error` | Serialises env vars into `EnvVars`. |
| `EnvSlice() []string` | Returns `["KEY=VALUE", ...]` format for `exec.Cmd.Env`. |

---

## Repository & Service

### Repository — `internal/repository/mcp_server.go`

```go
type MCPServerRepository interface {
    Create(s *model.MCPServer) error
    FindByID(id string) (*model.MCPServer, error)
    FindByColmeiaID(colmeiaID string) ([]model.MCPServer, error)
    Update(s *model.MCPServer) error
    Delete(id string) error
}
```

`FindByColmeiaID` queries `WHERE colmeia_id = ?` directly — no join table needed.

### Service — `internal/service/mcp_server.go`

```go
type MCPServerService interface {
    Create(colmeiaID, name, transport, command, url string, envVars map[string]string, active bool) (*model.MCPServer, error)
    GetByID(id string) (*model.MCPServer, error)
    ListForColmeia(colmeiaID string) ([]model.MCPServer, error)
    Update(id, name, transport, command, url string, envVars map[string]string, active bool) (*model.MCPServer, error)
    Delete(id string) error

    StartEnginesForColmeia(ctx context.Context, colmeiaID string) ([]tool.Tool, []*mcp.Engine, error)
}
```

**`StartEnginesForColmeia`** is the core method for dispatch integration:

```
For each active MCPServer belonging to colmeiaID:
  1. Build transport (StdioTransport or SSETransport)
  2. Create Engine, call Start(ctx)  ← 30s timeout recommended
  3. Call ListTools(ctx)
  4. Wrap each tool as MCPToolAdapter
  5. Accumulate adapters + engines

Return ([]tool.Tool, []*Engine, error)
Callers must close all engines regardless of error.
```

If a server's `ListTools` call fails, that server is skipped (non-fatal) and its engine is closed immediately. The dispatch continues with whatever tools were successfully loaded.

---

## Queen Integration — GroupTools

`internal/swarm/queen.go`

### New fields

```go
type Queen struct {
    // ... existing fields ...

    // GroupTools holds per-group tools (e.g. MCP adapters scoped to a colmeia).
    // Keyed by groupID → tool name → tool.
    GroupTools map[string]map[string]tool.Tool
}
```

### New methods

```go
// Register a tool visible only within groupID.
func (q *Queen) EquipGroupTool(groupID string, t tool.Tool)

// Remove a group-scoped tool by name.
func (q *Queen) UnequipGroupTool(groupID, name string)

// Merge global Tools + GroupTools[groupID] into a single snapshot map.
// Group tools shadow global tools with the same name.
func (q *Queen) mergedToolsForGroup(groupID string) map[string]tool.Tool
```

### Updated methods

| Method | Change |
|---|---|
| `AssembleSwarm(ctx, goal, maxWorkers)` | Now delegates to `AssembleSwarmForGroup(ctx, goal, maxWorkers, "")` — backward compatible. |
| `AssembleSwarmForGroup(ctx, goal, maxWorkers, groupID)` | **New.** Includes group tools in the tool listing shown to the LLM during meta-planning. |
| `runSpecialist(ctx, groupID, spec, ...)` | `groupID` added as second argument. Resolves tools from `mergedToolsForGroup(groupID)` instead of global `Tools` map. |
| `DispatchWorkflow(ctx, groupID, goal, pipeline)` | Passes `groupID` through to `runSpecialist`. |

Thread safety: all access to `GroupTools` is protected by the existing `mu sync.RWMutex`.

---

## Dispatch Lifecycle

The MCP engine lifecycle is managed inside `handleColmeiaDispatch` in `internal/api/colmeia_handler.go`.

```
handleColmeiaDispatch
│
├─ 1. Load colmeia (with MCPServers preloaded)
├─ 2. Build enrichedGoal (history, skills, semantic memory)
├─ 3. StartEnginesForColmeia(30s ctx)
│       └─ returns mcpTools, mcpEngines
├─ 4. EquipGroupTool(groupID, t) for each t in mcpTools
├─ 5. stopMCPEngines deferred (UnequipGroupTool + Engine.Close)
│
├─ [queen_managed=true]
│   ├─ 6. AssembleSwarmForGroup(2min ctx, groupID)
│   │       └─ LLM sees MCP tools in tool listing
│   └─ 7. goroutine: DispatchWorkflow → wait → stopMCPEngines
│
└─ [queen_managed=false]
    ├─ 6. BuildSpecialists(colmeia)
    └─ 7. goroutine: DispatchWorkflow → wait → stopMCPEngines
```

**Important:** Engines are started **before** `AssembleSwarmForGroup` so the LLM sees MCP tools during meta-planning and can assign them to specialists' `AllowedTools`.

**Lifecycle boundary:** Engines are closed inside the goroutine via `defer stopMCPEngines()`, ensuring cleanup runs even if the context is cancelled or the workflow panics.

---

## REST API Reference

All MCP server management is **scoped to a colmeia**. There is no global MCP server catalogue.

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/colmeias/:id/mcp-servers` | List MCP servers belonging to a hive. |
| `POST` | `/api/colmeias/:id/mcp-servers` | Create a new MCP server inside a hive. |
| `GET` | `/api/colmeias/:id/mcp-servers/:serverId` | Get a single MCP server. |
| `PUT` | `/api/colmeias/:id/mcp-servers/:serverId` | Update an MCP server configuration. |
| `DELETE` | `/api/colmeias/:id/mcp-servers/:serverId` | Delete an MCP server from a hive. |

### Request Bodies

**POST /api/colmeias/:id/mcp-servers**

`command` accepts either a raw runtime command or an already-wrapped sandbox command. The service auto-wraps raw commands via `wrapSandboxCommand` before storing them.

```json
{
  "name": "postgres-prod",
  "transport": "stdio",
  "command": ["npx", "-y", "@modelcontextprotocol/server-postgres", "postgres://user:pass@localhost/mydb"],
  "env_vars": {
    "NODE_ENV": "production"
  },
  "active": true
}
```

Stored in DB as `["sbx", "exec", "mcp-base", "npx", "-y", "@modelcontextprotocol/server-postgres", "postgres://..."]`.

```json
{
  "name": "sqlite-mcp",
  "transport": "stdio",
  "command": ["npx", "-y", "@modelcontextprotocol/server-sqlite", "/workspace/database.db"],
  "active": true
}
```

```json
{
  "name": "context7",
  "transport": "http",
  "url": "https://mcp.context7.com/mcp",
  "active": true
}
```

```json
{
  "name": "remote-filesystem",
  "transport": "sse",
  "url": "https://mcp.example.com/sse",
  "env_vars": {
    "Authorization": "Bearer sk-..."
  },
  "active": true
}
```

---

## Sandbox Requirements

For **stdio transport** all MCP servers run inside an E2B sandbox via `sbx`. The Jandaira host only needs the `sbx` CLI installed — Node.js, Python, or any other runtime runs inside the isolated sandbox, not on the host.

Install `sbx` on the host (or Jandaira container image):

```dockerfile
# Install sbx CLI for sandboxed MCP server execution.
RUN curl -fsSL https://raw.githubusercontent.com/e2b-dev/E2B/main/packages/cli/install.sh | sh
```

Then provision the `mcp-base` sandbox (run once):

```bash
bash scripts/setup-sandbox.sh
# or with a custom name/workspace:
bash scripts/setup-sandbox.sh mcp-base /path/to/workspace
```

The script checks whether the `mcp-base` sandbox exists and creates it via `sbx create --name mcp-base shell <workspace>` if not. All stdio MCP servers execute as `sbx exec mcp-base <cmd>` at dispatch time.

Set `MCP_SANDBOX=docker` to use Docker instead of E2B (no `sbx` required):

```bash
MCP_SANDBOX=docker ./jandaira
```

In Docker mode, raw commands are wrapped as `docker run -i --rm <image> <cmd>` using a built-in image map (`npx`/`node`/`tsx` → `node:22-alpine`, `python`/`uvx`/`uv` → `ghcr.io/astral-sh/uv:python3.12-alpine`).

---

## Usage Examples

### 1. Create a PostgreSQL MCP server inside a hive

Send the raw runtime command — the service auto-wraps it as `sbx exec mcp-base npx ...`:

```bash
curl -X POST http://localhost:8080/api/colmeias/{colmeia-id}/mcp-servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "postgres-analytics",
    "transport": "stdio",
    "command": ["npx", "-y", "@modelcontextprotocol/server-postgres", "postgres://analyst:secret@db:5432/analytics"],
    "active": true
  }'
```

### 2. Dispatch a goal — MCP tools load automatically

```bash
curl -X POST http://localhost:8080/api/colmeias/{colmeia-id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "List all orders from last month and calculate the total revenue"}'
```

The Queen will see tools like `postgres_analytics_query` and `postgres_analytics_list_tables` in its tool listing and may assign them to a specialist agent.

### 3. List MCP servers for a hive

```bash
curl http://localhost:8080/api/colmeias/{colmeia-id}/mcp-servers
```

### 4. Update an MCP server

```bash
curl -X PUT http://localhost:8080/api/colmeias/{colmeia-id}/mcp-servers/{server-id} \
  -H "Content-Type: application/json" \
  -d '{
    "name": "postgres-analytics",
    "transport": "stdio",
    "command": ["npx", "-y", "@modelcontextprotocol/server-postgres", "postgres://..."],
    "active": false
  }'
```

### 5. Remote SSE server with auth header

```bash
curl -X POST http://localhost:8080/api/colmeias/{colmeia-id}/mcp-servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github-mcp",
    "transport": "sse",
    "url": "https://mcp.github.example.com",
    "env_vars": {"Authorization": "Bearer ghp_xxxxxxxxxxxx"},
    "active": true
  }'
```

### 6. Delete an MCP server

```bash
curl -X DELETE http://localhost:8080/api/colmeias/{colmeia-id}/mcp-servers/{server-id}
```

---

## File Map

```
internal/mcp/
├── transport.go          # Transport interface
├── transport_stdio.go    # Stdio (subprocess) implementation
├── transport_sse.go      # SSE (remote HTTP) implementation
├── jsonrpc.go            # JSON-RPC 2.0 request/response types
├── engine.go             # MCP client: handshake, ListTools, CallTool
└── adapter.go            # MCPToolAdapter — bridges Engine to tool.Tool

internal/model/
└── mcp_server.go         # MCPServer GORM entity + helpers (ColmeiaID FK)

internal/repository/
└── mcp_server.go         # CRUD scoped by ColmeiaID

internal/service/
└── mcp_server.go         # Business logic + StartEnginesForColmeia

internal/api/
└── mcp_handler.go        # HTTP handlers — colmeia-scoped CRUD

internal/swarm/
└── queen.go              # GroupTools, EquipGroupTool, AssembleSwarmForGroup
```
