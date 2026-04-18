# Jandaira — API Integration Guide

Base URL: `http://localhost:8080`  
Real-time events: see [`websocket.md`](./websocket.md)  
Full schema reference: see [`openapi.yaml`](./openapi.yaml)

---

## Prerequisites

All endpoints except `POST /api/setup` require the hive to be configured.  
Unconfigured requests return **428 Precondition Required**.

**Integration flow:**
1. Call `GET` any protected endpoint → `428` means setup is needed
2. Call `POST /api/setup` once
3. All other endpoints are now unlocked

---

## Setup

### `POST /api/setup`

Initialises the hive. Call once on first run.

**Request**
```json
{
  "api_key": "sk-...",
  "model": "gpt-4o-mini",
  "swarm_name": "my-swarm",
  "max_nectar": 20000,
  "max_agents": 5,
  "supervised": false,
  "isolated": false,
  "language": "en-US"
}
```

| Field        | Type    | Required | Default       | Notes |
|--------------|---------|:--------:|---------------|-------|
| `api_key`    | string  | yes      | —             | OpenAI API key, stored in local vault |
| `model`      | string  | no       | `gpt-4o-mini` | Any OpenAI model identifier |
| `swarm_name` | string  | no       | `enxame-alfa` | Default swarm group label |
| `max_nectar` | integer | no       | `20000`       | Token budget per workflow |
| `max_agents` | integer | no       | —             | Max concurrent agents |
| `supervised` | boolean | no       | `false`       | Require human approval on every tool call |
| `isolated`   | boolean | no       | `false`       | Agents run without shared context |
| `language`   | string  | no       | —             | Preferred response language (e.g. `pt-BR`) |

**Responses**

| Status | Meaning |
|--------|---------|
| `200`  | Configured successfully |
| `400`  | Missing `api_key` |
| `403`  | Already configured |
| `500`  | Internal error |

---

## Stateless Dispatch

One-shot workflow. No persistence. Progress via WebSocket.

### `POST /api/dispatch`

**Request**
```json
{
  "goal": "Research the top 5 AI frameworks and summarise their pros and cons",
  "group_id": "my-swarm"
}
```

| Field      | Required | Notes |
|------------|:--------:|-------|
| `goal`     | yes      | Mission objective |
| `group_id` | no       | Swarm group label. Falls back to configured `swarm_name` |

**Response — `202 Accepted`**
```json
{ "message": "Mission dispatched to the swarm. Follow progress via WebSocket." }
```

**WS events fired:** `log` → `agent_change` → `tool_start` → (`approval_request`) → `result` / `error`

---

## Catalog

Read-only. No side effects.

### `GET /api/tools`

Returns all tools registered in the Queen with their name, description, and JSON Schema parameters.

**Response — `200`**
```json
{
  "tools": [
    {
      "name": "web_search",
      "description": "Search the web for up-to-date information",
      "parameters": { "type": "object", "properties": { "query": { "type": "string" } } }
    }
  ]
}
```

Use the `name` values to populate the `allowed_tools` field when creating colmeia agents.

### `GET /api/agents`

Returns registered agents. Currently always returns an empty array (reserved).

---

## Sessions

Single-run persistent sessions. Agents are assembled per dispatch and stored.

### `GET /api/sessions`

Lists all sessions, newest first. Agent details omitted.

**Response — `200`**
```json
{
  "sessions": [ { "id": "uuid", "name": "...", "goal": "...", "status": "active", "created_at": "...", "updated_at": "..." } ],
  "total": 1
}
```

Status values: `active` | `completed` | `failed`

---

### `POST /api/sessions`

Creates a session. Does not start the workflow — call dispatch separately.

**Request**
```json
{ "name": "Weekly report", "goal": "Generate the Q2 performance report" }
```

| Field  | Required | Notes |
|--------|:--------:|-------|
| `goal` | yes      | Mission objective |
| `name` | no       | Human-readable label |

**Response — `201`**
```json
{
  "message": "...",
  "session": { "id": "uuid", "name": "Weekly report", "goal": "...", "status": "active", ... }
}
```

---

### `GET /api/sessions/:id`

Returns full session including all associated agents.

**Response — `200`**
```json
{
  "id": "uuid",
  "name": "...",
  "goal": "...",
  "status": "completed",
  "result": "Full workflow output...",
  "agents": [ { "id": 1, "name": "ResearchSpecialist", "role": "specialist", "status": "done", ... } ],
  "created_at": "...",
  "updated_at": "..."
}
```

