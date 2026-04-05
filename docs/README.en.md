# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="../logo.png" alt="Jandaira Logo" width="220"/>
</p>

A **multi-agent autonomous framework** written in Go, inspired by the collective intelligence of the native Brazilian bee *Melipona subnitida* — the **Jandaíra**.

---

> 🌐 **English** · [Português](../README.md) · [中文](README.zh.md) · [Русский](README.ru.md)

> 📦 [**Download pre-built binaries**](https://github.com/damiaoterto/jandaira/releases) — Linux, Windows, macOS and Raspberry Pi

---

## 📖 Why "Jandaira"?

The **Jandaíra** (*Melipona subnitida*) is a stingless bee endemic to the Caatinga biome of Brazil. Small, resilient, and extraordinarily cooperative — it doesn't need a centralized leader to build a functional hive. Each worker knows its role, executes its task autonomously, and returns the result to the collective.

This is exactly the architectural model this project implements:

- The **Queen (`Queen`)** does not execute tasks — she orchestrates, validates policies, and ensures security.
- The **Specialists (`Specialists`)** are lightweight agents with restricted tools, executing in isolated silos.
- **Nectar** is the metaphor for the token budget: each agent consumes nectar; when it runs out, the hive stops.
- The **Honeycomb (`Honeycomb`)** is the shared vector memory — collective knowledge that persists between missions.
- The **Beekeeper** is the human in the loop: they can approve or block any AI action before it is executed.

---

## 🏗️ Architecture

### Flow Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI (Bubble Tea)                         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  👤 User types objective  →  👑 Queen receives the goal │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ DispatchWorkflow()
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Queen (Orchestrator)                          │
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
│   RequiresApproval=true → UI pauses and shows request    │
│   Y = authorize tool │ N = block and inform AI           │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│                   🍯 Honeycomb (Vector DB)                │
│   Workflow result is embedded and indexed                 │
│   Long-term memory shared between missions               │
└──────────────────────────────────────────────────────────┘
```

### Package Map

```
jandaira/
├── cmd/
│   └── cli/
│       └── main.go          # Entrypoint: assembles the hive and starts the UI
│
└── internal/
    ├── brain/               # AI contracts (Brain, Honeycomb)
    │   ├── open_ai.go       # OpenAI implementation (Chat + Embed)
    │   └── local_vector.go  # Local Vector DB (JSON embeddings)
    │
    ├── queue/               # FIFO scheduler with limited concurrency
    │   └── group_queue.go   # GroupQueue: N workers per group
    │
    ├── security/            # Inter-agent payload encryption
    │   └── crypto.go        # AES-GCM Seal/Open + key generation
    │
    ├── swarm/               # Core agent system
    │   ├── queen.go         # Orchestrator: policies, HIL, pipeline
    │   └── specialist.go    # Specialist definition
    │
    ├── tool/                # Tools available to agents
    │   ├── list_directory.go
    │   ├── search_memory.go
    │   └── wasm.go          # Execution sandbox via wazero
    │
    └── ui/
        └── cli.go           # Bubble Tea interface (TUI)
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
| **Human-in-the-Loop** | Optional / external | ✅ Native: Beekeeper mode |
| **Token budget** | Manual | ✅ Automatic `NectarUsage` per swarm |
| **Vector memory** | Pinecone / external | ✅ Embedded (local, no server) |
| **Deploy** | Multiple services | ✅ Single static binary |
| **TUI interface** | Nonexistent | ✅ Bubble Tea with Lipgloss styles |
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

# Set your OpenAI key
export OPENAI_API_KEY="sk-..."
```

### Installation

#### Option 1 — Download pre-built binary *(recommended)*

Visit the [**Releases**](https://github.com/damiaoterto/jandaira/releases) page and download the binary for your system:

| System | File |
|---|---|
| Linux x86-64 | `jandaira-linux` |
| Windows | `jandaira-windows.exe` / `jandaira-setup.exe` |
| macOS | `jandaira-macos` |
| Raspberry Pi 4/5 | `jandaira-linux-arm64` |
| Raspberry Pi 2/3 | `jandaira-linux-armv7` |

```bash
# Linux/macOS: make it executable
chmod +x jandaira-linux
./jandaira-linux
```

#### Option 2 — Build from source

```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira

# Download dependencies
go mod tidy

# Build
go build -o jandaira ./cmd/cli/
```

### Run the hive

```bash
./jandaira
```

You will see the Jandaira TUI panel:

```
╔══════════════════════════════════╗
║   🍯  Jandaira Swarm OS  🍯       ║
║   Swarm Intelligence · Powered by Go ║
╚══════════════════════════════════╝

✦ The Jandaira Hive has awakened. The workers await your orders.

╭──────────────────────────────────────╮
│ 🐝 Objective  Tell the Queen what... │
╰──────────────────────────────────────╯
  ↵ send   esc / ctrl+c quit
```

### Example: create and test a Go file

1. Type your objective in the input field and press **Enter**:

   ```
   Create a Go file called sum.go that adds two numbers and prints the result
   ```

2. The Queen distributes the task to the Specialist pipeline:
   - **Wasm Developer** → writes `sum.go` using `write_file`
   - **Quality Auditor** → executes the code with `execute_code` and generates a report

3. If `RequiresApproval: true`, **Beekeeper mode** is activated at each tool use:

   ```
   ┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
   ⠿  ⚠️  The AI wants to use the tool 'write_file'

   ▸ filename:  sum.go
   ▸ content:
     package main

     import "fmt"

     func main() {
         fmt.Println(1 + 2)
     }

   👨‍🌾 Do you authorize? (Y = yes / N = no)
   ┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
   ```

   - Press **Y** to authorize — the Queen continues
   - Press **N** to block — the AI is informed and recalculates its approach

4. At the end, the report is displayed in the history and saved to local vector memory (`.jandaira/data`).

### Configure your own swarm

Edit `cmd/cli/main.go` to define your own Specialists and policy:

```go
// Swarm policy
queen.RegisterSwarm("my-swarm", swarm.Policy{
    MaxNectar:        50000,  // Token budget
    Isolate:          true,   // Isolated context per group
    RequiresApproval: true,   // Beekeeper mode (HIL)
})

// Specialists in pipeline
researcher := swarm.Specialist{
    Name: "Researcher",
    SystemPrompt: `You are a researcher. Use search_memory to find
                   relevant context and return a detailed summary.`,
    AllowedTools: []string{"search_memory"},
}

writer := swarm.Specialist{
    Name: "Writer",
    SystemPrompt: `You are a technical writer. Based on the received summary,
                   use write_file to create a Markdown report.`,
    AllowedTools: []string{"write_file"},
}

workflow := []swarm.Specialist{researcher, writer}
```

### Available tools

| Tool | Description |
|---|---|
| `list_directory` | Lists files and folders in a directory |
| `read_file` | Reads the content of a file |
| `write_file` | Creates or overwrites a file |
| `execute_code` | Executes code in an isolated Wasm sandbox |
| `search_memory` | Semantic search in the hive's vector memory |
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

## 🌐 API Reference (Server Mode)

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

All events are exchanged as JSON over the same WebSocket channel. The Beekeeper **no longer needs REST routes** — approvals are handled entirely via WebSocket.

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
// Example events received by the frontend:
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
// Approve the action:
{ "type": "approve", "id": "req-1712345678901", "approved": true }

// Deny the action:
{ "type": "approve", "id": "req-1712345678901", "approved": false }
```

> **Note:** The `id` field must exactly match the `id` received in the `approval_request`. Invalid or already-processed IDs return an `error` event.

---

## 🛣️ Roadmap

- [ ] Web Interface (Svelte + `go:embed`)
- [ ] Multi-LLM support (Anthropic, Gemini, Ollama)
- [x] Full Wasm sandbox per agent (isolated VFS via `wazero`)
- [ ] Nectar metrics dashboard (cost per mission)
- [ ] PDF/Markdown report export

---

## 🤝 Contributing

Pull Requests are welcome! Open an issue describing the feature or bug before starting.

---

*Jandaira: Autonomy, Security, and the Power of the Brazilian Swarm.* 🐝
