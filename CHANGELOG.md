# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Fixed

- **`jandaira-static` reverse proxy** (`cmd/static/main.go`): static server only served files; API calls from the frontend (`/api/*`, `/ws`) returned 404 on Linux/macOS native installs. Added `httputil.ReverseProxy` that forwards `/api/*` and `/ws` to `http://localhost:8080`, with SPA fallback to `index.html` for all other routes.
- **`jandaira.sh` syntax error** (`.github/workflows/build.yaml`): `&;` is invalid bash — the semicolon after `&` caused a parse error so both `jandaira-api` and `jandaira-static` silently failed to start on `./jandaira.sh start`. Split each backgrounded command and PID capture onto separate lines.

### Changed

- **Windows install command updated** (`README.md`, `docs/README.en.md`, `docs/README.es.md`, `docs/README.ru.md`, `docs/README.zh.md`): PowerShell snippet changed from `.\install-windows.ps1` to `powershell.exe -ExecutionPolicy Bypass -File .\install-windows.ps1` so users without a pre-configured execution policy can run the installer without extra steps.

### Added

- **OpenRouter provider** (`internal/brain/open_router.go`, `internal/api/setup_handler.go`): new `OpenRouterBrain` implements `Brain` and `StructuredBrain`, routing requests to any model available on openrouter.ai via their OpenAI-compatible API. `Chat` supports tool calling; `ChatJSON` uses `response_format: json_schema` for structured outputs; `Embed` returns an informative error (same policy as Anthropic). Default model `openai/gpt-4o-mini`. `POST /api/setup` with `"provider": "openrouter"` stores the API key as `OPENROUTER_API_KEY` in the vault and wires the brain to the Queen. 90 s HTTP timeout (vs 60 s for OpenAI) to absorb upstream routing latency. Transient-network retry reuses `httpDoWithRetry` from `open_ai.go`.
- **Groq provider** (`internal/brain/groq.go`, `internal/api/setup_handler.go`): new `GroqBrain` implements `Brain` and `StructuredBrain` against the Groq LPU inference API (`https://api.groq.com/openai/v1/chat/completions`). OpenAI-compatible schema — `Chat` supports tool calling with `tool_choice: auto`; `ChatJSON` uses `response_format: json_schema`; `Embed` returns informative error (no embedding endpoint). Uses `max_completion_tokens` (Groq deprecated `max_tokens`). Default model `llama-3.3-70b-versatile`. `POST /api/setup` with `"provider": "groq"` stores `GROQ_API_KEY` in vault. 60 s HTTP timeout; transient-network retry reuses `httpDoWithRetry`.
- **Outbound Webhooks API**: Completed the implementation of the Outbound Webhook feature for Colmeias (`internal/api/outbound_webhook_handler.go`, `internal/api/api.go`, `internal/model/colmeia.go`). Added full CRUD operations (`/api/colmeias/:id/outbound-webhooks`) to configure webhooks that automatically fire HTTP requests to external endpoints when a hive mission completes successfully. Included documentation updates in `docs/webhook-engine.md`.

### Changed

- **Brain token limit reads config dynamically** (`internal/brain/open_ai.go`, `internal/brain/anthropic.go`, `internal/brain/open_router.go`, `internal/brain/groq.go`, `internal/api/api.go`, `internal/api/setup_handler.go`, `internal/api/config_handler.go`): replaced static `MaxTokens int` field on all four brain structs with `MaxTokensFn func() int`. The closure is injected at brain creation via `Server.maxTokensFn()` and reads `MaxNectar` from the config service on every request — changes made via `PUT /api/config` are reflected immediately without a brain rebuild. Previously `MaxNectar` was copied once at setup time and became stale.
- **OpenRouter and Groq — HTTP 402 token-limit fallback** (`internal/brain/provider_http.go`): new shared `doPostWithFallback` helper retries the request without `max_tokens`/`max_completion_tokens` on HTTP 402, allowing providers with limited credit balances to service requests even when the configured token ceiling exceeds available credits. Both `OpenRouterBrain` and `GroqBrain` use this helper for `Chat` and `ChatJSON`.
- **`config_handler.go` — openrouter/groq provider-change support**: `PUT /api/config` with a provider change but no new API key now handles `openrouter` and `groq` cases (reads key from vault/env and rebuilds brain), matching the existing `anthropic`/`openai` paths.

