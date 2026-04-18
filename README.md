# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="jandaira.png" alt="Jandaira Logo"/>
</p>

Um framework de **multiagentes autônomos** escrito em Go, inspirado na inteligência coletiva da abelha nativa brasileira _Melipona subnitida_ — a **Jandaíra**.

---

> 🌐 [English](docs/README.en.md) · **Português** · [中文](docs/README.zh.md) · [Русский](docs/README.ru.md)

---

## 📖 Por que "Jandaira"?

A **Jandaíra** (_Melipona subnitida_) é uma abelha sem ferrão endêmica da Caatinga. Pequena, resiliente, e extraordinariamente cooperativa — ela não precisa de um líder centralizado para construir uma colmeia funcional. Cada operária conhece seu papel, executa sua tarefa com autonomia e devolve o resultado para o coletivo.

Esse é exatamente o modelo de arquitetura que o projeto implementa:

- A **Rainha (`Queen`)** não executa tarefas — ela orquestra, valida políticas e garante segurança.
- As **Especialistas (`Specialists`)** são agentes leves com ferramentas restritas, executando em silos isolados.
- O **Néctar** é a metáfora para o orçamento de tokens: cada agente consome néctar; quando acaba, a colmeia para.
- As **Skills** são capacidades reutilizáveis (instruções + ferramentas) que podem ser associadas a colmeias ou agentes. Na rainha, enriquecem o meta-planejamento; nos agentes manuais, são mescladas no prompt e ferramentas no momento do despacho.
- A **Colmeia (`Honeycomb`)** é o sistema de memória persistente em duas camadas: o `ShortTermMemory` mantém o contexto recente em RAM com expiração automática por TTL; o ChromaDB arquiva o conhecimento consolidado como vetores de longo prazo.
- O **Grafo de Conhecimento (`KnowledgeGraph`)** mapeia relações entre agentes, tópicos e ferramentas — a Rainha consulta esse grafo antes de cada missão para reutilizar perfis de especialistas que já obtiveram sucesso em objetivos semelhantes.
- O **Apicultor** é o humano no loop: pode aprovar ou bloquear qualquer ação da IA antes de ela ser executada.

---

## 🏗️ Arquitetura

### Visão Geral do Fluxo

```
┌─────────────────────────────────────────────────────────────────┐
│                   API REST + WebSocket (:8080)                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  👤 Cliente envia objetivo via POST /api/dispatch        │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ DispatchWorkflow()
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Queen (Orquestradora)                          │
│                                                                  │
│  ┌──────────────┐   ┌─────────────┐   ┌──────────────────────┐  │
│  │  GroupQueue  │   │   Policy    │   │   NectarUsage ($$)   │  │
│  │  (FIFO, N=3) │   │ (isolate,   │   │   Token budget       │  │
│  │              │   │  approval)  │   │   por enxame         │  │
│  └──────────────┘   └─────────────┘   └──────────────────────┘  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ Pipeline (Passagem de Bastão)
          ┌────────────────┴─────────────────┐
          ▼                                  ▼
┌──────────────────────┐          ┌──────────────────────┐
│  Especialista #1     │  ctx     │  Especialista #2     │
│  "Desenvolvedora"    │ ──────►  │  "Auditora"          │
│  Tools: write_file   │          │  Tools: execute_code │
│         search_mem   │          │         read_file    │
└──────────┬───────────┘          └──────────┬───────────┘
           │                                 │
           ▼                                 ▼
┌──────────────────────────────────────────────────────────┐
│                   🔐 Security Layer                       │
│   Payload criptografado (AES-GCM) entre cada passagem    │
│   de bastão — contexto nunca trafega em texto puro       │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│              👨‍🌾 Apicultor (Human-in-the-Loop)            │
│   RequiresApproval=true → WS envia approval_request      │
│   approved=true → autoriza │ approved=false → bloqueia   │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│                   🍯 Honeycomb (ChromaDB)                 │
│   Resultado do workflow é embeddado e indexado            │
│   Memória de longo prazo compartilhada entre missões     │
└──────────────────────────────────────────────────────────┘
```

