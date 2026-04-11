# WebSocket — Real-time Event Stream

All real-time events from the queen and agents are delivered over a single persistent WebSocket connection.

**Endpoint:** `GET /ws`  
**Protocol upgrade:** HTTP → WebSocket (RFC 6455)  
**Message encoding:** JSON (UTF-8)

> The connection requires no authentication. All connected clients receive the same broadcast stream.

---

## Connecting

```js
const ws = new WebSocket("ws://localhost:8080/ws");

ws.addEventListener("message", (event) => {
  const msg = JSON.parse(event.data);
  console.log(msg.type, msg);
});
```

---

## Message Schema

Every message — both inbound and outbound — follows the same `WsMessage` shape:

```ts
interface Agent {
  id: number;
  session_id: string;
  name: string;
  role: string;
  status: "idle" | "working" | "done" | "failed";
  created_at: string; // ISO 8601
  updated_at: string; // ISO 8601
}

interface WsMessage {
  type: string;        // event identifier (see tables below)
  id?: string;         // approval request ID
  message?: string;    // human-readable text
  tool?: string;       // tool name
  args?: string;       // JSON-encoded tool arguments
  agent?: string;      // agent/specialist name
  agent_data?: Agent;  // agent_created: full agent record
  approved?: boolean;  // inbound only: true = approved, false = denied
}
```

Fields not relevant to a given event type are omitted from the payload.

---

## Server → Client events

These are the events the server broadcasts to every connected client.

### `status`

Generic progress message emitted by the queen or a dispatch handler.

```json
{ "type": "status", "message": "🚀 Queen received the goal and is starting the swarm..." }
```

| Field     | Always present | Description |
|-----------|:--------------:|-------------|
| `message` | yes            | Human-readable progress text |

---

### `log`

Raw internal log line emitted by the queen during planning or workflow execution. Useful for detailed debugging dashboards.

```json
{ "type": "log", "message": "🧠 The Queen is consulting the manuals and designing the swarm architecture..." }
{ "type": "log", "message": "👑 Swarm planned with 3 super-trained bees: Researcher -> Writer -> Auditor" }
{ "type": "log", "message": "👑 [Queen] Passing the baton to: Writer" }
{ "type": "log", "message": "✅ [Writer] Task complete." }
{ "type": "log", "message": "👑 [Queen] Workflow complete! The swarm has finished." }
```

| Field     | Always present | Description |
|-----------|:--------------:|-------------|
| `message` | yes            | Log line (may contain emoji prefixes for context) |

> **Difference from `status`:** `status` events are emitted intentionally at key milestones (dispatch start, workflow result). `log` events are the queen's raw stdout, providing full internal visibility.

---

### `agent_created`

Fired once for each specialist after it has been persisted to the database. Emitted during session dispatch, before the workflow begins.

```json
{
  "type": "agent_created",
  "agent_data": {
    "id": 7,
    "session_id": "d4e5f6a7-...",
    "name": "ResearchSpecialist",
    "role": "specialist",
    "status": "idle",
    "created_at": "2026-04-11T14:00:00Z",
    "updated_at": "2026-04-11T14:00:00Z"
  }
}
```

| Field        | Always present | Description |
|--------------|:--------------:|-------------|
| `agent_data` | yes            | Full agent record as stored in the database |

---

### `agent_change`

Fired when the pipeline baton is passed to a new specialist agent.

```json
{ "type": "agent_change", "agent": "ResearchSpecialist" }
```

| Field   | Always present | Description |
|---------|:--------------:|-------------|
| `agent` | yes            | Name of the specialist now in control |

---

### `tool_start`

Fired immediately before an agent executes a tool (after approval, if required).

```json
{
  "type": "tool_start",
  "agent": "ResearchSpecialist",
  "tool": "web_search",
  "args": "{\"query\":\"latest AI frameworks 2026\"}"
}
```

| Field   | Always present | Description |
|---------|:--------------:|-------------|
| `agent` | yes            | Specialist executing the tool |
| `tool`  | yes            | Tool name |
| `args`  | yes            | JSON-encoded arguments passed to the tool |

---

### `approval_request`

Emitted when the queen needs the Beekeeper (human) to authorize a tool call before it runs. Only fires when the hive is configured with `supervised: true`.

```json
{
  "type": "approval_request",
  "id": "req-1712836800000000000",
  "tool": "write_file",
  "args": "{\"filename\":\"report.md\",\"content\":\"# Report\\n...\"}"
}
```

| Field  | Always present | Description |
|--------|:--------------:|-------------|
| `id`   | yes            | Unique request ID — must be echoed back in the `approve` response |
| `tool` | yes            | Tool the agent wants to execute |
| `args` | yes            | JSON-encoded arguments for that tool |

> The workflow **pauses** until the client sends an `approve` message. If no response arrives, the workflow remains blocked indefinitely. Ensure your UI always gives the user a path to approve or deny.

---

### `result`

Emitted when the full workflow completes successfully. Contains the accumulated output of all specialists.