### Fixed

- **`OpenAIBrain` — HTTP/2 GOAWAY transient retry** (`internal/brain/open_ai.go`): `Chat`, `ChatJSON`, and `Embed` made a single HTTP request with no retry logic; an OpenAI load-balancer connection rotation (HTTP/2 GOAWAY, `ErrCode=NO_ERROR`) propagated as a hard error and failed the entire job. Added `httpDoWithRetry` — up to 3 attempts with 500 ms → 1 s → 2 s exponential backoff for transient network errors (GOAWAY, connection reset, EOF, broken pipe). HTTP 4xx/5xx responses still fail immediately. Also fixed `executeWithRetry` in `internal/queue/group_queue.go`: the retry log omitted the actual error, and the final-failure log reported attempt count `i` instead of `i+1`.

### Security

- **Prompt injection defense and scope enforcement for specialists** (`internal/swarm/queen.go`): every specialist's system prompt is now wrapped with a non-overridable scope enforcement block via `buildScopedSystemPrompt`. The block instructs the agent to treat any in-context instruction that attempts to change its role (e.g. "ignore previous instructions", "you are now a…", "forget your role") as plain data, not as a command. If the task falls outside the specialist's defined scope the agent must respond with `SCOPE_VIOLATION: <reason>`; `isScopeViolation` detects that prefix and the `runSpecialist` loop returns an error, aborting the workflow immediately. Prevents agents from answering unrelated requests — e.g. an `Auditor` specialist configured for financial entries can no longer be hijacked into returning a cake recipe.

### Changed

- **Vector memory migrated to native VectorEngine** (`internal/brain/hnsw.go`, `internal/brain/vector_engine.go`): `QdrantHoneycomb` (gRPC client requiring an external Qdrant Docker container) replaced by an embedded, single-process `VectorEngine`. Storage: BadgerDB binary key-value store at `~/.config/jandaira/vectordb/`. Index: per-collection in-memory HNSW (Hierarchical Navigable Small World) approximate nearest-neighbour graph, rebuilt on startup from persisted vectors. Cache: all live document vectors held in RAM for O(1) retrieval. Background goroutine compacts BadgerDB value log every 5 minutes. `LocalVectorDB` (JSON-based fallback) removed from `memory.go`. `qdrant/go-client` and gRPC removed from `go.mod`; `github.com/dgraph-io/badger/v4` added. `QDRANT_HOST` environment variable no longer needed — zero external process dependencies.

### Fixed

- **Colmeia Qdrant collection created eagerly** (`internal/api/colmeia_handler.go`): `handleCreateColmeia` now calls `Honeycomb.EnsureCollection` immediately after the colmeia is persisted, using the real embedding dimension (via a probe embed) or 1536 as fallback. Previously the collection only existed after the first document upload, causing `store_memory` calls on a fresh colmeia to fail or land in the wrong collection.

- **`search_memory` no longer causes agent reflection-limit loop** (`internal/brain/qdrant.go`, `internal/tool/search_memory.go`): querying a Qdrant collection that doesn't yet exist returned a gRPC error which propagated back to the agent as a hard tool error; the agent retried on every iteration and exhausted the 5-step reflection limit. Fixed on two layers: `QdrantHoneycomb.Search` now detects `NOT_FOUND` gRPC status (and "doesn't exist" message) and returns empty results instead of an error; `SearchMemoryTool.Execute` converts any remaining search error into an informative string response (`nil` error) so the agent continues without retrying.

- **`store_memory` auto-creates Qdrant collection on missing collection** (`internal/brain/qdrant.go`): `QdrantHoneycomb.Store` now detects `NOT_FOUND` on upsert, calls `EnsureCollection` with the vector's actual dimension, and retries the upsert — eliminating hard errors for agents calling `store_memory` before any document was uploaded to a colmeia.

- **Existing colmeias get Qdrant collection on first dispatch** (`internal/api/colmeia_handler.go`): `handleColmeiaDispatch` now calls `EnsureCollection` for the colmeia's `groupID` before enriching the goal with semantic memory. Covers colmeias created before the eager-create fix was added to `handleCreateColmeia`.

