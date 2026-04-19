# Jandaira — App Flow

Jandaira is an AI agent swarm orchestration system. A **Queen** coordinates a pipeline of specialist agents, each powered by an LLM, that work sequentially to complete a goal. All real-time feedback flows through a single WebSocket broadcast channel.

---

## Core Components

```
┌─────────────────────────────────────────────────────────┐
│                        HTTP + WS                        │
│                      (Gin + Gorilla)                    │
└────────────────────────┬────────────────────────────────┘
                         │
                    ┌────▼────┐
                    │  Queen  │  ← orchestrates everything
                    └────┬────┘
          ┌──────────────┼──────────────┐
     ┌────▼────┐   ┌─────▼─────┐  ┌────▼──────┐
     │  Brain  │   │ GroupQueue │  │ Honeycomb │
     │  (LLM)  │   │ (workers)  │  │ (vectors) │
     └─────────┘   └───────────┘  └───────────┘
                         │
                  ┌──────▼──────┐
                  │ Specialists │  ← pipeline agents
                  └─────────────┘
```

| Component       | Role |
|-----------------|------|
| **Queen**        | Meta-planner. Assembles swarms, runs workflows, manages tool approval |
| **Brain**        | LLM interface (OpenAI / Anthropic). Chat, embedding, structured output. OpenAI uses `max_completion_tokens`; Anthropic uses `max_tokens`. |
| **GroupQueue**   | Concurrent job executor. Per-group semaphore, exponential backoff retry |
| **Honeycomb**    | Vector DB (local JSON). Stores embeddings for semantic search |
| **KnowledgeGraph** | Optional graph of specialist→topic relationships. Queen uses it to plan future swarms |
| **ShortTermMemory** | TTL-aware message buffer. Auto-compacts into Honeycomb when full |
| **Security**     | AES session key per workflow. Context encrypted between specialist handoffs |

---

## Skill Architecture

Skills are reusable capability units stored in the `skills` table. They can be attached to hives or individual agents via many-to-many junction tables (`colmeia_skills`, `agente_colmeia_skills`).

```
┌────────────────────────────────────────────────────────────────┐
│                        Skill Registry                          │
│  id | name           | instructions            | allowed_tools │
│  1  | Web Research   | Use web_search to...    | [web_search]  │
│  2  | Code Analysis  | Read files and analyze  | [read_file]   │
└────────────────────────────────────────────────────────────────┘
         │  many2many              │  many2many
         ▼                         ▼
   colmeia_skills          agente_colmeia_skills
   (colmeia ↔ skill)       (agente ↔ skill)
```

**At dispatch time:**

| Hive type        | Skills source          | How injected |
|------------------|------------------------|--------------|
| `queen_managed`  | `colmeia.Skills`       | `SKILLS DISPONÍVEIS` block prepended to enrichedGoal → Queen reads during meta-planning |
| `user_defined`   | `agente.Skills`        | `instructions` appended to `SystemPrompt`; `allowed_tools` merged (deduplication) in `BuildSpecialists` |

---

## Startup Flow

```
main()
  ├── Load config (DB + env)
  ├── Run DB migrations (GORM auto-migrate)
  ├── Init Brain (OpenAI client)
  ├── Init Honeycomb (LocalVectorDB from disk)
  ├── Init KnowledgeGraph (optional, from disk)
  ├── Init GroupQueue (maxConcurrent from config)
  ├── NewQueen(queue, brain, honeycomb)
  ├── Queen.EquipTool(list_directory, read_file, execute_code, search_memory, store_memory, web_search)
  ├── Queen.RegisterSwarm(groupID, Policy{MaxNectar, Isolate, RequiresApproval})
  └── api.NewServer(queen, port, services...)
        ├── Wire Queen callbacks → WS broadcast
        │     LogFunc, AgentChangeFunc, ToolStartFunc, AskPermissionFunc
        └── server.Start()  →  Gin HTTP server
```

---

## Setup Flow (first run)

```
Client  POST /api/setup { api_key, model, swarm_name, ... }
          │
          ▼
  configService.Save(config)
  Queen.RegisterSwarm(swarm_name, Policy)
          │
          ▼
  All other endpoints unlocked (setupMiddleware checks IsConfigured)
```

Unconfigured requests to any other endpoint return `428 Precondition Required`.

---

## WebSocket Connection

```
Client  GET /ws  →  HTTP upgrade  →  persistent WS connection
                                         │
                                         ▼
                                  server.clients[conn] = true

Queen callbacks fire on every swarm event:
  LogFunc(msg)                 →  Broadcast { type: "log",           message }
  AgentChangeFunc(name)        →  Broadcast { type: "agent_change",  agent }
  ToolStartFunc(agent,tool,args) → Broadcast { type: "tool_start",   agent, tool, args }
  AskPermissionFunc(tool,args) →  Broadcast { type: "approval_request", id, tool, args }
                                             + blocks on ApprovalChan

Client sends:
  { type: "approve", id: "req-...", approved: true/false }
    │
    ▼
  server.ApprovalChan ← approved
  workflow resumes (or aborts if denied)
```