```json
{ "type": "result", "message": "Original Goal: ...\n\n--- Report from Researcher ---\n...\n--- Report from Writer ---\n..." }
```

| Field     | Always present | Description |
|-----------|:--------------:|-------------|
| `message` | yes            | Full multi-agent workflow report |

---

### `error`

Emitted when any stage fails or the workflow times out (10-minute limit).

```json
{ "type": "error", "message": "worker 'Writer' failed: context deadline exceeded" }
{ "type": "error", "message": "Mission timeout reached." }
{ "type": "error", "message": "Invalid or already processed approval ID." }
```

| Field     | Always present | Description |
|-----------|:--------------:|-------------|
| `message` | yes            | Error description |

---

## Client → Server events

The client can send messages back to the server over the same connection.

### `approve`

Responds to a pending `approval_request`. The `id` field must exactly match the `id` received in the request.

```json
// Authorize the tool call:
{ "type": "approve", "id": "req-1712836800000000000", "approved": true }

// Deny the tool call:
{ "type": "approve", "id": "req-1712836800000000000", "approved": false }
```

| Field      | Required | Description |
|------------|:--------:|-------------|
| `id`       | yes      | ID from the matching `approval_request` |
| `approved` | yes      | `true` = execute the tool; `false` = block it and notify the agent |

**On approval:** the tool runs and a `tool_start` event is broadcast to all clients, followed by the workflow resuming.  
**On denial:** the agent receives an error message informing it the action was blocked and it must adjust its plan.  
**Invalid ID:** the server replies with an `error` event (`"Invalid or already processed approval ID."`).

---

## Typical event sequence

Below is the expected order of events for a supervised 2-agent workflow:

```
CLIENT connects to ws://localhost:8080/ws

# Swarm assembly — one agent_created per specialist
SERVER  →  { type: "agent_created",    agent_data: { id: 1, name: "Researcher", status: "idle", ... } }
SERVER  →  { type: "agent_created",    agent_data: { id: 2, name: "Writer",     status: "idle", ... } }
SERVER  →  { type: "status",           message: "🚀 Session abc-123 started with 2 agent(s)..." }
SERVER  →  { type: "log",              message: "🧠 The Queen is consulting the manuals..." }
SERVER  →  { type: "log",              message: "👑 Swarm planned with 2 bees: Researcher -> Writer" }

# Workflow execution
SERVER  →  { type: "agent_change",     agent: "Researcher" }
SERVER  →  { type: "log",              message: "👑 [Queen] Passing the baton to: Researcher" }
SERVER  →  { type: "approval_request", id: "req-...", tool: "web_search", args: "{...}" }

CLIENT  →  { type: "approve",          id: "req-...", approved: true }

SERVER  →  { type: "status",           message: "👨‍🌾 Action authorized by the Beekeeper. Resuming workflow..." }
SERVER  →  { type: "tool_start",       agent: "Researcher", tool: "web_search", args: "{...}" }
SERVER  →  { type: "log",              message: "✅ [Researcher] Task complete." }
SERVER  →  { type: "agent_change",     agent: "Writer" }
SERVER  →  { type: "log",              message: "👑 [Queen] Passing the baton to: Writer" }
SERVER  →  { type: "tool_start",       agent: "Writer", tool: "write_file", args: "{...}" }
SERVER  →  { type: "log",              message: "✅ [Writer] Task complete." }
SERVER  →  { type: "log",              message: "👑 [Queen] Workflow complete! The swarm has finished." }
SERVER  →  { type: "result",           message: "Original Goal: ...\n\n--- Report from Researcher ---\n..." }
```

---

## Reconnection

The server does not buffer events. If the client disconnects mid-workflow, events emitted during the disconnection are lost. Implement exponential-backoff reconnection on the client side.

```js
function connect() {
  const ws = new WebSocket("ws://localhost:8080/ws");

  ws.addEventListener("close", () => {
    setTimeout(connect, Math.min(1000 * 2 ** retries++, 30000));
  });

  ws.addEventListener("open", () => { retries = 0; });
}

let retries = 0;
connect();
```

---

## Handling all event types

```js
const handlers = {
  status:           (msg) => console.info("[STATUS]", msg.message),
  log:              (msg) => console.debug("[LOG]", msg.message),
  agent_created:    (msg) => console.info("[AGENT CREATED]", msg.agent_data),
  agent_change:     (msg) => console.info("[AGENT]", msg.agent, "is now active"),
  tool_start:       (msg) => console.info("[TOOL]", msg.agent, "→", msg.tool, msg.args),
  approval_request: (msg) => promptUser(msg),   // show approval UI
  result:           (msg) => displayResult(msg.message),
  error:            (msg) => displayError(msg.message),
};

ws.addEventListener("message", (event) => {
  const msg = JSON.parse(event.data);
  handlers[msg.type]?.(msg);
});

function promptUser(msg) {
  const approved = confirm(`Allow agent to run "${msg.tool}"?\n\n${msg.args}`);
  ws.send(JSON.stringify({ type: "approve", id: msg.id, approved }));
}
```
