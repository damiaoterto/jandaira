# Webhook Engine — Implementation Reference

## Overview

The Webhook Engine allows external systems to trigger Jandaira hive workflows via HTTP
without additional authentication beyond an optional HMAC-SHA256 secret. A webhook
maps a unique public URL slug to a target `Colmeia`, and uses a `GoalTemplate` to
transform the incoming JSON payload into the goal the Queen will process.

```
External System  →  POST /api/webhooks/{slug}  →  Webhook Engine  →  Queen.DispatchWorkflow
```

---

## Architecture

### Layers

| Layer | File | Responsibility |
|---|---|---|
| Model | `internal/model/webhook.go` | GORM entity, `webhooks` table |
| Repository | `internal/repository/webhook.go` | DB access — CRUD + `FindBySlug` |
| Service | `internal/service/webhook.go` | Business logic + `ProcessPayload` |
| Handler | `internal/api/webhook_handler.go` | HTTP trigger + CRUD endpoints |
| Router | `internal/api/api.go` | Route registration |
| Migration | `internal/database/database.go` | `AutoMigrate(&model.Webhook{})` |
| Bootstrap | `cmd/api/main.go` | DI wiring |

### Data model

```go
type Webhook struct {
    BaseModel                    // ID uint, CreatedAt, UpdatedAt
    Name         string          // Human-readable label
    Slug         string          // Unique URL segment — UNIQUE INDEX
    ColmeiaID    string          // FK → colmeias.id (UUID string)
    Secret       string          // Optional HMAC-SHA256 signing secret
    Active       bool            // false → 410 Gone without processing
    GoalTemplate string          // Go text/template rendered against payload
}
```

### Route registration

```
// Public — exempt from setupMiddleware
r.POST("/api/webhooks/:slug", s.handleWebhookTrigger)

// Protected — behind setupMiddleware
api.Group("/webhooks")
    GET    ""     → handleListWebhooks
    POST   ""     → handleCreateWebhook
    GET    "/:id" → handleGetWebhook
    PUT    "/:id" → handleUpdateWebhook
    DELETE "/:id" → handleDeleteWebhook
```

The public trigger route is registered directly on `r` (the root Gin engine) before
the `api` group with `setupMiddleware`, so external callers always receive a meaningful
response even if the hive is not yet configured. The handler itself returns an error if
the linked colmeia or queen is unavailable.

---

## GoalTemplate

`ProcessPayload` uses Go's standard `text/template` package. The incoming JSON body is
unmarshalled into `map[string]interface{}` and passed as the template dot (`.`).

```go
func (s *webhookService) ProcessPayload(
    webhook *model.Webhook,
    payload map[string]interface{},
) (string, error) {
    tmpl, err := template.New("goal").Parse(webhook.GoalTemplate)
    // ...
    tmpl.Execute(&buf, payload)
}
```

### Syntax reference

| Template | Payload key | Result |
|---|---|---|
| `{{.project_name}}` | `{"project_name":"Jandaira"}` | `Jandaira` |
| `{{.repository.name}}` | `{"repository":{"name":"jandaira"}}` | `jandaira` |
| `{{.severity}}` | `{"severity":"critical"}` | `critical` |
| `{{.number}}` | `{"number":42}` | `42` |

> **Note:** Nested field access (`{{.repository.name}}`) works because Go's
> `text/template` traverses `map[string]interface{}` via the dot operator. The JSON
> must be a nested object, not a string.

### Error handling

If the template fails to parse or execute (e.g. syntax error), the handler returns
`422 Unprocessable Entity` with the template error message. The dispatch is **not**
attempted and no `HistoricoDespacho` record is created.

---

## HMAC-SHA256 Signature Validation

When a webhook has a non-empty `Secret`, every incoming trigger must include a valid
signature. The implementation follows the GitHub Webhooks standard.

### Header format

```
X-Hub-Signature-256: sha256=<hex-encoded-HMAC-SHA256-of-raw-body>
```

The `X-Webhook-Signature` header is accepted as a fallback alias.

### Validation logic

```go
func validateWebhookHMAC(secret, body []byte, signatureHeader string) bool {
    const prefix = "sha256="
    if !strings.HasPrefix(signatureHeader, prefix) {
        return false
    }
    got, err := hex.DecodeString(signatureHeader[len(prefix):])
    if err != nil {
        return false
    }
    mac := hmac.New(sha256.New, secret)
    mac.Write(body)
    return hmac.Equal(got, mac.Sum(nil))   // constant-time comparison
}
```

`hmac.Equal` is used for the final comparison (constant-time) to prevent timing attacks.

### Generating a signature (caller side)

```bash
BODY='{"project_name":"Jandaira","env":"prod"}'
SECRET="my-hmac-secret"
SIG="sha256=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')"

curl -X POST http://localhost:8080/api/webhooks/monitor-deploy \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: $SIG" \
  -d "$BODY"
```

**Python equivalent:**
```python
import hmac, hashlib, json, requests

secret  = b"my-hmac-secret"
payload = {"project_name": "Jandaira", "env": "prod"}
body    = json.dumps(payload, separators=(",", ":")).encode()
sig     = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()

requests.post(
    "http://localhost:8080/api/webhooks/monitor-deploy",
    data=body,
    headers={"Content-Type": "application/json", "X-Hub-Signature-256": sig},
)
```

---

## Dispatch Flow (handleWebhookTrigger)