### Mapa de Pacotes

```
jandaira/
├── cmd/
│   └── api/
│       └── main.go          # Entrypoint: servidor HTTP + WebSocket
│
└── internal/
    ├── brain/               # Sistema nervoso do enxame
    │   ├── open_ai.go       # Brain: Chat + Embed via OpenAI
    │   ├── memory.go        # Honeycomb: interface vetorial + LocalVectorDB
    │   ├── chroma.go        # ChromaHoneycomb: backend ChromaDB
    │   ├── graph.go         # KnowledgeGraph: grafo agente ↔ tópico (GraphRAG)
    │   ├── short_term.go    # ShortTermMemory: buffer TTL + compactação automática
    │   └── document.go      # Extração de texto + chunking (PDF, DOCX, XLSX…)
    │
    ├── queue/               # Escalonador FIFO com concorrência limitada
    │   └── group_queue.go   # GroupQueue: N workers por grupo
    │
    ├── security/            # Criptografia de payloads inter-agentes
    │   ├── crypto.go        # AES-GCM Seal/Open + geração de chave
    │   ├── vault.go         # Vault local para segredos
    │   └── sandbox.go       # Sandbox de execução
    │
    ├── swarm/               # Núcleo do sistema de agentes
    │   └── queen.go         # Orquestradora: políticas, HIL, pipeline
    │
    ├── tool/                # Ferramentas disponíveis aos agentes
    │   ├── list_directory.go
    │   ├── search_memory.go # search_memory + store_memory
    │   └── wasm.go          # Sandbox de execução via wazero
    │
    ├── api/                 # Handlers HTTP e WebSocket
    ├── config/              # Configuração da aplicação
    ├── database/            # Conexão SQLite
    ├── i18n/                # Internacionalização
    ├── model/               # Modelos de dados
    ├── prompt/              # Templates de prompt
    ├── repository/          # Acesso a dados
    └── service/             # Lógica de negócio
```

---

## 🧠 Arquitetura de Memória

O `internal/brain/` vai além de um banco vetorial: implementa uma hierarquia de memória em dois níveis com um grafo de conhecimento que cresce a cada missão.

### Memória de Curto Prazo — `ShortTermMemory`

`brain/short_term.go` é um buffer de mensagens com TTL por entrada. Ele resolve o problema de overflow de contexto em enxames de longa duração:

- Cada mensagem recebe um timestamp de expiração no momento da inserção
- Entradas expiradas são descartadas silenciosamente no próximo acesso
- **Compactação automática**: quando o buffer atinge `maxEntries`, o LLM sumariza o histórico acumulado em um parágrafo denso → o resumo é embeddado e arquivado no ChromaDB como `short_term_archive` → o buffer RAM é zerado
- `Flush(ctx)` deve ser chamado ao final de cada sessão para garantir arquivamento completo; em caso de falha do LLM, o transcript bruto é arquivado como fallback

```
 Nova mensagem inserida
         │
         ▼
┌──────────────────────────────────┐
│      ShortTermMemory (RAM)       │
│  [msg₁ · expiração: +30min]     │
│  [msg₂ · expiração: +30min]     │
│  ...                             │
│  [msgN · expiração: +30min]     │ ← overflow: compact() dispara
└──────────────────────────────────┘
         │
         ▼
   LLM sumariza o histórico
         │
         ▼
┌──────────────────────────────────┐
│  ChromaDB  (Memória de Longo Prazo)│
│  type: "short_term_archive"      │
│  content: "Em [sessão], o agente │
│  decidiu X, encontrou Y..."      │
└──────────────────────────────────┘
```

### Grafo de Conhecimento — `KnowledgeGraph` (GraphRAG)

