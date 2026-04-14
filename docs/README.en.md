# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="../jandaira.png" alt="Jandaira Logo"/>
</p>

A **multi-agent autonomous framework** written in Go, inspired by the collective intelligence of the native Brazilian bee *Melipona subnitida* — the **Jandaíra**.

---

> 🌐 **English** · [Português](../README.md) · [中文](README.zh.md) · [Русский](README.ru.md)

---

## 📖 Why "Jandaira"?

The **Jandaíra** (*Melipona subnitida*) is a stingless bee endemic to the Caatinga biome of Brazil. Small, resilient, and extraordinarily cooperative — it doesn't need a centralized leader to build a functional hive. Each worker knows its role, executes its task autonomously, and returns the result to the collective.

This is exactly the architectural model this project implements:

- The **Queen (`Queen`)** does not execute tasks — she orchestrates, validates policies, and ensures security.
- The **Specialists (`Specialists`)** are lightweight agents with restricted tools, executing in isolated silos.
- **Nectar** is the metaphor for the token budget: each agent consumes nectar; when it runs out, the hive stops.
- The **Honeycomb (`Honeycomb`)** is the shared vector memory — collective knowledge that persists between missions, stored in ChromaDB.
- The **Beekeeper** is the human in the loop: they can approve or block any AI action before it is executed.

---

## 🏗️ Architecture

### Flow Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                   API REST + WebSocket (:8080)                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  👤 Client sends goal via POST /api/dispatch             │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ DispatchWorkflow()
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Queen (Orchestrator)                           │
│                                                                  │
│  ┌──────────────┐   ┌─────────────┐   ┌──────────────────────┐  │
│  │  GroupQueue  │   │   Policy    │   │   NectarUsage ($$)   │  │
│  │  (FIFO, N=3) │   │ (isolate,   │   │   Token budget       │  │
│  │              │   │  approval)  │   │   per swarm          │  │
│  └──────────────┘   └─────────────┘   └──────────────────────┘  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ Pipeline (Baton Pass)
          ┌────────────────┴─────────────────┐
          ▼                                  ▼
┌──────────────────────┐          ┌──────────────────────┐
│  Specialist #1       │  ctx     │  Specialist #2       │
│  "Developer"         │ ──────►  │  "Auditor"           │
│  Tools: write_file   │          │  Tools: execute_code │
│         search_mem   │          │         read_file    │
└──────────┬───────────┘          └──────────┬───────────┘
           │                                 │
           ▼                                 ▼
┌──────────────────────────────────────────────────────────┐
│                   🔐 Security Layer                       │
│   Encrypted payload (AES-GCM) between each baton pass    │
│   — context never travels in plain text                  │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│              👨‍🌾 Beekeeper (Human-in-the-Loop)            │
│   RequiresApproval=true → WS sends approval_request      │
│   approved=true → authorize │ approved=false → block     │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│                   🍯 Honeycomb (ChromaDB)                 │
│   Workflow result is embedded and indexed                 │
│   Long-term memory shared between missions               │
└──────────────────────────────────────────────────────────┘
```

### Package Map

```
jandaira/
├── cmd/
│   └── api/
│       └── main.go          # Entrypoint: HTTP + WebSocket server
│
└── internal/
    ├── brain/               # AI contracts (Brain, Honeycomb)
    │   ├── open_ai.go       # OpenAI implementation (Chat + Embed)
    │   ├── memory.go        # Honeycomb interface + LocalVectorDB
    │   └── chroma.go        # ChromaDB implementation (ChromaHoneycomb)
    │
    ├── queue/               # FIFO scheduler with limited concurrency
    │   └── group_queue.go   # GroupQueue: N workers per group
    │
    ├── security/            # Inter-agent payload encryption
    │   ├── crypto.go        # AES-GCM Seal/Open + key generation
    │   ├── vault.go         # Local secrets vault
    │   └── sandbox.go       # Execution sandbox
    │
    ├── swarm/               # Core agent system
    │   └── queen.go         # Orchestrator: policies, HIL, pipeline
    │
    ├── tool/                # Tools available to agents
    │   ├── list_directory.go
    │   ├── search_memory.go # search_memory + store_memory
    │   └── wasm.go          # Execution sandbox via wazero
    │
    ├── api/                 # HTTP handlers and WebSocket
    ├── config/              # Application configuration
    ├── database/            # SQLite connection
    ├── i18n/                # Internationalization
    ├── model/               # Data models
    ├── prompt/              # Prompt templates
    ├── repository/          # Data access
    └── service/             # Business logic
