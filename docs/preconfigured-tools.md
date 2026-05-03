# Preconfigured Tools — Implementation Guide

Preconfigured tools are tools that require an external API key to operate. Tokens are stored in the encrypted vault (`AES-256-GCM`) and injected at execution time — no restart required after configuring a key.

---

## Architecture

```
POST /api/tools/preconfigured/:tool
         │
         ▼
preconfigured_tools_handler.go   ← generic handler (registry-driven)
         │
         ▼
security.Vault.SaveSecret(tool + "_api_key", key)
         │
         ▼
~/.config/jandaira/.secrets/vault.enc   ← AES-256-GCM encrypted JSON
```

At execution time:

```
Queen dispatches task
         │
         ▼
tool.FirecrawlTool.Execute(ctx, argsJSON)
         │
         ▼
Vault.GetSecret("firecrawl_api_key")
         │
         ▼
firecrawl.NewClient(option.WithAPIKey(key))
         │
         ▼
Firecrawl API
```

---

## Files

| File | Role |
|------|------|
| `internal/api/preconfigured_tools_handler.go` | Generic HTTP handlers + tool registry |
| `internal/tool/firecrawl.go` | FirecrawlTool — implements the `Tool` interface |
| `internal/security/vault.go` | `SaveSecret`, `GetSecret`, `DeleteSecret` |
| `cmd/api/main.go` | Tool registration with injected vault |

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/tools/preconfigured` | List all tools and their configured status |
| `GET` | `/api/tools/preconfigured/:tool` | Status of a specific tool |
| `POST` | `/api/tools/preconfigured/:tool` | Save API key to vault |
| `DELETE` | `/api/tools/preconfigured/:tool` | Remove API key from vault |

### Configure a tool

```bash
curl -X POST http://localhost:8080/api/tools/preconfigured/firecrawl \
  -H "Content-Type: application/json" \
  -d '{"api_key": "fc-your-key-here"}'
```

```json
{ "message": "firecrawl API key configured successfully." }
```

### Check status

```bash
curl http://localhost:8080/api/tools/preconfigured/firecrawl
```

```json
{
  "tool": "firecrawl",
  "description": "Web scraping and crawling via Firecrawl API",
  "configured": true
}
```

### List all

```bash
curl http://localhost:8080/api/tools/preconfigured
```

```json
{
  "tools": [
    {
      "name": "firecrawl",
      "description": "Web scraping and crawling via Firecrawl API",
      "configured": true
    }
  ]
}
```

### Remove key

```bash
curl -X DELETE http://localhost:8080/api/tools/preconfigured/firecrawl
```

```json
{ "message": "firecrawl API key removed." }
```

---

## Firecrawl Tool

Registered as `"firecrawl"` in the Queen's tool map. Supports four actions via a single `action` parameter.

### Parameters

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | `string` | yes | `scrape`, `crawl`, `search`, or `map` |
| `url` | `string` | for scrape/crawl/map | Target URL |
| `query` | `string` | for search | Web search query |
| `limit` | `integer` | no | Max pages/results (default: 10) |

### Actions

**scrape** — extracts markdown from a single page:
```json
{ "action": "scrape", "url": "https://example.com" }
```

**crawl** — recursively crawls a site up to `limit` pages:
```json
{ "action": "crawl", "url": "https://docs.example.com", "limit": 20 }
```

**search** — web search returning result summaries:
```json
{ "action": "search", "query": "Go concurrency patterns 2024", "limit": 5 }
```

**map** — discovers all links on a site:
```json
{ "action": "map", "url": "https://example.com", "limit": 100 }
```

---

## Adding a New Preconfigured Tool

### 1. Register in the handler registry

`internal/api/preconfigured_tools_handler.go`:

```go
var preconfiguredTools = map[string]string{
    "firecrawl":  "Web scraping and crawling via Firecrawl API",
    "mytool":     "Description of what this tool does",   // ← add here
}
```

No other handler code changes needed. The vault key is derived automatically as `mytool_api_key`.

### 2. Implement the Tool interface

Create `internal/tool/mytool.go`:

```go
package tool

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/damiaoterto/jandaira/internal/security"
)

const myToolVaultKey = "mytool_api_key"

type MyTool struct {
    Vault *security.Vault
}

func (t *MyTool) Name() string { return "mytool" }

func (t *MyTool) Description() string {
    return "Description of what this tool does and when to use it."
}

func (t *MyTool) Parameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "input": map[string]interface{}{
                "type":        "string",
                "description": "Input for the tool",
            },
        },
        "required": []string{"input"},
    }
}

func (t *MyTool) Execute(ctx context.Context, argsJSON string) (string, error) {
    if t.Vault == nil {
        return "", fmt.Errorf("mytool not configured: vault unavailable")
    }

    apiKey, err := t.Vault.GetSecret(myToolVaultKey)
    if err != nil {
        return "", fmt.Errorf("mytool API key not set: configure via POST /api/tools/preconfigured/mytool")
    }

    var args struct {
        Input string `json:"input"`
    }
    if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
        return "", fmt.Errorf("invalid arguments: %w", err)
    }

    // use apiKey and args.Input to call the external service
    _ = apiKey
    return "result", nil
}
```

### 3. Equip the tool in main.go

`cmd/api/main.go`:

```go
queen.EquipTool(&tool.MyTool{Vault: vault})
```

That's all. The vault is already initialized at startup and shared across all tools.

---

## Vault Key Convention

Vault keys follow the pattern `{tool-name}_api_key`:

| Tool | Vault key |
|------|-----------|
| `firecrawl` | `firecrawl_api_key` |
| `mytool` | `mytool_api_key` |

The handler derives this automatically via `vaultKeyForTool(name)` — the tool struct must use the same constant to stay consistent.

---

## Security Notes

- Keys are encrypted at rest with AES-256-GCM using a per-installation master key.
- Master key lives at `~/.config/jandaira/.secrets/master.key` (mode `0600`).
- Encrypted store lives at `~/.config/jandaira/.secrets/vault.enc` (mode `0600`).
- API keys are never returned by any endpoint — `GET` returns only `configured: true/false`.
- Vault is initialized per-request in handlers (same pattern as the rest of the codebase), not stored on the Server struct.