- **Reflection limit raised 5 → 10 and summary preserves tool history** (`internal/swarm/queen.go`): limit of 5 iterations was too low for workflows that search memory, calculate, store, and verify (easily 5+ LLM turns). Raised to 10. The forced final-summary call now appends the stop instruction to the full message history instead of trimming to system + first user; the agent can now reference all memory search results and calculation outputs already retrieved when composing the final answer.

- **Queen planning prompt: tool-efficiency rule** (`internal/swarm/queen.go`): added output rule 5 instructing the Queen not to assign `execute_code` to specialists whose only computation is arithmetic or string formatting, and to prefer direct LLM computation over Wasm compilation for simple math — eliminating the 2-3 wasted iterations financial/reporting specialists spent compiling trivial Go code.

- **`search_memory` result limit raised and made configurable** (`internal/tool/search_memory.go`, `internal/api/colmeia_handler.go`, `internal/swarm/queen.go`): hardcoded `limit: 3` forced agents to call `search_memory` 4-5 times to retrieve a full transaction history, consuming most of the 10-iteration budget. Default raised to 10 and a `limit` parameter exposed so agents can request up to 50 results in a single call. Dispatch pre-seed search raised from 3 → 10 in `handleColmeiaDispatch` and from 5 → 20 in `DispatchWorkflow` so more historical context is injected upfront, reducing the need for in-workflow memory searches.

- **`store_memory` respects colmeia collection** (`internal/tool/search_memory.go`): added optional `collection` parameter to `StoreMemoryTool` (mirrors the existing parameter on `SearchMemoryTool`). Agents now pass the value from `[HIVE MEMORY COLLECTION: ...]` injected in the dispatch context so that stored records land in the correct per-colmeia collection instead of the global swarm collection.

### Added

- **Document tracking model** (`internal/model/document.go`, `internal/repository/document.go`, `internal/service/document.go`): new `Document` entity persists metadata for every uploaded file (filename, workspace path, Qdrant collection, scope key/value, chunk count). Enables listing and deleting documents without a separate Qdrant query.
- **`GET /api/sessions/:id/documents`** (`internal/api/document_handler.go`): list all documents uploaded to a session, ordered by upload date.
- **`DELETE /api/sessions/:id/documents/:docId`** (`internal/api/document_handler.go`): delete a document — removes the SQLite record, all Qdrant chunks matching `filename` + `session_id`, and the workspace file from disk.
- **`GET /api/colmeias/:id/documents`** (`internal/api/document_handler.go`): list all documents uploaded to a hive.
- **`DELETE /api/colmeias/:id/documents/:docId`** (`internal/api/document_handler.go`): delete a hive document — same cascade as the session variant (SQLite + Qdrant + disk).
- **`Honeycomb.DeleteByFilter`** (`internal/brain/memory.go`, `internal/brain/qdrant.go`): new method on the `Honeycomb` interface that deletes all Qdrant points whose payload matches every key/value pair in a filter map. Implemented on both `QdrantHoneycomb` (gRPC filter delete) and `LocalVectorDB` (in-memory scan).

### Fixed

- **Colmeia document search returning empty results** (`internal/api/colmeia_handler.go`, `internal/tool/search_memory.go`): documents uploaded to a hive were indexed into Qdrant collection `colmeia-{id}` but all search paths used the bare `colmeiaID` as collection name — pre-seed in `handleColmeiaDispatch`, pre-seed in `DispatchWorkflow`, and `SearchMemoryTool` all missed the indexed chunks. Fixed: `groupID` in `handleColmeiaDispatch` now uses `"colmeia-" + sanitizeID(colmeiaID)` (matching the storage collection name), and the pre-seed search uses `groupID`. `SearchMemoryTool` gains an optional `collection` parameter so agents can target a specific collection; the hive collection name is injected into `enrichedGoal` as `[HIVE MEMORY COLLECTION: ...]` so agents can use it.

- **UTF-8 panic on non-UTF-8 files** (`internal/api/document_handler.go`): uploading Latin-1 encoded files (CSV, PDF) caused `qdrant/go-client` to panic with `invalid UTF-8 in string`. All metadata values are now sanitized via `toValidUTF8()` (wraps `strings.ToValidUTF8`) before being passed to Qdrant.

### Changed