```

---

## ⚡ Differentials vs. NanoClaw

| Feature | NanoClaw (Python) | Jandaira (Go) |
|---|---|---|
| **Language** | Python | Go 1.22+ |
| **Concurrency** | `asyncio` / threads | Native Goroutines + channels |
| **Agent isolation** | Docker containers | Wasm via `wazero` (no Docker) |
| **IPC communication** | JSON on disk / Redis | Typed shared memory |
| **Inter-agent encryption** | ❌ Does not exist | ✅ AES-GCM between each pass |
| **Human-in-the-Loop** | Optional / external | ✅ Native: Beekeeper mode via WebSocket |
| **Token budget** | Manual | ✅ Automatic `NectarUsage` per swarm |
| **Vector memory** | Pinecone / external | ✅ ChromaDB via Docker |
| **Interface** | Nonexistent | ✅ REST API + WebSocket |
| **IPC latency** | High (disk/network I/O) | Minimal (memory) |

### Why does Go outperform Python here?

1. **Goroutines are cheaper than threads** — running 100 simultaneous agents costs a fraction of what it would in Python with `asyncio` or `threading`.
2. **Static binary** — zero runtime dependencies. A `go build` generates an executable that runs on any Linux without installing anything.
3. **No GIL** — Python has the Global Interpreter Lock; Go truly parallelizes across multiple cores.
4. **`wazero` is 100% Go** — the Wasm runtime requires no CGo, Docker, or external systems. The agent runs in a sandbox inside the same process.

---

## 🚀 Usage Tutorial

### Prerequisites

```bash
# Go 1.22 or higher
go version

# Docker (for ChromaDB)
docker --version

# OpenAI API key
export OPENAI_API_KEY="sk-..."
```

### Starting ChromaDB

```bash
# Via Docker directly
docker run -d --name chroma -p 8000:8000 chromadb/chroma:latest

# Or using the project's docker-compose
docker compose up -d
```

By default the server connects to `http://localhost:8000`. To use a different address:

```bash
export CHROMA_URL="http://my-chroma:8000"
```

### Installation

#### Option 1 — Build from source

```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira

# Download dependencies
go mod tidy

# Build the API server
go build -o jandaira-api ./cmd/api/
```

#### Option 2 — Run directly

```bash
go run ./cmd/api/main.go --port 8080
```

### Run the hive

```bash
./jandaira-api --port 8080
```

The server will be available at `http://localhost:8080`. Monitor real-time events via WebSocket at `ws://localhost:8080/ws`.

### Example: create and test a Go file

1. Send the goal via `POST /api/dispatch`:

   ```bash
   curl -X POST http://localhost:8080/api/dispatch \
     -H "Content-Type: application/json" \
     -d '{"goal": "Create a Go file called sum.go that adds two numbers", "group_id": "enxame-alfa"}'
   ```

2. The Queen distributes the task to the Specialist pipeline:
   - **Wasm Developer** → writes `sum.go` using `write_file`
   - **Quality Auditor** → executes the code with `execute_code` and generates a report

3. Follow progress via WebSocket:

   ```json
   { "type": "agent_change", "agent": "Wasm Developer" }
   { "type": "tool_start",   "agent": "Wasm Developer", "tool": "write_file", "args": "{...}" }
   { "type": "result",       "message": "# Final Report\n..." }
   ```

4. If `RequiresApproval: true`, **Beekeeper mode** is activated. The server sends an `approval_request` via WebSocket and waits for a response:

   ```json
   // Server sends:
   { "type": "approval_request", "id": "req-1712345678901", "tool": "write_file", "args": "{...}" }

   // Client responds:
   { "type": "approve", "id": "req-1712345678901", "approved": true }
   ```

5. At the end, the result is saved to ChromaDB vector memory for future use.

### Configure your own swarm

Edit `cmd/api/main.go` to define your swarm policy:

```go
queen.RegisterSwarm("my-swarm", swarm.Policy{
    MaxNectar:        50000,  // Token budget
    Isolate:          true,   // Isolated context per group
    RequiresApproval: true,   // Beekeeper mode (HIL)
})
```

### Available tools

| Tool | Description |
|---|---|
| `list_directory` | Lists files and folders in a directory |
| `read_file` | Reads the content of a file |
| `write_file` | Creates or overwrites a file |
| `execute_code` | Executes code in an isolated Wasm sandbox |
| `web_search` | Searches the web via DuckDuckGo (direct answers, definitions, summaries) |
| `search_memory` | Semantic search in the hive's vector memory (ChromaDB) |
| `store_memory` | Saves knowledge to vector memory |

