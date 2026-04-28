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
- A **Colmeia (`Honeycomb`)** é o sistema de memória persistente em duas camadas: o `ShortTermMemory` mantém o contexto recente em RAM com expiração automática por TTL; o `VectorEngine` (BadgerDB + HNSW embutido) arquiva o conhecimento consolidado como vetores de longo prazo — sem dependências externas.
- O **Grafo de Conhecimento (`KnowledgeGraph`)** mapeia relações entre agentes, tópicos e ferramentas — a Rainha consulta esse grafo antes de cada missão para reutilizar perfis de especialistas que já obtiveram sucesso em objetivos semelhantes.
- O **Apicultor** é o humano no loop: pode aprovar ou bloquear qualquer ação da IA antes de ela ser executada.
- Os **Webhooks** são gatilhos HTTP externos que permitem sistemas de CI/CD, GitHub, Prometheus e similares dispararem enxames de agentes apenas chamando uma URL. O **GoalTemplate** usa `text/template` do Go para transformar o payload JSON recebido no objetivo que a Rainha processará — sem recompilar o binário.

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
│  Tools: execute_code │          │  Tools: execute_code │
│         search_mem   │          │         store_memory │
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
│             🍯 Honeycomb (VectorEngine)                  │
│   Resultado do workflow é embeddado e indexado            │
│   Memória de longo prazo embutida (BadgerDB + HNSW)      │
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
    │   ├── memory.go        # Honeycomb: interface + tipos Result/Document
    │   ├── hnsw.go          # Índice HNSW (vizinhos aproximados)
    │   ├── vector_engine.go # VectorEngine: BadgerDB + HNSW embutido
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
- **Compactação automática**: quando o buffer atinge `maxEntries`, o LLM sumariza o histórico acumulado em um parágrafo denso → o resumo é embeddado e arquivado no VectorEngine como `short_term_archive` → o buffer RAM é zerado
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
│  VectorEngine (Longo Prazo)      │
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
| **Memória vetorial**          | Pinecone / externo    | ✅ VectorEngine embutido (BadgerDB + HNSW) |
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

# Chave OpenAI
export OPENAI_API_KEY="sk-..."
```

> **Nenhum Docker necessário.** O banco vetorial (`VectorEngine`) é embutido no binário e persiste em `~/.config/jandaira/vectordb/` automaticamente.

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

#### Opção 3 — Docker (Fullstack)

Para rodar o projeto completo (Backend e Frontend) via Docker, basta baixar e rodar a imagem oficial:

```bash
docker pull ghcr.io/damiaoterto/jandaira:latest
docker run -d -p 8080:8080/tcp -p 9000:9000/tcp ghcr.io/damiaoterto/jandaira:latest
```

O painel Frontend estará disponível em `http://localhost:9000` e a API em `http://localhost:8080`.

#### Opção 4 — Script de instalação automática (Linux/macOS)

Detecta o sistema operacional, baixa os binários e o frontend, e registra os serviços para iniciar com o sistema:

```bash
curl -fsSL https://github.com/damiaoterto/jandaira/releases/latest/download/install.sh | sudo bash
```

O painel ficará em `http://localhost:9000` e a API em `http://localhost:8080`.

#### Opção 5 — Windows