All connected clients receive the same broadcast. No per-session filtering.

---

## Stateless Dispatch Flow

`POST /api/dispatch { goal, group_id }`

```
handleDispatch
  │
  ├── Queen.AssembleSwarm(ctx, goal, maxWorkers)
  │     ├── Build system prompt with available tools + KnowledgeGraph context
  │     ├── Brain.Chat(ctx, [system, user:goal], nil)
  │     └── Parse JSON → []Specialist { Name, SystemPrompt, AllowedTools }
  │
  ├── Queen.DispatchWorkflow(ctx, groupID, goal, specialists)
  │     └── GroupQueue.Submit(job)  →  runs async
  │
  └── HTTP 202 Accepted  (client follows via WS)
```

---

## DispatchWorkflow — Core Execution Loop

```
contextAccumulator = "Original Goal: {goal}"
sessionKey = AES key (generated per workflow)

for each specialist in pipeline:
  ├── fire AgentChangeFunc  →  WS "agent_change"
  ├── Seal(sessionKey, contextAccumulator)  →  encryptedPayload
  ├── runSpecialist(specialist, encryptedPayload, sessionKey, policy)
  │     ├── Open(sessionKey, payload)  →  taskContext
  │     ├── messages = [SystemPrompt, user:taskContext]
  │     └── reflection loop (max 10 turns):
  │           ├── Brain.Chat(ctx, messages, availableTools)
  │           ├── if no tool calls → task done, Seal(response) → return
  │           ├── for each tool call:
  │           │     ├── if policy.RequiresApproval:
  │           │     │     ├── AskPermissionFunc  →  WS "approval_request"
  │           │     │     └── wait ApprovalChan
  │           │     ├── if approved → ToolStartFunc  →  WS "tool_start"
  │           │     │                  tool.Execute(ctx, args)
  │           │     └── if denied  → inject error into messages
  │           └── append [assistant:response, tool:result] to messages
  │
  ├── Open(sessionKey, encryptedOutput)  →  decryptedOutput
  └── contextAccumulator += "\n--- Report from {name} ---\n{output}"

after all specialists:
  ├── Honeycomb.Store(ctx, groupID, docID, embed(accumulator), metadata)
  ├── registerWorkflowInGraph(ctx, goal, pipeline)
  └── resultChan ← contextAccumulator
```

Context grows with each specialist. Each agent sees the full output of all previous agents.

---

## Session Flow

Sessions persist the swarm execution in the DB with full agent tracking.

```
POST /api/sessions { name, goal }
  └── sessionService.Create()  →  Session { id, status: "active" }

POST /api/sessions/:id/dispatch { goal? }
  ├── sessionService.GetSession(id)
  ├── if status completed/failed → 409 Conflict
  ├── Queen.AssembleSwarm(ctx, goal, maxWorkers)
  ├── for each specialist:
  │     sessionService.CreateAgent(sessionID, name, role:"specialist")
  │     Broadcast { type: "agent_created", agent_data }
  ├── Queen.DispatchWorkflow(ctx, groupID, goal, specialists)  →  async
  │
  └── HTTP 202

on WS result  →  sessionService.CompleteSession(id, result)
on WS error   →  sessionService.FailSession(id)
```

---

## Skill-Aware Colmeia Dispatch

```
POST /api/colmeias/:id/dispatch { goal }
  │
  ├── 1. Load colmeia + agents + skills from DB
  │        (Preload: Colmeia.Skills, Agentes, Agentes.Skills)
  │
  ├── 2. BuildGoalWithHistory → inject last 3 completed dispatches
  │
  ├── 3. BuildSkillsContext (if colmeia.Skills not empty)
  │        → prepend "SKILLS DISPONÍVEIS..." block to enrichedGoal
  │
  ├── 4. Honeycomb semantic search → append top-3 relevant past results
  │
  ├── 5. queen_managed=true  → Queen.AssembleSwarm(enrichedGoal)
  │        Queen reads SKILLS block and assigns skill instructions/tools
  │        to each specialist it creates
  │
  └── 6. queen_managed=false → BuildSpecialists(colmeia)
           For each agent: merge agent.Skills[i].Instructions into SystemPrompt
                           merge agent.Skills[i].AllowedTools into AllowedTools
```

---

## Colmeia Agent Management

Pre-defined agents are only allowed in `queen_managed=false` hives.

```
POST /api/colmeias/:id/agentes { name, system_prompt, allowed_tools }
  │
  ├── Load colmeia from DB
  ├── if colmeia.QueenManaged == true → 409 Conflict
  │     "Queen-managed hive does not accept pre-defined agents"
  └── colmeiaService.AddAgente(colmeiaID, name, systemPrompt, allowedTools)
        └── 201 Created { agente }

GET /api/colmeias/:id/agentes/:agentId
  └── colmeiaService.GetAgente(agenteID)
        ├── 200 OK { agente with skills }
        └── 404 if not found
```

