# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- **Colmeias Persistentes** (`internal/model/colmeia.go`, `internal/repository/colmeia.go`, `internal/service/colmeia.go`, `internal/api/colmeia_handler.go`): colmeias nomeadas e persistentes com agentes pré-definidos pelo usuário ou gerenciados pela rainha. Cada colmeia mantém histórico de despachos (`HistoricoDespacho`) que é injetado como contexto nas conversas seguintes, permitindo continuidade entre múltiplas interações com a mesma colmeia.
  - **`Colmeia`**: entidade persistente com `name`, `description` e flag `queen_managed`.
  - **`AgenteColmeia`**: agente persistente com `name`, `system_prompt` e `allowed_tools` (JSON) — totalmente editáveis pelo usuário via API.
  - **`HistoricoDespacho`**: registro de cada despacho com `goal`, `result` e `status`. As 3 últimas conversas concluídas são injetadas como contexto no próximo despacho.
  - **Modos de despacho**: `queen_managed=true` → rainha monta o enxame automaticamente; `queen_managed=false` → usa os agentes pré-definidos pelo usuário.
  - **Novas rotas REST**: `GET/POST /api/colmeias`, `GET/PUT/DELETE /api/colmeias/:id`, `POST /api/colmeias/:id/dispatch`, `GET /api/colmeias/:id/historico`, `GET/POST /api/colmeias/:id/agentes`, `PUT/DELETE /api/colmeias/:id/agentes/:agentId`.

- **Knowledge Graph** (`internal/brain/graph.go`): nova interface `KnowledgeGraph` com implementação `LocalKnowledgeGraph` (JSON persistido em disco). A Queen registra agentes e tópicos como nós após cada workflow e usa o grafo para enriquecer o planejamento de futuros enxames com histórico de especialistas (`expert_in` edges). ([`swarm/queen.go`](internal/swarm/queen.go))
- **Memória de Curto Prazo com TTL** (`internal/brain/short_term.go`): `ShortTermMemory` — buffer de mensagens com expiração por entrada. Quando o limite é atingido ou `Flush()` é chamado, as mensagens são sumarizadas pelo LLM e arquivadas no ChromaDB como `short_term_archive`, evitando overflow de contexto em sessões longas.

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