- **API messages translated to English** (`internal/api/`): all user-facing error and success messages across every handler (`session_handler.go`, `colmeia_handler.go`, `setup_handler.go`, `config_handler.go`, `skill_handler.go`, `document_handler.go`, `api.go`) translated from Portuguese to English for consistency.
- **`openapi.yaml`** (`docs/openapi.yaml`): added `GET`/`DELETE` document endpoints for sessions and hives, `Document` schema, and reusable `DocumentID` path parameter.

### Added

- **`GET /api/colmeias/:id/agentes/:agentId`** (`internal/api/colmeia_handler.go`): new endpoint to retrieve a single pre-defined agent by ID, including its associated skills. Previously only list (`GET /agentes`) and mutation (`PUT`, `DELETE`) endpoints existed for hive agents.

### Changed

- **`POST /api/colmeias/:id/agentes` — queen_managed guard** (`internal/api/colmeia_handler.go`): adding a pre-defined agent to a `queen_managed=true` hive now returns `409 Conflict`. Queen-managed hives assemble agents dynamically on every dispatch; manually pre-defining agents in that mode had no effect and was a source of confusion. Set `queen_managed=false` to use custom agents.

### Changed

- **`StoreMemoryTool` — persistent storage without file system** (`internal/tool/search_memory.go`): removed `write_file` and `create_directory` from the queen's toolkit. `store_memory` is now the sole persistence mechanism. Tool description updated to make this explicit. Added `type` and `metadata` parameters so agents can tag records (e.g. `financial_entry`, `calculation_result`) with arbitrary key-value fields.
- **`OpenAIBrain` — `max_completion_tokens`** (`internal/brain/open_ai.go`): replaced `max_tokens` with `max_completion_tokens` in both `Chat` and `ChatJSON` methods. Required by newer OpenAI reasoning models (o1, o3, o4-mini) that reject the legacy parameter.
- **`search_memory` / `store_memory` — English-only strings** (`internal/tool/search_memory.go`): all user-facing strings (descriptions, error messages, output text) translated to English.

### Fixed

- **`StoreMemoryTool` — no data loss on embedding failure** (`internal/tool/search_memory.go`): when `Brain.Embed` fails (e.g. Anthropic provider without an OpenAI key for embeddings), the tool now falls back to a uniform 1536-dim vector and sets `metadata["embedding"]="none"`, persisting the record to Qdrant instead of returning an error. Financial transactions and calculation results are never silently dropped.
- **`SearchMemoryTool` — graceful degradation on embedding failure** (`internal/tool/search_memory.go`): returns an informative message instead of an error when embedding is unavailable, preventing agent retry loops.

### Changed

- **Migração de memória vetorial: ChromaDB → Qdrant** (`internal/brain/qdrant.go`): `ChromaHoneycomb` substituído por `QdrantHoneycomb`, que se conecta ao Qdrant via gRPC (porta 6334) usando `github.com/qdrant/go-client`. IDs string mapeados para UUID via SHA1. Distância coseno nativa — score já retornado como similaridade em [0,1], sem conversão. Variável de ambiente `CHROMA_URL` substituída por `QDRANT_HOST` (somente hostname; porta 6334 fixada). `docker-compose-dev.yml` atualizado: serviço `chroma` removido, `QDRANT_HOST=qdrant` configurado no serviço `api`.

### Fixed

- **Contexto desatualizado ao retornar à colmeia** (`internal/swarm/queen.go`): cada novo `DispatchWorkflow` iniciava o `contextAccumulator` vazio, dependendo exclusivamente de especialistas que chamassem `search_memory` explicitamente. Corrigido: antes de iniciar o pipeline, o `DispatchWorkflow` agora busca as top-5 memórias relevantes no Honeycomb (via embedding do objetivo) e as injeta automaticamente no `contextAccumulator`. Resultado: contexto histórico disponível para todos os especialistas sem chamada explícita à ferramenta.

### Added