---

### `DELETE /api/sessions/:id`

Permanently deletes session and all its agents (cascade).

**Response — `200`**
```json
{ "message": "..." }
```

---

### `POST /api/sessions/:id/dispatch`

Starts the swarm for the session asynchronously. The workflow runs in background; follow progress via WebSocket.

**Request** *(optional body)*
```json
{ "goal": "Focus only on the revenue section" }
```

Omit body to use the session's stored goal.

**Response — `202`**
```json
{ "message": "...", "session_id": "uuid", "agents": 3 }
```

**Error cases**

| Status | Reason |
|--------|--------|
| `409`  | Session already `completed` or `failed` — create a new session |
| `404`  | Session not found |

**WS events fired:** `agent_created` (one per specialist) → `log` → `agent_change` → `tool_start` → (`approval_request`) → `result` / `error`

---

### `GET /api/sessions/:id/agents`

Returns all agents created for this session.

**Response — `200`**
```json
{
  "agents": [ { "id": 1, "name": "ResearchSpecialist", "role": "specialist", "status": "done", ... } ],
  "total": 3
}
```

Agent status values: `idle` | `working` | `done` | `failed`

---

### `POST /api/sessions/:id/documents`

Uploads a document, extracts text, splits into chunks, embeds each chunk, and stores vectors in the hive's vector DB. Session agents can then retrieve them with the `search_memory` tool.

**Request** — `multipart/form-data`

| Field        | Required | Notes |
|--------------|:--------:|-------|
| `file`       | yes      | `.pdf`, `.docx`, `.txt`, `.csv`, `.xlsx` — max 32 MB |

**Query params**

| Param        | Required | Default | Notes |
|--------------|:--------:|---------|-------|
| `collection` | no       | `swarm_name` | Vector DB collection name |

**Response — `201`**
```json
{
  "message": "...",
  "filename": "report.pdf",
  "workspace_path": "workspace/sessions/{id}/report.txt",
  "chunks": 12,
  "collection": "enxame-alfa",
  "session_id": "uuid"
}
```

**Error cases**

| Status | Reason |
|--------|--------|
| `400`  | No `file` field in request |
| `422`  | Unsupported extension or document has no extractable text |

Fires a `status` WS event on success: `📄 Document 'X' indexed: N/M chunks saved.`

---

## Colmeias (Persistent Hives)

Reusable hives with persistent agents and full conversation memory. Each dispatch automatically enriches the goal with:

1. **DB history** — last 3 completed dispatches prepended as context
2. **Semantic memory** — if Honeycomb (vector DB) is enabled, top-3 semantically similar past results (cosine score > 0.7) are appended

Memory is scoped per hive (keyed by hive ID). No cross-hive leakage.

---

### `GET /api/colmeias`

Lists all hives.

**Response — `200`**
```json
{
  "colmeias": [ { "id": "uuid", "name": "Research Hive", "description": "...", "queen_managed": true, ... } ],
  "total": 2
}
```

---

### `POST /api/colmeias`

Creates a persistent hive.

**Request**
```json
{ "name": "Research Hive", "description": "General-purpose research", "queen_managed": true }
```

| Field          | Required | Default | Notes |
|----------------|:--------:|---------|-------|
| `name`         | yes      | —       | Human-readable label |
| `description`  | no       | —       | Optional description |
| `queen_managed`| no       | `true`  | `true` = Queen assembles agents dynamically; `false` = use pre-defined agents |

**Response — `201`**
```json
{ "message": "...", "colmeia": { "id": "uuid", "name": "...", "queen_managed": true, ... } }
```

---

### `GET /api/colmeias/:id`

Returns hive with its pre-defined agents.

**Response — `200`**
```json
{
  "id": "uuid",
  "name": "Research Hive",
  "queen_managed": false,
  "agentes": [
    { "id": 1, "colmeia_id": "uuid", "name": "Analyst", "system_prompt": "...", "allowed_tools": "[\"web_search\"]", ... }
  ],
  ...
}
```

`allowed_tools` is a JSON-encoded string — parse it on the client.

---

### `PUT /api/colmeias/:id`

Updates hive fields.

**Request** — same shape as `POST /api/colmeias`

**Response — `200`**
```json
{ "message": "...", "colmeia": { ... } }
```

---

### `DELETE /api/colmeias/:id`

Permanently deletes hive, all its agents, and full dispatch history (cascade).

**Response — `200`**
```json
{ "message": "..." }
```

---