`brain/graph.go` implementa um grafo de conhecimento persistido em JSON (`~/.config/jandaira/knowledge_graph.json`) que acumula expertise automaticamente a cada workflow concluído.

**Modelo de dados**

| Elemento | Tipo | Exemplo |
|---|---|---|
| Perfil de especialista | nó `agent` | `"Analista de Dados"` |
| Domínio da missão | nó `topic` | `"análise de relatório financeiro"` |
| Vínculo de expertise | aresta `expert_in` | `agent → topic` |

**Ciclo de aprendizado automático da Queen**

Após cada workflow, `registerWorkflowInGraph` executa em background:
1. Cria/atualiza um nó `topic` com o objetivo da missão (até 80 chars)
2. Para cada especialista do pipeline, cria/atualiza um nó `agent` com o preview do prompt
3. Cria arestas `expert_in` ligando cada agente ao tópico

Antes de montar o próximo enxame, `graphContextForGoal` faz:
1. Extrai palavras-chave do objetivo (> 4 chars)
2. Busca nós `topic` cujo label contenha cada palavra-chave
3. Retorna os nós `agent` conectados via `expert_in`
4. Injeta o bloco **"PAST SPECIALIST KNOWLEDGE"** no prompt de meta-planejamento

Resultado: a Rainha projeta enxames progressivamente melhores ao longo do tempo, sem chamadas LLM extras, apenas consultando o grafo acumulado.

```
 Novo objetivo: "Analisar dados de vendas trimestrais"
         │
         ▼
  graphContextForGoal() — extrai palavras-chave
         │
         ▼
┌────────────────────────────────────────────┐
│              KnowledgeGraph                │
│                                            │
│  "Analista de Vendas" ─expert_in─► "dados de vendas"
│  "Extrator de Relatórios" ─expert_in─► "análise trimestral"
│                                            │
└────────────────────────────────────────────┘
         │  perfis históricos encontrados
         ▼
  Prompt da Queen enriquecido:
  "PAST SPECIALIST KNOWLEDGE:
   - Analista de Vendas: especialista em...
   - Extrator de Relatórios: usa read_file e..."
         │
         ▼
  AssembleSwarm() com contexto histórico → delegação mais precisa
```

---

## ⚡ Diferenciais vs. NanoClaw

| Característica                | NanoClaw (Python)     | Jandaira (Go)                          |
| ----------------------------- | --------------------- | -------------------------------------- |
| **Linguagem**                 | Python                | Go 1.22+                               |
| **Concorrência**              | `asyncio` / threads   | Goroutines nativas + channels          |
| **Isolamento de agentes**     | Docker containers     | Wasm via `wazero` (sem Docker)         |
| **Comunicação IPC**           | JSON em disco / Redis | Memória compartilhada, tipada          |
| **Criptografia inter-agente** | ❌ Não existe         | ✅ AES-GCM entre cada bastão           |
| **Human-in-the-Loop**         | Opcional / externo    | ✅ Nativo: modo Apicultor via WebSocket |
| **Budget de tokens**          | Manual                | ✅ `NectarUsage` automático por enxame |
| **Memória vetorial**          | Pinecone / externo    | ✅ ChromaDB via Docker                 |
| **Grafo de conhecimento**     | ❌ Não existe         | ✅ `KnowledgeGraph` — GraphRAG nativo  |
| **Memória de curto prazo**    | ❌ Não existe         | ✅ `ShortTermMemory` com TTL e compactação LLM |
| **Interface**                 | Inexistente           | ✅ API REST + WebSocket                |
| **Latência de IPC**           | Alta (I/O disco/rede) | Mínima (memória)                       |

### Por que Go supera Python aqui?