---

## Colmeia Flow

Colmeias are persistent named hives. Multiple dispatches accumulate memory.

```
POST /api/colmeias { name, queen_managed }
  └── colmeiaService.CreateColmeia()

POST /api/colmeias/:id/dispatch { goal }
  │
  ├── 1. Load colmeia + agents from DB
  │
  ├── 2. BuildGoalWithHistory(colmeia, goal)
  │     └── historico.FindRecentCompleted(colmeiaID, 3)
  │           → prepend last 3 completed dispatches as context
  │
  ├── 3. Honeycomb semantic search (if enabled)
  │     ├── Brain.Embed(ctx, goal)  →  queryVector
  │     ├── Honeycomb.Search(ctx, colmeiaID, queryVector, 3)
  │     │     cosine similarity > 0.7 threshold
  │     └── append top matches to enrichedGoal
  │
  ├── 4. colmeiaService.CreateHistorico(colmeiaID, originalGoal)
  │
  ├── 5. groupID = colmeiaID  ← isolated vector memory per hive
  │
  ├── 6. Build pipeline
  │     ├── queen_managed=true  → Queen.AssembleSwarm(enrichedGoal)
  │     └── queen_managed=false → BuildSpecialists(colmeia.Agentes)
  │
  ├── 7. Queen.DispatchWorkflow(ctx, colmeiaID, enrichedGoal, pipeline)  →  async
  │
  └── HTTP 202 { colmeia_id, historico_id, agents, mode }

on WS result  →  colmeiaService.CompleteHistorico(id, result)
                 Honeycomb.Store(ctx, colmeiaID, ...)  ← stored in hive's own collection
on WS error   →  colmeiaService.FailHistorico(id)
```

---

## Memory Architecture

Jandaira has three memory layers:

### Layer 1 — DB History (HistoricoDespacho)
- Stores every dispatch: original goal, result, status
- Queried by `BuildGoalWithHistory` on each new colmeia dispatch
- Last 3 completed dispatches injected as conversation context
- Persists forever across restarts

### Layer 2 — Honeycomb (Vector DB — Qdrant)
- Qdrant backend via gRPC (port 6334). **Primary and sole persistence target for agent data.**
- After each workflow completes, the full accumulated context is embedded and stored
- Scoped per `groupID` (= `colmeiaID` for hives)
- On new colmeia dispatch: semantic search for top-3 relevant past results (cosine > 0.7)
- Also used by ShortTermMemory for compacted archives
- `store_memory` tool writes here; agents have no access to the file system for writes
- When embedding fails (e.g. Anthropic provider without OpenAI key), a fallback uniform vector is used — data is never lost, semantic search is degraded for that record

### Layer 3 — ShortTermMemory
- TTL-aware in-RAM message buffer
- When full (maxEntries) or expired: LLM summarises into one paragraph → stored in Honeycomb
- Prevents context window overflow in long-running specialists

### Layer 4 — KnowledgeGraph (optional)
- Graph of `agent` → `expert_in` → `topic`, `agent` → `uses_tool` → `tool`
- Populated after each workflow via `registerWorkflowInGraph`
- Queen queries it during `AssembleSwarm` to inform specialist creation

---

## Security Model

Each workflow generates a fresh AES key (`security.GenerateKey()`).  
The `contextAccumulator` is encrypted before being passed to each specialist and decrypted after the response returns. This prevents tool output from being read outside the intended specialist scope.

```
contextAccumulator  →  Seal(sessionKey)  →  encryptedPayload
                                                    │
                                             runSpecialist
                                                    │
encryptedOutput  →  Open(sessionKey)  →  decryptedOutput
```

---

## Data Flow Summary

```
User goal
  │
  ▼
[API Handler]
  ├── Enrich with DB history (colmeia only)
  ├── Enrich with semantic memory (colmeia + Honeycomb)
  └── CreateHistorico / CreateSession
  │
  ▼
[Queen.AssembleSwarm]  ←  Brain.Chat (meta-planning)
  └── []Specialist { Name, SystemPrompt, AllowedTools }
  │
  ▼
[GroupQueue.Submit]  ←  runs in goroutine
  │
  ▼
[DispatchWorkflow]
  └── for each specialist:
        encrypt context → runSpecialist → decrypt output
              │
              └── Brain.Chat ↔ tool.Execute (loop max 10x)
                       │
                       WS: agent_change, tool_start, approval_request
  │
  ▼
[Result]
  ├── Honeycomb.Store (embed full context)
  ├── KnowledgeGraph.register
  ├── DB: CompleteHistorico / CompleteSession
  └── WS: { type: "result", message: fullContext }
```