- **Gerenciamento de Skills** (`internal/model/skill.go`, `internal/repository/skill.go`, `internal/service/skill.go`, `internal/api/skill_handler.go`): skills são capacidades reutilizáveis que encapsulam instruções e ferramentas para um domínio específico. Podem ser associadas a colmeias ou a agentes individuais via tabelas many-to-many (`colmeia_skills`, `agente_colmeia_skills`).
  - **`Skill`**: entidade global com `name`, `description`, `instructions` (injetadas no system prompt) e `allowed_tools` (JSON).
  - **Queen-managed**: skills da colmeia são injetadas como bloco `SKILLS DISPONÍVEIS` no prompt de meta-planejamento da Rainha antes de cada `AssembleSwarm`. A Rainha decide quais especialistas recebem cada skill.
  - **Manual** (`queen_managed=false`): skills dos agentes são mescladas em `BuildSpecialists` — `instructions` adicionadas ao `SystemPrompt` e `allowed_tools` unidos sem duplicatas.
  - **Memória de longo prazo confirmada**: `LocalVectorDB.Search` já filtrava `score > 0.7`; `handleColmeiaDispatch` injeta histórico DB (últimos 3) + resultados semânticos Honeycomb antes de cada despacho.
  - **Novas rotas REST**: `GET/POST /api/skills`, `GET/PUT/DELETE /api/skills/:id`, `GET/POST /api/colmeias/:id/skills`, `DELETE /api/colmeias/:id/skills/:skillId`, `GET/POST /api/colmeias/:id/agentes/:agentId/skills`, `DELETE /api/colmeias/:id/agentes/:agentId/skills/:skillId`.
  - **Docs atualizados**: `openapi.yaml`, `api-integration.md`, `app-flow.md`, `README.md`.

- **Colmeias Persistentes** (`internal/model/colmeia.go`, `internal/repository/colmeia.go`, `internal/service/colmeia.go`, `internal/api/colmeia_handler.go`): colmeias nomeadas e persistentes com agentes pré-definidos pelo usuário ou gerenciados pela rainha. Cada colmeia mantém histórico de despachos (`HistoricoDespacho`) que é injetado como contexto nas conversas seguintes, permitindo continuidade entre múltiplas interações com a mesma colmeia.
  - **`Colmeia`**: entidade persistente com `name`, `description` e flag `queen_managed`.
  - **`AgenteColmeia`**: agente persistente com `name`, `system_prompt` e `allowed_tools` (JSON) — totalmente editáveis pelo usuário via API.
  - **`HistoricoDespacho`**: registro de cada despacho com `goal`, `result` e `status`. As 3 últimas conversas concluídas são injetadas como contexto no próximo despacho.
  - **Modos de despacho**: `queen_managed=true` → rainha monta o enxame automaticamente; `queen_managed=false` → usa os agentes pré-definidos pelo usuário.
  - **Novas rotas REST**: `GET/POST /api/colmeias`, `GET/PUT/DELETE /api/colmeias/:id`, `POST /api/colmeias/:id/dispatch`, `GET /api/colmeias/:id/historico`, `GET/POST /api/colmeias/:id/agentes`, `PUT/DELETE /api/colmeias/:id/agentes/:agentId`.

- **Knowledge Graph** (`internal/brain/graph.go`): nova interface `KnowledgeGraph` com implementação `LocalKnowledgeGraph` (JSON persistido em disco). A Queen registra agentes e tópicos como nós após cada workflow e usa o grafo para enriquecer o planejamento de futuros enxames com histórico de especialistas (`expert_in` edges). ([`swarm/queen.go`](internal/swarm/queen.go))
- **Memória de Curto Prazo com TTL** (`internal/brain/short_term.go`): `ShortTermMemory` — buffer de mensagens com expiração por entrada. Quando o limite é atingido ou `Flush()` é chamado, as mensagens são sumarizadas pelo LLM e arquivadas no ChromaDB como `short_term_archive`, evitando overflow de contexto em sessões longas.

### Fixed