1. **Goroutines são mais baratas que threads** — rodar 100 agentes simultâneos custa frações do que custaria em Python com `asyncio` ou `threading`.
2. **Binário estático** — zero dependências em runtime. Um `go build` gera um executável que roda em qualquer Linux sem instalar nada.
3. **Sem GIL** — Python tem o Global Interpreter Lock; Go paraleliza de verdade em múltiplos núcleos.
4. **`wazero` é 100% Go** — o runtime Wasm não exige CGo, Docker ou sistemas externos. O agente roda em sandbox dentro do mesmo processo.

---

## 🚀 Tutorial de Uso

### Pré-requisitos

```bash
# Go 1.22 ou superior
go version

# Docker (para o ChromaDB)
docker --version

# Chave OpenAI
export OPENAI_API_KEY="sk-..."
```

### Subindo o ChromaDB

```bash
# Via Docker diretamente
docker run -d --name chroma -p 8000:8000 chromadb/chroma:latest

# Ou usando o docker-compose do projeto
docker compose up -d
```

Por padrão o servidor conecta em `http://localhost:8000`. Para usar outro endereço:

```bash
export CHROMA_URL="http://meu-chroma:8000"
```

### Instalação

#### Opção 1 — Compilar a partir do código-fonte

```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira

# Baixar dependências
go mod tidy

# Compilar o servidor API
go build -o jandaira-api ./cmd/api/
```

#### Opção 2 — Executar diretamente

```bash
go run ./cmd/api/main.go --port 8080
```

### Executar a colmeia

```bash
./jandaira-api --port 8080
```

O servidor estará disponível em `http://localhost:8080`. Monitore os eventos em tempo real via WebSocket em `ws://localhost:8080/ws`.

### Exemplo: criar e testar um arquivo Go

1. Envie o objetivo via `POST /api/dispatch`:

   ```bash
   curl -X POST http://localhost:8080/api/dispatch \
     -H "Content-Type: application/json" \
     -d '{"goal": "Crie um arquivo Go chamado soma.go que some dois números", "group_id": "enxame-alfa"}'
   ```

2. A Rainha distribui a tarefa para a pipeline de Especialistas:
   - **Desenvolvedora Wasm** → escreve `soma.go` usando `write_file`
   - **Auditora de Qualidade** → executa o código com `execute_code` e gera um relatório

3. Acompanhe o progresso pelo WebSocket:

   ```json
   { "type": "agent_change", "agent": "Desenvolvedora Wasm" }
   { "type": "tool_start",   "agent": "Desenvolvedora Wasm", "tool": "write_file", "args": "{...}" }
   { "type": "result",       "message": "# Relatório Final\n..." }
   ```

4. Se `RequiresApproval: true`, o **modo Apicultor** é ativado. O servidor envia um `approval_request` via WebSocket e aguarda a resposta:

   ```json
   // Servidor envia:
   { "type": "approval_request", "id": "req-1712345678901", "tool": "write_file", "args": "{...}" }

   // Cliente responde:
   { "type": "approve", "id": "req-1712345678901", "approved": true }
   ```

5. Ao final, o resultado é salvo na memória vetorial do ChromaDB para uso futuro.

### Configurar seu próprio enxame

Edite `cmd/api/main.go` para definir a política do enxame:

```go
queen.RegisterSwarm("meu-enxame", swarm.Policy{
    MaxNectar:        50000,  // Budget de tokens
    Isolate:          true,   // Contexto isolado por grupo
    RequiresApproval: true,   // Modo Apicultor (HIL)
})
```

### Skills — capacidades reutilizáveis

Uma **skill** encapsula instruções e ferramentas para um domínio específico. Pode ser associada a uma colmeia ou a agentes individuais.

