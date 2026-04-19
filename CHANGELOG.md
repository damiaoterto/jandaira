# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

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