- **Runtime image** (`Dockerfile`): trocado de `alpine` para `golang:1.26-bookworm` como imagem de runtime. Alpine usa musl libc — incompatível com LanceDB/ONNX que exige glibc (`libstdc++.so.6`). Builder também migrado para `golang:1.26-bookworm` para garantir que o binário seja linkado contra glibc. Home dir de `appuser` criado com `-m` e ownership de `/app` corrigido.
- **`execute_code` tool** (`internal/tool/wasm.go`): refatorado para aceitar código Go diretamente no campo `code` (string), eliminando dependência do `write_file` para criar o arquivo `.go` antes da execução. Código é gravado em dir temporário com `go.mod` mínimo, compilado para WASM (`GOOS=wasip1`), executado via wazero com `/app` montado no sandbox. `GOCACHE` explícito para evitar falha de permissão no container.
- **`read_file` tool** (`internal/tool/list_directory.go`): arquivo inexistente retornava `error` — specialist entrava em loop de retry até reflection limit. Corrigido para retornar mensagem informativa `"arquivo não existe (primeira execução — trate como lista vazia)"` com `nil` error.
- **`write_file` tool** (`internal/tool/list_directory.go`): `os.WriteFile` falhava silenciosamente quando diretório pai não existia. Adicionado `os.MkdirAll` antes de escrever.
- **Reflection limit** (`internal/swarm/queen.go`): ao atingir o limite de iterações, specialist retornava `error` matando o job. Corrigido para forçar uma chamada final ao LLM com contexto trimado (só system + primeiro user message) pedindo resumo — job sempre conclui com resultado em vez de falhar. Limite reduzido de 10 para 5 iterações.
- **JSON inválido da Queen** (`internal/swarm/queen.go`): LLM gerava JSON com escapes inválidos estilo LaTeX (`\(`, `\$`) causando `json.Unmarshal` falhar com `invalid character '(' in string escape code`. Adicionado `sanitizeJSONEscapes` que substitui qualquer `\X` inválido pelo caractere literal antes do parse.
- **Queen Structured Outputs** (`internal/brain/llm.go`, `internal/brain/open_ai.go`, `internal/swarm/queen.go`): `AssembleSwarm` now uses OpenAI Structured Outputs (`response_format: json_schema`) to guarantee the swarm plan always returns valid JSON matching the `SwarmPlan` schema. New optional `StructuredBrain` interface extends `Brain` with a `ChatJSON` method — `OpenAIBrain` implements it; other providers fall back to the previous `sanitizeJSONEscapes` path. Eliminates the `invalid character '\n' in string literal` error class in `AssembleSwarm`.

---

## [v1.2.0] — 2026-04-14

### Added

- **Vectorização de documentos** (`internal/brain/document.go`, `internal/api/document_handler.go`): pipeline completo de ingestão — extração de texto (PDF, DOCX, TXT, CSV, XLSX), chunking com sobreposição e embedding vetorial armazenado no ChromaDB. (`a3fb933`)
- **Ferramenta de busca web** (`internal/tool/duckduckgo.go`): tool `web_search` integrada ao DuckDuckGo, disponível para especialistas via `AllowedTools`. (`a4c404b`)

### Fixed

- **Document loader**: correção no handler de upload de documentos. (`6eb5be2`, `internal/api/document_handler.go`)
- **WebSearch — conteúdo HTML**: adicionada extração de conteúdo textual das páginas retornadas pela busca, evitando HTML cru no contexto do agente. (`6781223`)
- **Session handler**: ajustes na lógica de despacho de sessão, status de agentes e propagação de erros. (`c50d54e`)
- **CI/CD**: removidas tags desnecessárias no workflow de build. (`f47ad3c`, `.github/workflows/build.yaml`)

---

## [v1.1.2] — anterior a 2026-04-13

### Changed

- **Memória vetorial migrada para ChromaDB** (`internal/brain/chroma.go`): substituição do `LocalVectorDB` por `ChromaHoneycomb` como backend padrão de memória de longo prazo. (`6e72ac9`)

---

## [v1.1.1]

_(sem entradas neste intervalo)_

---

## [v1.1.0]

_(sem entradas neste intervalo)_

---

## [v1.0.2]

_(sem entradas neste intervalo)_

---

[Unreleased]: https://github.com/damiaoterto/jandaira/compare/v1.2.0...HEAD
[v1.2.0]: https://github.com/damiaoterto/jandaira/compare/v1.1.2...v1.2.0
[v1.1.2]: https://github.com/damiaoterto/jandaira/compare/v1.1.1...v1.1.2
[v1.1.1]: https://github.com/damiaoterto/jandaira/compare/v1.1.0...v1.1.1
[v1.1.0]: https://github.com/damiaoterto/jandaira/compare/v1.0.2...v1.1.0
[v1.0.2]: https://github.com/damiaoterto/jandaira/releases/tag/v1.0.2