```bash
# Criar skill
curl -X POST http://localhost:8080/api/skills \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Web Research",
    "description": "Pesquisa na web usando DuckDuckGo",
    "instructions": "Use web_search para coletar informações atualizadas antes de responder.",
    "allowed_tools": ["web_search"]
  }'

# Associar à colmeia (rainha usa no meta-planejamento)
curl -X POST http://localhost:8080/api/colmeias/{id}/skills \
  -H "Content-Type: application/json" \
  -d '{ "skill_id": 1 }'

# Associar a agente pré-definido (mesclado no despacho)
curl -X POST http://localhost:8080/api/colmeias/{id}/agentes/{agentId}/skills \
  -H "Content-Type: application/json" \
  -d '{ "skill_id": 1 }'
```

**Como funciona:**

- **Queen-managed** (`queen_managed: true`): as skills da colmeia são injetadas como bloco `SKILLS DISPONÍVEIS` no prompt da Rainha. Ela decide quais especialistas recebem cada skill.
- **Manual** (`queen_managed: false`): as skills de cada agente são mescladas automaticamente no `system_prompt` e nas ferramentas permitidas no momento do despacho.

### Ferramentas disponíveis

| Ferramenta       | Descrição                                                                 |
| ---------------- | ------------------------------------------------------------------------- |
| `list_directory` | Lista arquivos e pastas de um diretório                                   |
| `read_file`      | Lê o conteúdo de um arquivo                                               |
| `write_file`     | Cria ou sobrescreve um arquivo                                            |
| `execute_code`   | Executa código em sandbox Wasm isolado                                    |
| `web_search`     | Busca na internet via DuckDuckGo (respostas diretas, definições, resumos) |
| `search_memory`  | Busca semântica na memória vetorial (ChromaDB)                            |
| `store_memory`   | Salva conhecimento na memória vetorial                                    |

---

## 🔐 Segurança

Cada "passagem de bastão" entre Especialistas é **criptografada com AES-GCM**:

1. Uma chave de sessão efêmera é gerada no início de cada workflow
2. O contexto acumulado é **cifrado antes de ser enviado** para a próxima Especialista
3. A Especialista recebe o payload cifrado, descriptografa, processa e **re-cifra** sua resposta
4. Nenhum contexto trafega em texto puro entre agentes

Isso simula um canal IPC seguro, onde mesmo que um agente seja comprometido, ele não consegue ler o histórico de outros agentes do pipeline.

---

## 🌐 API Reference

O servidor HTTP é iniciado com `./jandaira-api --port 8080` e expõe as seguintes rotas:

### Rotas REST

#### Configuração e Despacho

| Método | Rota            | Descrição                                                |
| ------ | --------------- | -------------------------------------------------------- |
| `POST` | `/api/setup`    | Configura a colmeia na primeira execução                 |
| `POST` | `/api/dispatch` | Envia um objetivo ao enxame para execução (sem estado)   |
| `GET`  | `/api/tools`    | Lista todas as ferramentas disponíveis e seus parâmetros |
| `GET`  | `/ws`           | Abre uma conexão WebSocket para eventos em tempo real    |

#### Sessões

| Método   | Rota                           | Descrição                                      |
| -------- | ------------------------------ | ---------------------------------------------- |
| `GET`    | `/api/sessions`                | Lista todas as sessões                         |
| `POST`   | `/api/sessions`                | Cria uma nova sessão                           |
| `GET`    | `/api/sessions/:id`            | Busca sessão com agentes                       |
| `DELETE` | `/api/sessions/:id`            | Deleta sessão (cascade)                        |
| `POST`   | `/api/sessions/:id/dispatch`   | Despacha workflow para a sessão                |
| `GET`    | `/api/sessions/:id/agents`     | Lista agentes da sessão                        |
| `POST`   | `/api/sessions/:id/documents`  | Faz upload e indexa documento                  |

#### Skills

