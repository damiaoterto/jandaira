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
- The **Honeycomb (`Honeycomb`)** is the two-tier persistent memory system: `ShortTermMemory` keeps recent context in RAM with automatic TTL expiry; the embedded `VectorEngine` (BadgerDB + HNSW) archives consolidated long-term knowledge as vector embeddings — no external processes required.
- The **Knowledge Graph (`KnowledgeGraph`)** maps relationships between agents, topics, and tools — the Queen queries it before every mission to reuse specialist profiles that have already succeeded on similar goals.
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
│  Tools: execute_code │          │  Tools: execute_code │
│         search_mem   │          │         store_memory │
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
│             🍯 Honeycomb (VectorEngine)                  │
│   Workflow result is embedded and indexed                 │
│   Embedded long-term memory (BadgerDB + HNSW)            │
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
    ├── brain/               # Hive nervous system
    │   ├── open_ai.go       # Brain: Chat + Embed via OpenAI
    │   ├── memory.go        # Honeycomb: interface + Result/Document types
    │   ├── hnsw.go          # HNSW index (approximate nearest neighbour)
    │   ├── vector_engine.go # VectorEngine: embedded BadgerDB + HNSW
    │   ├── graph.go         # KnowledgeGraph: agent ↔ topic graph (GraphRAG)
    │   ├── short_term.go    # ShortTermMemory: TTL buffer + auto-compaction
    │   └── document.go      # Text extraction + chunking (PDF, DOCX, XLSX…)
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

## 🧠 Memory Architecture

`internal/brain/` goes far beyond a vector store: it implements a two-tier memory hierarchy with a knowledge graph that grows with every mission.

### Short-Term Memory — `ShortTermMemory`

`brain/short_term.go` is a per-entry TTL message buffer. It solves the context overflow problem in long-running swarms:

- Each message receives an expiry timestamp at insertion time
- Expired entries are silently dropped on the next access
- **Automatic compaction**: when the buffer hits `maxEntries`, the LLM summarises the accumulated history into a dense paragraph → the summary is embedded and archived in VectorEngine as `short_term_archive` → the RAM buffer is cleared
- `Flush(ctx)` should be called at session end to guarantee complete archival; if the LLM fails, the raw transcript is archived as a fallback

```
 New message inserted
         │
         ▼
┌──────────────────────────────────┐
│      ShortTermMemory (RAM)       │
│  [msg₁ · expires: +30min]       │
│  [msg₂ · expires: +30min]       │
│  ...                             │
│  [msgN · expires: +30min]       │ ← overflow: compact() fires
└──────────────────────────────────┘
         │
         ▼
   LLM summarises history
         │
         ▼
┌──────────────────────────────────┐
│  VectorEngine (Long-Term Memory) │
│  type: "short_term_archive"      │
│  content: "In [session], the     │
│  agent decided X, found Y..."    │
└──────────────────────────────────┘
```

### Knowledge Graph — `KnowledgeGraph` (GraphRAG)

`brain/graph.go` implements a JSON-persisted knowledge graph (`~/.config/jandaira/knowledge_graph.json`) that automatically accumulates expertise after every completed workflow.

**Data model**

| Element | Type | Example |
|---|---|---|
| Specialist profile | `agent` node | `"Data Analyst"` |
| Mission domain | `topic` node | `"financial report analysis"` |
| Expertise link | `expert_in` edge | `agent → topic` |

**Queen's automatic learning cycle**

After each workflow, `registerWorkflowInGraph` runs in the background:
1. Creates/updates a `topic` node with the mission goal (up to 80 chars)
2. For each pipeline specialist, creates/updates an `agent` node with the prompt preview
3. Creates `expert_in` edges linking each agent to the topic

Before assembling the next swarm, `graphContextForGoal`:
1. Extracts keywords from the goal (> 4 chars)
2. Finds `topic` nodes whose label contains each keyword
3. Returns the `agent` nodes connected via `expert_in`
4. Injects a **"PAST SPECIALIST KNOWLEDGE"** block into the meta-planning prompt

Result: the Queen designs progressively better swarms over time, using only graph lookups — no extra LLM calls.

```
 New goal: "Analyse quarterly sales data"
         │
         ▼
  graphContextForGoal() — extract keywords
         │
         ▼
┌────────────────────────────────────────────┐
│              KnowledgeGraph                │
│                                            │
│  "Sales Analyst"  ─expert_in─► "sales data"
│  "Report Extractor" ─expert_in─► "quarterly analysis"
│                                            │
└────────────────────────────────────────────┘
         │  historical profiles found
         ▼
  Queen prompt enriched with past specialists
         │
         ▼
  AssembleSwarm() → more precise delegation
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
| **Vector memory** | Pinecone / external | ✅ Embedded VectorEngine (BadgerDB + HNSW) |
| **Knowledge graph** | ❌ Does not exist | ✅ `KnowledgeGraph` — native GraphRAG |
| **Short-term memory** | ❌ Does not exist | ✅ `ShortTermMemory` with TTL + LLM compaction |
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

# OpenAI API key
export OPENAI_API_KEY="sk-..."
```