```
POST /api/webhooks/:slug
         │
         ▼
1. webhookService.GetBySlug(slug)
         │  404 → "Webhook not found."
         │  webhook.Active == false → 410 Gone
         ▼
2. io.ReadAll(body)
         │
         ▼
3. validateWebhookHMAC (if webhook.Secret != "")
         │  invalid → 401 Unauthorized
         ▼
4. json.Unmarshal(body) → map[string]interface{}
         │  parse error → 400 Bad Request
         ▼
5. webhookService.ProcessPayload(webhook, payload)
         │  template error → 422 Unprocessable Entity
         ▼  goal = rendered string
6. colmeiaService.GetColmeia(webhook.ColmeiaID)
         │  404 → "Colmeia linked to this webhook was not found."
         ▼
7. colmeiaService.BuildGoalWithHistory(colmeia, goal)
8. colmeiaService.BuildSkillsContext(colmeia)   [prepend if non-empty]
9. colmeiaService.CreateHistorico(colmeia.ID, goal)
         │  500 → abort
         ▼
10. Queen.IsSwarmRegistered(groupID) → RegisterSwarm if needed
         groupID = "colmeia-" + sanitizeID(colmeia.ID)
         ▼
11. Broadcast WsMessage{type:"status", "🪝 Webhook '{name}' triggered..."}
         ▼
12a. colmeia.QueenManaged == true:
         Queen.AssembleSwarm(ctx 2min, enrichedGoal, maxWorkers)
         → go Queen.DispatchWorkflow(ctx 10min, groupID, goal, specialists)

12b. colmeia.QueenManaged == false:
         colmeiaService.BuildSpecialists(colmeia)
         → go Queen.DispatchWorkflow(ctx 10min, groupID, goal, predefinedSpecialists)
         ▼
13. 202 Accepted → WebhookTriggerResponse
         │
         goroutine:
           resultChan → CompleteHistorico + Broadcast result
           errChan    → FailHistorico    + Broadcast error
           ctx.Done() → FailHistorico    + Broadcast timeout
```

---

## Configuration Examples

### 1. GitHub Push → Code Review

```bash
# Create hive
curl -X POST http://localhost:8080/api/colmeias \
  -H "Content-Type: application/json" \
  -d '{"name":"Code Review Hive","queen_managed":true}'

# Create webhook
curl -X POST http://localhost:8080/api/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name":          "GitHub Push Review",
    "slug":          "github-push",
    "colmeia_id":    "<hive-id>",
    "secret":        "<github-webhook-secret>",
    "goal_template": "Review the latest push to {{.repository.full_name}} on branch {{.ref}}. Pusher: {{.pusher.name}}. Commits: {{len .commits}}",
    "active":        true
  }'
```

Configure in GitHub → Settings → Webhooks:
- **Payload URL:** `http://your-server:8080/api/webhooks/github-push`
- **Content type:** `application/json`
- **Secret:** `<github-webhook-secret>`

### 2. Prometheus AlertManager → Incident Response

```bash
curl -X POST http://localhost:8080/api/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name":          "Prometheus Alerts",
    "slug":          "prometheus-alert",
    "colmeia_id":    "<hive-id>",
    "goal_template": "Investigate alert \"{{.alertname}}\" on instance {{.instance}}. Severity: {{.severity}}. Labels: job={{.job}}",
    "active":        true
  }'
```

`alertmanager.yml` receiver:
```yaml
receivers:
  - name: jandaira
    webhook_configs:
      - url: http://your-server:8080/api/webhooks/prometheus-alert
        send_resolved: false
```

### 3. CI/CD Deploy Monitor (no secret)

```bash
curl -X POST http://localhost:8080/api/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name":          "Deploy Monitor",
    "slug":          "deploy-monitor",
    "colmeia_id":    "<hive-id>",
    "goal_template": "Analyse the deploy of {{.project}} v{{.version}} to {{.environment}}. Status: {{.status}}. Duration: {{.duration_seconds}}s",
    "active":        true
  }'
```

---

## Disabling a Webhook

Set `active: false` to stop processing without deleting the record. The public trigger
returns `410 Gone` immediately, leaving no `HistoricoDespacho` records.

```bash
curl -X PUT http://localhost:8080/api/webhooks/1 \
  -H "Content-Type: application/json" \
  -d '{
    "name":          "Deploy Monitor",
    "slug":          "deploy-monitor",
    "colmeia_id":    "<hive-id>",
    "goal_template": "...",
    "active":        false
  }'
```

---

## Memory Isolation

Each webhook trigger uses the same `groupID` as the target colmeia's regular dispatches:

```
groupID = "colmeia-" + sanitizeID(colmeia.ID)
```

This means:
- Webhook-triggered workflows share vector memory with manual dispatches to the same hive.
- `BuildGoalWithHistory` injects the last 10 completed dispatches as conversation context,
  regardless of whether they were triggered manually or via webhook.
- Each hive remains isolated from all other hives.

---

## Error Reference

| HTTP | Condition | Handler action |
|---|---|---|
| `404` | Slug not found | Return immediately, no side effects |
| `410` | `active: false` | Return immediately, no side effects |
| `401` | HMAC mismatch / missing header | Return immediately, no side effects |
| `400` | Body is not valid JSON | Return immediately, no side effects |
| `422` | Template parse/execute error | Return immediately, no `HistoricoDespacho` created |
| `404` | Linked colmeia deleted | Return after slug lookup, no `HistoricoDespacho` |
| `500` | `CreateHistorico` DB error | Return after enrichment, dispatch aborted |
| `400` | `queen_managed=false` + no agents | `FailHistorico` called, 400 returned |
| `500` | `AssembleSwarm` error (sync) | `FailHistorico` called, 500 returned |
| `202` | Dispatch started | Goroutine owns result/error/timeout handling |