| Método   | Rota                                              | Descrição                                               |
| -------- | ------------------------------------------------- | ------------------------------------------------------- |
| `GET`    | `/api/skills`                                     | Lista todas as skills                                   |
| `POST`   | `/api/skills`                                     | Cria skill                                              |
| `GET`    | `/api/skills/:id`                                 | Busca skill                                             |
| `PUT`    | `/api/skills/:id`                                 | Atualiza skill                                          |
| `DELETE` | `/api/skills/:id`                                 | Remove skill (desassocia de todas as colmeias e agentes)|
| `GET`    | `/api/colmeias/:id/skills`                        | Lista skills da colmeia                                 |
| `POST`   | `/api/colmeias/:id/skills`                        | Associa skill à colmeia (`{ "skill_id": 1 }`)           |
| `DELETE` | `/api/colmeias/:id/skills/:skillId`               | Remove associação skill-colmeia                         |
| `GET`    | `/api/colmeias/:id/agentes/:agentId/skills`       | Lista skills do agente                                  |
| `POST`   | `/api/colmeias/:id/agentes/:agentId/skills`       | Associa skill ao agente                                 |
| `DELETE` | `/api/colmeias/:id/agentes/:agentId/skills/:skillId` | Remove associação skill-agente                       |

#### Colmeias Persistentes

Colmeias são hives nomeadas e persistentes. Diferente de sessões, uma colmeia pode receber **múltiplas mensagens ao longo do tempo**, mantendo histórico de conversas como contexto. Os agentes podem ser **pré-definidos pelo usuário** (com prompts e ferramentas customizáveis) ou **montados automaticamente pela rainha**.

| Método   | Rota                                    | Descrição                                              |
| -------- | --------------------------------------- | ------------------------------------------------------ |
| `GET`    | `/api/colmeias`                         | Lista todas as colmeias                                |
| `POST`   | `/api/colmeias`                         | Cria colmeia (`queen_managed: true/false`)             |
| `GET`    | `/api/colmeias/:id`                     | Busca colmeia com agentes                              |
| `PUT`    | `/api/colmeias/:id`                     | Atualiza colmeia                                       |
| `DELETE` | `/api/colmeias/:id`                     | Deleta colmeia (cascade)                               |
| `POST`   | `/api/colmeias/:id/dispatch`            | Envia mensagem à colmeia                               |
| `GET`    | `/api/colmeias/:id/historico`           | Lista histórico de conversas                           |
| `GET`    | `/api/colmeias/:id/agentes`             | Lista agentes da colmeia                               |
| `POST`   | `/api/colmeias/:id/agentes`             | Adiciona agente pré-definido                           |
| `PUT`    | `/api/colmeias/:id/agentes/:agentId`    | Edita nome, prompt e ferramentas do agente             |
| `DELETE` | `/api/colmeias/:id/agentes/:agentId`    | Remove agente da colmeia                               |

**Exemplo — criar colmeia com agentes definidos pelo usuário:**

```bash
# 1. Criar colmeia
curl -X POST http://localhost:8080/api/colmeias \
  -H "Content-Type: application/json" \
  -d '{"name": "Colmeia de Pesquisa", "queen_managed": false}'

# 2. Adicionar agente com prompt customizado
curl -X POST http://localhost:8080/api/colmeias/{id}/agentes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Pesquisador Web",
    "system_prompt": "Você é um especialista em pesquisa. Use web_search para coletar informações atualizadas.",
    "allowed_tools": ["web_search", "search_memory", "store_memory"]
  }'

# 3. Enviar primeira mensagem
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Pesquise as principais notícias sobre IA desta semana"}'

# 4. Enviar segunda mensagem (histórico anterior é injetado como contexto)
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Com base na pesquisa anterior, faça um resumo executivo"}'
```

#### `POST /api/dispatch`

```json
// Request
{ "goal": "Crie um arquivo Go que some dois números", "group_id": "enxame-alfa" }

// Response 202
{ "message": "Mission dispatched to the swarm. Follow progress via WebSocket." }
```

#### `GET /api/tools`

```json
// Response 200
{
  "tools": [
    { "name": "write_file", "description": "Cria ou sobrescreve um arquivo", "parameters": { ... } },
    { "name": "execute_code", "description": "Executa código em sandbox Wasm", "parameters": { ... } }
  ]
}
```

