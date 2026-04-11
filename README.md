# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="jandaira.png" alt="Jandaira Logo"/>
</p>

Um framework de **multiagentes autônomos** escrito em Go, inspirado na inteligência coletiva da abelha nativa brasileira _Melipona subnitida_ — a **Jandaíra**.

---

> 🌐 [English](docs/README.en.md) · **Português** · [中文](docs/README.zh.md) · [Русский](docs/README.ru.md)

> 📦 [**Download de binários pré-compilados**](https://github.com/damiaoterto/jandaira/releases) — Linux, Windows, macOS e Raspberry Pi

---

## 📖 Por que "Jandaira"?

A **Jandaíra** (_Melipona subnitida_) é uma abelha sem ferrão endêmica da Caatinga. Pequena, resiliente, e extraordinariamente cooperativa — ela não precisa de um líder centralizado para construir uma colmeia funcional. Cada operária conhece seu papel, executa sua tarefa com autonomia e devolve o resultado para o coletivo.

Esse é exatamente o modelo de arquitetura que o projeto implementa:

- A **Rainha (`Queen`)** não executa tarefas — ela orquestra, valida políticas e garante segurança.
- As **Especialistas (`Specialists`)** são agentes leves com ferramentas restritas, executando em silos isolados.
- O **Néctar** é a metáfora para o orçamento de tokens: cada agente consome néctar; quando acaba, a colmeia para.
- A **Colmeia (`Honeycomb`)** é a memória vetorial compartilhada — o conhecimento coletivo que persiste entre missões.
- O **Apicultor** é o humano no loop: pode aprovar ou bloquear qualquer ação da IA antes de ela ser executada.

---

## 🏗️ Arquitetura

### Visão Geral do Fluxo

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI (Bubble Tea)                         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  👤 Usuário digita objetivo  →  👑 Queen recebe a meta  │   │
│  └─────────────────────────────────────────────────────────┘   │
└──────────────────────────┬──────────────────────────────────────┘
                           │ DispatchWorkflow()
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Queen (Orquestradora)                         │
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
│   RequiresApproval=true → UI pausa e exibe o pedido      │
│   S = autoriza a ferramenta │ N = bloqueia e informa IA  │
└──────────────────────────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────┐
│                   🍯 Honeycomb (Vector DB)                │
│   Resultado do workflow é embeddado e indexado            │
│   Memória de longo prazo compartilhada entre missões     │
└──────────────────────────────────────────────────────────┘
```

### Mapa de Pacotes

```
jandaira/
├── cmd/
│   └── cli/
│       └── main.go          # Entrypoint: monta a colmeia e inicia a UI
│
└── internal/
    ├── brain/               # Contratos de IA (Brain, Honeycomb)
    │   ├── open_ai.go       # Implementação OpenAI (Chat + Embed)
    │   └── local_vector.go  # Vector DB local (JSON embeddings)
    │
    ├── queue/               # Escalonador FIFO com concorrência limitada
    │   └── group_queue.go   # GroupQueue: N workers por grupo
    │
    ├── security/            # Criptografia de payloads inter-agentes
    │   └── crypto.go        # AES-GCM Seal/Open + geração de chave
    │
    ├── swarm/               # Núcleo do sistema de agentes
    │   ├── queen.go         # Orquestradora: políticas, HIL, pipeline
    │   └── specialist.go    # Definição de Especialista
    │
    ├── tool/                # Ferramentas disponíveis aos agentes
    │   ├── list_directory.go
    │   ├── search_memory.go
    │   └── wasm.go          # Sandbox de execução via wazero
    │
    └── ui/
        └── cli.go           # Interface Bubble Tea (TUI)
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
| **Human-in-the-Loop**         | Opcional / externo    | ✅ Nativo: modo Apicultor              |
| **Budget de tokens**          | Manual                | ✅ `NectarUsage` automático por enxame |
| **Memória vetorial**          | Pinecone / externo    | ✅ Embedded (local, sem servidor)      |
| **Deploy**                    | Múltiplos serviços    | ✅ Binário único estático              |
| **Interface TUI**             | Inexistente           | ✅ Bubble Tea com styles Lipgloss      |
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

# Opcional: Defina via variável de ambiente (Pipeline/CI)
export OPENAI_API_KEY="sk-..."
# NOTA: O Assistente Interativo (Wizard) também pode solicitar essa chave
# no primeiro acesso e guardá-la de forma oculta no Cloud Vault nativo (`~/.config/jandaira/.secrets`).
```

### Instalação

#### Opção 1 — Baixar binário pré-compilado _(recomendado)_

Acesse a página de [**Releases**](https://github.com/damiaoterto/jandaira/releases) e baixe o binário para o seu sistema:

| Sistema          | Arquivo                                       |
| ---------------- | --------------------------------------------- |
| Linux x86-64     | `jandaira-linux`                              |
| Windows          | `jandaira-windows.exe` / `jandaira-setup.exe` |
| macOS            | `jandaira-macos`                              |
| Raspberry Pi 4/5 | `jandaira-linux-arm64`                        |
| Raspberry Pi 2/3 | `jandaira-linux-armv7`                        |

```bash
# Linux/macOS: tornar executável
chmod +x jandaira-linux
./jandaira-linux
```

#### Opção 2 — Compilar a partir do código-fonte

```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira

# Baixar dependências
go mod tidy

# Compilar
go build -o jandaira ./cmd/cli/
```

### Executar a colmeia

```bash
./jandaira
```

Você verá o painel TUI da Jandaira:

```
╔══════════════════════════════════╗
║   🍯  Jandaira Swarm OS  🍯       ║
║   Swarm Intelligence · Powered by Go ║
╚══════════════════════════════════╝

✦ A Colmeia Jandaira despertou. As operárias aguardam as suas ordens.

╭──────────────────────────────────────╮
│ 🐝 Objetivo  Diga à Rainha o que...  │
╰──────────────────────────────────────╯
  ↵ enviar   esc / ctrl+c sair
```

### Exemplo: criar e testar um arquivo Go

1. Digite seu objetivo no campo de entrada e pressione **Enter**:

   ```
   Crie um arquivo Go chamado soma.go que some dois números e imprima o resultado
   ```

2. A Rainha distribui a tarefa para a pipeline de Especialistas:
   - **Desenvolvedora Wasm** → escreve `soma.go` usando `write_file`
   - **Auditora de Qualidade** → executa o código com `execute_code` e gera um relatório

3. Se `RequiresApproval: true`, o **modo Apicultor** é ativado a cada uso de ferramenta:

   ```
   ┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
   ⠿  ⚠️  A IA quer usar a ferramenta  'write_file'

   ▸ filename:  soma.go
   ▸ content:
     package main

     import "fmt"

     func main() {
         fmt.Println(1 + 2)
     }

   👨‍🌾 Você autoriza? (S = sim / N = não)
   ┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
   ```

   - Pressione **S** (ou Y) para autorizar — a Rainha continua
   - Pressione **N** para bloquear — a IA é informada e recalcula sua abordagem

4. Ao final, o relatório é exibido no histórico e salvo na memória vetorial local (`.jandaira/data`).

### Configurar seu próprio enxame

Edite `cmd/cli/main.go` para definir suas próprias Especialistas e política:

```go
// Política do enxame
queen.RegisterSwarm("meu-enxame", swarm.Policy{
    MaxNectar:        50000,  // Budget de tokens
    Isolate:          true,   // Contexto isolado por grupo
    RequiresApproval: true,   // Modo Apicultor (HIL)
})

// Especialistas em pipeline
pesquisadora := swarm.Specialist{
    Name: "Pesquisadora",
    SystemPrompt: `Você é uma pesquisadora. Use search_memory para buscar
                   contexto relevante e retorne um resumo detalhado.`,
    AllowedTools: []string{"search_memory"},
}

redatora := swarm.Specialist{
    Name: "Redatora",
    SystemPrompt: `Você é uma redatora técnica. Com base no resumo recebido,
                   use write_file para criar um relatório em Markdown.`,
    AllowedTools: []string{"write_file"},
}

workflow := []swarm.Specialist{pesquisadora, redatora}
```

### Ferramentas disponíveis

| Ferramenta       | Descrição                                      |
| ---------------- | ---------------------------------------------- |
| `list_directory` | Lista arquivos e pastas de um diretório        |
| `read_file`      | Lê o conteúdo de um arquivo                    |
| `write_file`     | Cria ou sobrescreve um arquivo                 |
| `execute_code`   | Executa código em sandbox Wasm isolado         |
| `search_memory`  | Busca semântica na memória vetorial da colmeia |
| `store_memory`   | Salva conhecimento na memória vetorial         |

---

## 🔐 Segurança

Cada "passagem de bastão" entre Especialistas é **criptografada com AES-GCM**:

1. Uma chave de sessão efêmera é gerada no início de cada workflow
2. O contexto acumulado é **cifrado antes de ser enviado** para a próxima Especialista
3. A Especialista recebe o payload cifrado, descriptografa, processa e **re-cifra** sua resposta
4. Nenhum contexto trafega em texto puro entre agentes

Isso simula um canal IPC seguro, onde mesmo que um agente seja comprometido, ele não consegue ler o histórico de outros agentes do pipeline.

---

## 🌐 API Reference (Modo Servidor)

O servidor HTTP é iniciado com `./jandaira-api --port 8080` e expõe as seguintes rotas:

### Rotas REST

| Método | Rota            | Descrição                                                |
| ------ | --------------- | -------------------------------------------------------- |
| `POST` | `/api/dispatch` | Envia um objetivo ao enxame para execução                |
| `GET`  | `/api/tools`    | Lista todas as ferramentas disponíveis e seus parâmetros |
| `GET`  | `/api/agents`   | Lista os especialistas do workflow configurado           |
| `GET`  | `/ws`           | Abre uma conexão WebSocket para eventos em tempo real    |

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

Todos os eventos trafegam como JSON pelo mesmo canal WebSocket. O Apicultor **não precisa mais de rotas REST** — a aprovação é feita inteiramente pelo WebSocket.

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
// Exemplos de eventos recebidos pelo frontend:
{ "type": "status",     "message": "🚀 Queen received the goal and is starting the swarm..." }
{ "type": "agent_change", "agent": "Desenvolvedora Wasm" }
{ "type": "tool_start", "agent": "Desenvolvedora Wasm", "tool": "write_file", "args": "{...}" }
{ "type": "approval_request", "id": "req-1712345678901", "tool": "write_file", "args": "{...}" }
{ "type": "result",     "message": "# Relatório Final\n..." }
{ "type": "error",      "message": "Mission timeout reached." }
```

#### Frontend → Servidor

| `type`    | Quando enviar                                 | Campos obrigatórios |
| --------- | --------------------------------------------- | ------------------- |
| `approve` | Resposta do Apicultor a um `approval_request` | `id`, `approved`    |

```json
// Aprovar a ação:
{ "type": "approve", "id": "req-1712345678901", "approved": true }

// Negar a ação:
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