---

## 🔐 Security

Each "baton pass" between Specialists is **encrypted with AES-GCM**:

1. An ephemeral session key is generated at the beginning of each workflow
2. The accumulated context is **encrypted before being sent** to the next Specialist
3. The Specialist receives the encrypted payload, decrypts, processes, and **re-encrypts** its response
4. No context travels in plain text between agents

This simulates a secure IPC channel, where even if one agent is compromised, it cannot read the history of other agents in the pipeline.

---

## 🌐 API Reference

Start the HTTP server with `./jandaira-api --port 8080`. The following routes are available:

### REST Routes

| Method | Route | Description |
|---|---|---|
| `POST` | `/api/dispatch` | Submits a goal to the swarm for execution |
| `GET` | `/api/tools` | Lists all available tools and their parameters |
| `GET` | `/api/agents` | Lists the specialists in the configured workflow |
| `GET` | `/ws` | Opens a WebSocket connection for real-time events |

#### `POST /api/dispatch`

```json
// Request
{ "goal": "Create a Go file that sums two numbers", "group_id": "enxame-alfa" }

// Response 202
{ "message": "Mission dispatched to the swarm. Follow progress via WebSocket." }
```

#### `GET /api/tools`

```json
// Response 200
{
  "tools": [
    { "name": "write_file", "description": "Creates or overwrites a file", "parameters": { ... } },
    { "name": "execute_code", "description": "Executes code in a Wasm sandbox", "parameters": { ... } }
  ]
}
```

#### `GET /api/agents`

```json
// Response 200
{
  "agents": [
    { "name": "Wasm Developer", "system_prompt": "...", "allowed_tools": ["write_file", "search_memory"] },
    { "name": "Quality Auditor", "system_prompt": "...", "allowed_tools": ["execute_code", "read_file"] }
  ]
}
```

---

### WebSocket Events (`/ws`)

All events are exchanged as JSON over the same WebSocket channel. The Beekeeper **does not need REST routes** — approvals are handled entirely via WebSocket.

#### Server → Frontend

| `type` | When fired | Relevant fields |
|---|---|---|
| `status` | Progress messages from the Queen | `message` |
| `agent_change` | A specialist takes control of the pipeline | `agent` |
| `tool_start` | A tool is about to be executed | `agent`, `tool`, `args` |
| `approval_request` | The AI wants to use a gated tool | `id`, `tool`, `args` |
| `result` | Final workflow report | `message` |
| `error` | Failure or timeout | `message` |

```json
{ "type": "status",           "message": "🚀 Queen received the goal and is starting the swarm..." }
{ "type": "agent_change",     "agent": "Wasm Developer" }
{ "type": "tool_start",       "agent": "Wasm Developer", "tool": "write_file", "args": "{...}" }
{ "type": "approval_request", "id": "req-1712345678901", "tool": "write_file", "args": "{...}" }
{ "type": "result",           "message": "# Final Report\n..." }
{ "type": "error",            "message": "Mission timeout reached." }
```

#### Frontend → Server

| `type` | When to send | Required fields |
|---|---|---|
| `approve` | Beekeeper response to an `approval_request` | `id`, `approved` |

```json
{ "type": "approve", "id": "req-1712345678901", "approved": true }
{ "type": "approve", "id": "req-1712345678901", "approved": false }
```

> **Note:** The `id` field must exactly match the `id` received in the `approval_request`. Invalid or already-processed IDs return an `error` event.

---

## ⚖️ License and Commercial Use (Dual License)

**Jandaira Swarm OS** is distributed under a dual-licensing model, designed to foster open-source development while meeting corporate compliance needs.

* **Open Source Use (AGPLv3):** The source code is freely available under the [GNU Affero General Public License v3.0](../LICENCE). Anyone can use, modify, and distribute the software for free, provided that all modifications and the source code of derivative projects (including SaaS and backend network services) are also made available under the same license.
* **Enterprise Commercial Use:** For companies looking to integrate Jandaira into proprietary commercial products, web services (SaaS), or corporate backends without being required to open-source their own applications, we offer a **Commercial License**.

**Summary:** The project is open and free for the open-source community. Organizations with strict compliance requirements can purchase a commercial license to keep their intellectual property completely private. For commercial inquiries, please contact the maintainers.

---

## 🤝 Contributing

Pull Requests are welcome! Open an issue describing the feature or bug before starting.

---

*Jandaira: Autonomy, Security, and the Power of the Brazilian Swarm.* 🐝