> **No Docker required.** The vector database (`VectorEngine`) is embedded in the binary and persists automatically to `~/.config/jandaira/vectordb/`.

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
   - **Wasm Developer** → compiles and runs `sum.go` inside the Wasm sandbox via `execute_code`
   - **Quality Auditor** → validates the result and persists the report with `store_memory`

3. Follow progress via WebSocket:

   ```json
   { "type": "agent_change", "agent": "Wasm Developer" }
   { "type": "tool_start",   "agent": "Wasm Developer", "tool": "execute_code", "args": "{...}" }
   { "type": "result",       "message": "# Final Report\n..." }
   ```

4. If `RequiresApproval: true`, **Beekeeper mode** is activated. The server sends an `approval_request` via WebSocket and waits for a response:

   ```json
   // Server sends:
   { "type": "approval_request", "id": "req-1712345678901", "tool": "execute_code", "args": "{...}" }

   // Client responds:
   { "type": "approve", "id": "req-1712345678901", "approved": true }
   ```

5. At the end, the result is embedded and saved to the local VectorEngine for future use.

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
| `read_file` | Reads file content (read-only — agents never persist data to disk) |
| `execute_code` | Compiles and runs Go code in an isolated Wasm sandbox — use for calculations and data processing |
| `web_search` | Searches the web via DuckDuckGo (direct answers, definitions, summaries) |
| `search_memory` | Semantic search in the hive's vector memory (BadgerDB + HNSW); degrades gracefully when embedding is unavailable |
| `store_memory` | **The sole permanent persistence mechanism.** Saves records to the embedded VectorEngine with optional `type` and `metadata` fields. Use for financial entries, calculation results, and any data that must survive across sessions. |

> **Note:** `write_file` and `create_directory` have been removed from the agent toolkit. All persistent data flows through `store_memory` into the vector database.

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

#### Setup & Dispatch

| Method | Route | Description |
|---|---|---|
| `POST` | `/api/setup` | Configure the hive on first run |
| `POST` | `/api/dispatch` | Submit a goal to the swarm (stateless) |
| `GET` | `/api/tools` | Lists all available tools and their parameters |
| `GET` | `/ws` | Opens a WebSocket connection for real-time events |

#### Sessions

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/sessions` | List all sessions |
| `POST` | `/api/sessions` | Create a new session |
| `GET` | `/api/sessions/:id` | Get session with agents |
| `DELETE` | `/api/sessions/:id` | Delete session (cascade) |
| `POST` | `/api/sessions/:id/dispatch` | Dispatch workflow for a session |
| `GET` | `/api/sessions/:id/agents` | List session agents |
| `POST` | `/api/sessions/:id/documents` | Upload and index a document |

#### Persistent Hives (Colmeias)

Hives are persistent, named entities. Unlike sessions, a hive can receive **multiple messages over time**, carrying conversation history as context for each new dispatch. Agents can be **pre-defined by the user** (with custom prompts and tools, only when `queen_managed=false`) or **assembled automatically by the Queen** (`queen_managed=true`). Attempting to add pre-defined agents to a `queen_managed=true` hive returns `409 Conflict`.

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/colmeias` | List all hives |
| `POST` | `/api/colmeias` | Create hive (`queen_managed: true/false`) |
| `GET` | `/api/colmeias/:id` | Get hive with agents |
| `PUT` | `/api/colmeias/:id` | Update hive |
| `DELETE` | `/api/colmeias/:id` | Delete hive (cascade) |
| `POST` | `/api/colmeias/:id/dispatch` | Send a message to the hive |
| `GET` | `/api/colmeias/:id/historico` | List conversation history |
| `GET` | `/api/colmeias/:id/agentes` | List hive agents |
| `POST` | `/api/colmeias/:id/agentes` | Add pre-defined agent (`queen_managed=false` required) |
| `GET` | `/api/colmeias/:id/agentes/:agentId` | Get agent by ID |
| `PUT` | `/api/colmeias/:id/agentes/:agentId` | Edit agent name, prompt, tools |
| `DELETE` | `/api/colmeias/:id/agentes/:agentId` | Remove agent from hive |

**Example — create a hive with user-defined agents:**

```bash
# 1. Create hive
curl -X POST http://localhost:8080/api/colmeias \
  -H "Content-Type: application/json" \
  -d '{"name": "Research Hive", "queen_managed": false}'

# 2. Add agent with custom prompt
curl -X POST http://localhost:8080/api/colmeias/{id}/agentes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Web Researcher",
    "system_prompt": "You are a research specialist. Use web_search to gather up-to-date information.",
    "allowed_tools": ["web_search", "search_memory", "store_memory"]
  }'

# 3. Send first message
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Find the top AI news from this week"}'

# 4. Send follow-up (previous history injected as context automatically)
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Based on the previous research, write an executive summary"}'
```

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
    { "name": "execute_code", "description": "Compiles and runs Go code in an isolated Wasm sandbox", "parameters": { ... } },
    { "name": "store_memory", "description": "Persists data to the embedded VectorEngine", "parameters": { ... } }
  ]
}
```

#### `GET /api/agents`

```json
// Response 200
{
  "agents": [
    { "name": "Wasm Developer",   "system_prompt": "...", "allowed_tools": ["execute_code", "search_memory"] },
    { "name": "Quality Auditor",  "system_prompt": "...", "allowed_tools": ["execute_code", "store_memory", "read_file"] }
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