### `POST /api/colmeias/:id/dispatch`

Sends a new message/goal to the hive. Core interaction endpoint.

**Request**
```json
{ "goal": "Summarise the latest AI research papers" }
```

**Context enrichment** (automatic, server-side):
- Last 3 completed dispatches from DB injected before the goal
- Honeycomb semantic search run against the goal; top matches appended

**Response — `202`**
```json
{
  "message": "...",
  "colmeia_id": "uuid",
  "historico_id": "uuid",
  "agents": 3,
  "mode": "queen_managed"
}
```

`mode` values: `queen_managed` | `user_defined`

**Error cases**

| Status | Reason |
|--------|--------|
| `400`  | `queen_managed=false` but no agents defined |
| `404`  | Hive not found |

**WS events fired:** `status` → `log` → `agent_change` → `tool_start` → (`approval_request`) → `result` / `error`

Store `historico_id` to correlate the WS `result` event with this specific dispatch.

---

### `GET /api/colmeias/:id/historico`

Returns all past dispatches for the hive, newest first.

**Response — `200`**
```json
{
  "historico": [
    {
      "id": "uuid",
      "colmeia_id": "uuid",
      "goal": "Summarise the latest AI research papers",
      "result": "Full workflow output...",
      "status": "completed",
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "total": 5
}
```

`goal` is always the original user message — never contains injected context.  
`result` is populated after status reaches `completed`.  
Status values: `active` | `completed` | `failed`

---

### `GET /api/colmeias/:id/agentes`

Lists pre-defined agents for the hive.

**Response — `200`**
```json
{
  "agentes": [ { "id": 1, "name": "Analyst", "system_prompt": "...", "allowed_tools": "[\"web_search\"]", ... } ],
  "total": 2
}
```

---

### `POST /api/colmeias/:id/agentes`

Adds a pre-defined agent to the hive. Only relevant when `queen_managed=false`.

**Request**
```json
{
  "name": "Code Reviewer",
  "system_prompt": "You are an expert Go code reviewer. Read files and provide detailed feedback.",
  "allowed_tools": ["read_file", "list_directory", "web_search"]
}
```

| Field           | Required | Notes |
|-----------------|:--------:|-------|
| `name`          | yes      | Agent display name |
| `system_prompt` | yes      | Full instructions for this agent |
| `allowed_tools` | no       | Array of tool names from `GET /api/tools` |

**Response — `201`**
```json
{ "message": "...", "agente": { "id": 1, "name": "Code Reviewer", ... } }
```

---

### `PUT /api/colmeias/:id/agentes/:agentId`

Updates an agent's name, system prompt, and allowed tools.  
Changes take effect on the next dispatch.

**Request** — same shape as `POST /api/colmeias/:id/agentes`

**Response — `200`**
```json
{ "message": "...", "agente": { ... } }
```

---

### `DELETE /api/colmeias/:id/agentes/:agentId`

Removes an agent from the hive.

**Response — `200`**
```json
{ "message": "..." }
```

---

## Common Error Responses

All errors follow the same shape:

```json
{ "error": "Human-readable error message." }
```

| Status | Meaning |
|--------|---------|
| `400`  | Invalid or missing request parameters |
| `403`  | Action forbidden (e.g. setup already done) |
| `404`  | Resource not found |
| `409`  | Conflict (e.g. session already finished) |
| `422`  | Unprocessable entity (e.g. unsupported file format) |
| `428`  | Hive not configured — call `POST /api/setup` first |
| `500`  | Internal server error |

---

## Typical Integration Flows

### A. First run

```
POST /api/setup        → configure hive
GET  /api/tools        → fetch tool list for UI selectors
GET  /ws               → open WebSocket, keep alive
```

### B. One-shot mission

```
POST /api/dispatch     → fire and forget
WS   result/error      → display output
```

### C. Persistent hive conversation

```
POST /api/colmeias                     → create hive (once)
POST /api/colmeias/:id/agentes         → add agents if queen_managed=false (once)

# Each user message:
POST /api/colmeias/:id/dispatch        → send goal, get historico_id
WS   result                            → display result
GET  /api/colmeias/:id/historico       → refresh conversation history
```

### D. Supervised workflow (human-in-the-loop)

```
POST /api/setup { "supervised": true } → enable supervision

POST /api/colmeias/:id/dispatch        → start workflow
WS   approval_request                  → show approval modal
WS   send { type: "approve", ... }     → user approves/denies
WS   tool_start                        → tool executes
WS   result                            → workflow done
```