#### `GET /api/agents`

```json
// Response 200
{
  "agents": [
    {
      "name": "Desenvolvedora Wasm",
      "system_prompt": "...",
      "allowed_tools": ["write_file", "search_memory"]
    },
    {
      "name": "Auditora de Qualidade",
      "system_prompt": "...",
      "allowed_tools": ["execute_code", "read_file"]
    }
  ]
}
```

---

### Eventos WebSocket (`/ws`)

Todos os eventos trafegam como JSON pelo mesmo canal WebSocket. O Apicultor **não precisa de rotas REST** — a aprovação é feita inteiramente pelo WebSocket.

#### Servidor → Frontend

| `type`             | Quando é disparado                            | Campos relevantes       |
| ------------------ | --------------------------------------------- | ----------------------- |
| `status`           | Mensagens de progresso da Rainha              | `message`               |
| `agent_change`     | Um especialista assume o controle do pipeline | `agent`                 |
| `tool_start`       | Uma ferramenta está prestes a ser executada   | `agent`, `tool`, `args` |
| `approval_request` | A IA quer usar uma ferramenta bloqueada       | `id`, `tool`, `args`    |
| `result`           | Relatório final do workflow                   | `message`               |
| `error`            | Falha ou timeout                              | `message`               |

```json
{ "type": "status",           "message": "🚀 Queen received the goal and is starting the swarm..." }
{ "type": "agent_change",     "agent": "Desenvolvedora Wasm" }
{ "type": "tool_start",       "agent": "Desenvolvedora Wasm", "tool": "write_file", "args": "{...}" }
{ "type": "approval_request", "id": "req-1712345678901", "tool": "write_file", "args": "{...}" }
{ "type": "result",           "message": "# Relatório Final\n..." }
{ "type": "error",            "message": "Mission timeout reached." }
```

#### Frontend → Servidor

| `type`    | Quando enviar                                 | Campos obrigatórios |
| --------- | --------------------------------------------- | ------------------- |
| `approve` | Resposta do Apicultor a um `approval_request` | `id`, `approved`    |

```json
{ "type": "approve", "id": "req-1712345678901", "approved": true }
{ "type": "approve", "id": "req-1712345678901", "approved": false }
```

> **Nota:** O campo `id` deve corresponder exatamente ao `id` recebido no `approval_request`. IDs inválidos ou já processados retornam um evento `error`.

---

## ⚖️ Licença e Uso Comercial (Dual License)

O **Jandaira Swarm OS** é distribuído sob um modelo de licenciamento duplo (*Dual License*), projetado para fomentar o desenvolvimento de código aberto enquanto atende às necessidades de empresas.

* **Uso Open Source (AGPLv3):** O código-fonte está disponível gratuitamente sob a licença [GNU Affero General Public License v3.0](LICENCE). Qualquer pessoa ou organização pode usar, modificar e distribuir o software livremente, desde que todas as modificações e o código-fonte de projetos derivados (incluindo serviços SaaS e backend prestados via rede) também sejam disponibilizados sob a mesma licença.
* **Uso Comercial Empresarial:** Para empresas que desejam integrar o Jandaira em produtos comerciais proprietários, serviços web (SaaS) ou backends corporativos sem a obrigatoriedade de abrir o código-fonte de suas próprias aplicações, oferecemos a **Licença Comercial**.

**Resumo:** O projeto é aberto e gratuito para a comunidade de código aberto. Organizações com restrições de compliance podem adquirir uma licença comercial para manter sua propriedade intelectual totalmente protegida. Para detalhes comerciais, entre em contato.

---

## 🤝 Contribuindo

Pull Requests são bem-vindos! Abra uma issue descrevendo a feature ou bug antes de começar.

---

_Jandaira: Autonomia, Segurança e a Força do Enxame Brasileiro._ 🐝