Baixe o instalador da [página de releases](https://github.com/damiaoterto/jandaira/releases/latest) e execute como Administrador no PowerShell:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\install-windows.ps1
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
   - **Desenvolvedora Wasm** → compila e executa `soma.go` em sandbox via `execute_code`
   - **Auditora de Qualidade** → valida o resultado e persiste o relatório com `store_memory`

3. Acompanhe o progresso pelo WebSocket:

   ```json
   { "type": "agent_change", "agent": "Desenvolvedora Wasm" }
   { "type": "tool_start",   "agent": "Desenvolvedora Wasm", "tool": "execute_code", "args": "{...}" }
   { "type": "result",       "message": "# Relatório Final\n..." }
   ```

4. Se `RequiresApproval: true`, o **modo Apicultor** é ativado. O servidor envia um `approval_request` via WebSocket e aguarda a resposta:

   ```json
   // Servidor envia:
   { "type": "approval_request", "id": "req-1712345678901", "tool": "execute_code", "args": "{...}" }

   // Cliente responde:
   { "type": "approve", "id": "req-1712345678901", "approved": true }
   ```

5. Ao final, o resultado é embeddado e salvo no VectorEngine local para uso futuro.

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

| Ferramenta       | Descrição                                                                                            |
| ---------------- | ---------------------------------------------------------------------------------------------------- |
| `list_directory` | Lista arquivos e pastas de um diretório                                                              |
| `read_file`      | Lê o conteúdo de um arquivo (somente leitura — nenhum dado é persistido em disco pelos agentes)     |
| `execute_code`   | Executa código Go em sandbox Wasm isolado — use para cálculos e processamento de dados               |
| `web_search`     | Busca na internet via DuckDuckGo (respostas diretas, definições, resumos)                            |
| `search_memory`  | Busca semântica no VectorEngine (BadgerDB + HNSW); degrada graciosamente se embedding indisponível   |
| `store_memory`   | **Único mecanismo de persistência permanente.** Salva dados no VectorEngine com campos `type` e `metadata`. Use para registros financeiros, resultados de cálculos e qualquer dado que precise sobreviver entre sessões. |

> **Nota:** `write_file` e `create_directory` foram removidos do toolkit dos agentes. Todo dado persistente vai para o banco vetorial via `store_memory`.

---

## 🔐 Segurança

Cada "passagem de bastão" entre Especialistas é **criptografada com AES-GCM**:

1. Uma chave de sessão efêmera é gerada no início de cada workflow
2. O contexto acumulado é **cifrado antes de ser enviado** para a próxima Especialista
3. A Especialista recebe o payload cifrado, descriptografa, processa e **re-cifra** sua resposta
4. Nenhum contexto trafega em texto puro entre agentes

Isso simula um canal IPC seguro, onde mesmo que um agente seja comprometido, ele não consegue ler o histórico de outros agentes do pipeline.

---

## 🪝 Webhook Engine

O **Webhook Engine** permite que sistemas externos acionem colmeias via HTTP, sem autenticação adicional. Cada webhook possui um `slug` único na URL, um `GoalTemplate` baseado em `text/template` do Go e, opcionalmente, um `secret` para validação HMAC-SHA256.

### Fluxo

```
Sistema externo (GitHub, Prometheus, Slack, CI/CD…)
     │  POST /api/webhooks/monitor-deploy
     │  Body: {"project_name": "Jandaira", "env": "prod"}
     ▼
┌─────────────────────────────────────────────┐
│              Webhook Engine                  │
│                                             │
│  1. Localiza webhook pelo slug              │
│  2. Valida HMAC-SHA256 (se secret definido) │
│  3. Renderiza GoalTemplate com o payload:   │
│     "Analise o deploy de Jandaira em prod"  │
│  4. Chama Queen.DispatchWorkflow            │
└─────────────────────────────────────────────┘
     │
     ▼
Queen → AssembleSwarm / BuildSpecialists → Resultado via WebSocket
```

### GoalTemplate

Usa `text/template` nativo do Go — qualquer campo do payload JSON é referenciável com `{{.campo}}`:

```
"Analise o deploy do projeto {{.project_name}} no ambiente {{.env}}"
"Alerta: {{.alertname}} — instância {{.instance}} ({{.severity}})"
"PR #{{.number}} em {{.repository.name}}: {{.title}}"
```

> **Dica:** Campos aninhados como `{{.repository.name}}` funcionam desde que o valor seja um objeto JSON desserializado como `map[string]interface{}`.

### Validação HMAC-SHA256

Se `secret` estiver configurado, o chamador deve enviar o header:

```
X-Hub-Signature-256: sha256=<hex-encoded-HMAC-SHA256-do-body>
```

Compatível com o padrão GitHub Webhooks. Payloads sem assinatura válida recebem `401 Unauthorized`.

### Outbound Webhooks (Webhooks de Saída)

O Jandaira também suporta **Outbound Webhooks**, permitindo que a colmeia envie automaticamente o resultado do seu processamento para sistemas externos (Discord, Slack, etc.) assim que a missão for concluída. O formato da requisição é personalizável via `BodyTemplate`.

O template possui funções (filtros) integradas essenciais para enviar payloads JSON estruturados:
- `json`: Escapa o texto com segurança (quebras de linha, aspas) para dentro do JSON.
- `truncate <limite>`: Corta o tamanho máximo da string, evitando erros em APIs como a do Discord (limite de 2000 caracteres).
- `normalize`: Limpa o texto da IA, removendo metadados de orquestração, logs internos e dumps de memória, entregando apenas o relatório final do último agente.

**Exemplo de payload de saída:**
```json
{
  "content": {{.result | normalize | truncate 1900 | json}}
}
```

### Exemplo completo

```bash
# 1. Criar webhook
curl -X POST http://localhost:8080/api/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Monitor Deploy",
    "slug": "monitor-deploy",
    "colmeia_id": "<id-da-colmeia>",
    "secret": "meu-segredo",
    "goal_template": "Analise o deploy do projeto {{.project_name}} no ambiente {{.env}}",
    "active": true
  }'

# 2. Disparar (sistema externo com assinatura HMAC)
BODY='{"project_name":"Jandaira","env":"prod"}'
SIG="sha256=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "meu-segredo" | awk '{print $2}')"

curl -X POST http://localhost:8080/api/webhooks/monitor-deploy \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: $SIG" \
  -d "$BODY"

# Resposta 202:
# {
#   "webhook_slug": "monitor-deploy",
#   "colmeia_id": "...",
#   "historico_id": "...",
#   "mode": "queen_managed"
# }
```

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

Colmeias são hives nomeadas e persistentes. Diferente de sessões, uma colmeia pode receber **múltiplas mensagens ao longo do tempo**, mantendo histórico de conversas como contexto. Os agentes podem ser **pré-definidos pelo usuário** (com prompts e ferramentas customizáveis, somente quando `queen_managed=false`) ou **montados automaticamente pela rainha** (`queen_managed=true`). Tentar adicionar agentes pré-definidos a uma colmeia `queen_managed=true` retorna `409 Conflict`.

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
| `POST`   | `/api/colmeias/:id/agentes`             | Adiciona agente pré-definido (`queen_managed=false` obrigatório) |
| `GET`    | `/api/colmeias/:id/agentes/:agentId`    | Busca agente por ID                                    |
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

#### Webhooks

| Método   | Rota                    | Descrição                                             |
| -------- | ----------------------- | ----------------------------------------------------- |
| `POST`   | `/api/webhooks/:slug`   | **Público** — aciona workflow da colmeia associada    |
| `GET`    | `/api/webhooks`         | Lista todos os webhooks                               |
| `POST`   | `/api/webhooks`         | Cria webhook                                          |
| `GET`    | `/api/webhooks/:id`     | Busca webhook por ID                                  |
| `PUT`    | `/api/webhooks/:id`     | Atualiza webhook                                      |
| `DELETE` | `/api/webhooks/:id`     | Remove webhook                                        |

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
    { "name": "execute_code",   "description": "Executa código Go em sandbox Wasm isolado", "parameters": { ... } },
    { "name": "store_memory",   "description": "Persiste dados no VectorEngine embutido", "parameters": { ... } }
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
      "allowed_tools": ["execute_code", "search_memory"]
    },
    {
      "name": "Auditora de Qualidade",
      "system_prompt": "...",
      "allowed_tools": ["execute_code", "store_memory", "read_file"]
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
