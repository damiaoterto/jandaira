# 🐝 Jandaira Swarm OS

<p align="center">
  <img src="jandaira.png" alt="Jandaira Logo"/>
</p>

Um framework simples e poderoso de **multiagentes autônomos** escrito em Go. Inspirado na abelha nativa brasileira **Jandaíra**, ele permite criar "colmeias" de IAs que trabalham juntas de forma segura e eficiente.

> 🌐 [English](docs/README.en.md) · **Português** · [Español](docs/README.es.md) · [中文](docs/README.zh.md) · [Русский](docs/README.ru.md)

---

## 🚀 Setup e Instalação (Comece por aqui!)

Rodar o Jandaira é muito fácil! O sistema já vem com seu próprio banco de dados embutido, então você **não precisa de Docker** se for rodar só a API.

### 1. Pré-requisitos
* Ter o [Go](https://go.dev/) (versão 1.22 ou superior) instalado.
* Uma chave de API da OpenAI (ou compatível).



### 2. Escolha como instalar

**Opção A: Instalação Automática (Linux/macOS - Mais Fácil)**
Baixa e configura tudo para você automaticamente.
```bash
curl -fsSL https://github.com/damiaoterto/jandaira/releases/latest/download/install.sh | sudo bash
```
*Painel Frontend: `http://localhost:9000` | API: `http://localhost:8080`*

**Opção B: Via Docker (Sistema Completo)**
Ideal se você quer o Backend + Frontend rodando juntos sem instalar nada no seu PC.
```bash
docker pull ghcr.io/damiaoterto/jandaira:latest
docker run -d -p 8080:8080/tcp -p 9000:9000/tcp ghcr.io/damiaoterto/jandaira:latest
```

**Opção C: Compilando do Código-Fonte**
Para quem quer modificar ou contribuir com o projeto.
```bash
git clone https://github.com/damiaoterto/jandaira.git
cd jandaira
go mod tidy
go run ./cmd/api/main.go --port 8080
```

**Opção D: Instalação no Windows**
Baixe o instalador da [página de releases](https://github.com/damiaoterto/jandaira/releases/latest) e execute como Administrador no PowerShell:
```powershell
powershell.exe -ExecutionPolicy Bypass -File .\install-windows.ps1
```

### 3. Testando sua Colmeia
Após iniciar o servidor (ele estará rodando na porta 8080), você pode enviar um objetivo para a IA:

```bash
curl -X POST http://localhost:8080/api/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Crie um arquivo Go chamado soma.go que some dois números", "group_id": "enxame-alfa"}'
```
Você pode acompanhar o que a IA está fazendo em tempo real pelo WebSocket: `ws://localhost:8080/ws`.

---

## ⚖️ Licença de Uso (Explicada de Forma Simples)

O **Jandaira Swarm OS** possui um modelo de licença dupla para ser justo com a comunidade e com as empresas.

1. **Para a Comunidade (100% Grátis - AGPLv3):**
   Você pode baixar, usar, modificar e distribuir o Jandaira de graça. 
   ⚠️ **A Regra:** Se você usar o Jandaira para criar um produto, projeto ou serviço na web, **você é obrigado a deixar o código-fonte do seu projeto aberto e público** para todo mundo.

2. **Para Empresas (Licença Comercial):**
   Você quer usar o Jandaira na sua empresa ou criar um produto fechado, mas **não quer** compartilhar o código-fonte do seu sistema? 
   ✅ **A Solução:** Nós vendemos uma **Licença Comercial**. Com ela, você pode usar o Jandaira em projetos privados sem a obrigação de abrir o seu código. Entre em contato conosco!

---

## 📖 O que é o Jandaira?

Inspirado na abelha brasileira que trabalha em conjunto sem precisar de um líder central, nosso sistema divide o trabalho entre vários "agentes IAs":

- **Rainha (`Queen`):** Não executa tarefas. Ela apenas organiza, gerencia o "néctar" (seus tokens/dinheiro) e garante a segurança.
- **Especialistas (`Specialists`):** São as operárias. Cada agente tem uma função específica (ex: desenvolvedor, auditor) e ferramentas limitadas para executar seu trabalho.
- **Apicultor (Você!):** O humano no controle. A IA pode pedir sua aprovação antes de executar ações perigosas.

---

## 🏗️ Como a Arquitetura Funciona

### O Fluxo Principal

```mermaid
sequenceDiagram
    participant C as 👤 Cliente
    participant Q as 👑 Queen (API)
    participant E1 as 🐝 Especialista 1
    participant E2 as 🐝 Especialista 2
    participant H as 👨‍🌾 Apicultor
    participant BD as 🍯 VectorEngine

    C->>Q: Envia Objetivo (POST /api/dispatch)
    Q->>Q: Valida Política e Néctar (Tokens)
    Q->>E1: Passa o bastão (Inicia pipeline)
    E1->>E1: Executa Tarefa (ex: escreve código)
    E1-->>E2: Envia resultado (Payload Criptografado AES-GCM)
    E2->>E2: Executa Tarefa (ex: audita código)
    E2->>H: Pede aprovação? (Approval Request)
    H-->>E2: Humano aprova ação
    E2->>BD: Salva no Banco Vetorial
    Q-->>C: Retorna Resultado Final
```

### Como a Memória Funciona (Curto e Longo Prazo)

Para não gastar muitos tokens e manter a IA inteligente ao longo do tempo, dividimos a memória em duas:

```mermaid
graph TD
    M1[💬 Nova Mensagem] --> STM[🧠 Memória de Curto Prazo RAM]
    STM -->|Encheu o limite?| COMPACT[LLM faz um Resumo]
    COMPACT --> LTM[(🍯 Banco Vetorial Longo Prazo)]
    
    style STM fill:#f9f,stroke:#333,stroke-width:2px
    style LTM fill:#bbf,stroke:#333,stroke-width:2px
```

### Grafo de Conhecimento (IA aprendendo sozinha)

A Rainha aprende com as missões passadas! Se um agente foi bem ao "analisar vendas", ela vai chamá-lo novamente no futuro.

```mermaid
graph LR
    O[Objetivo: Analisar Vendas] --> R{Rainha consulta Grafo}
    R -->|Acha Perfil| A1((Analista de Vendas))
    A1 -->|Expert em| T[Tópico: dados de vendas]
    R -->|Monta equipe com base na experiência| E[Enxame Final]
```

---

## 🔌 Integrações MCP (Model Context Protocol)

O Jandaira suporta integração nativa com qualquer servidor MCP. Cada servidor MCP pertence a uma colmeia (relação um-para-muitos) e suas ferramentas ficam disponíveis automaticamente durante o despacho.

**Transportes suportados:**
- **Stdio** — inicia o servidor MCP como subprocesso sandboxado via E2B (`sbx exec mcp-base <cmd>`). Ideal para bancos de dados, sistema de arquivos, ferramentas locais. O comando é auto-envelopado pelo serviço.
- **SSE** — conecta a servidores MCP remotos via HTTP+SSE (protocolo MCP 2024-11-05).
- **HTTP** — conecta a servidores modernos via Streamable HTTP (protocolo MCP 2025-03-26). Ex: Context7.

```bash
# 1. Cria um servidor MCP de PostgreSQL já dentro de uma colmeia
#    O comando ["npx", ...] é auto-envelopado como "sbx exec mcp-base npx ..."
curl -X POST http://localhost:8080/api/colmeias/{id}/mcp-servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "postgres-analytics",
    "transport": "stdio",
    "command": ["npx", "-y", "@modelcontextprotocol/server-postgres", "postgres://user:pass@localhost/db"],
    "active": true
  }'

# 2. Despacha — ferramentas MCP carregam automaticamente
#    A Rainha vê ferramentas como "postgres_analytics_query" e as atribui a especialistas
curl -X POST http://localhost:8080/api/colmeias/{id}/dispatch \
  -H "Content-Type: application/json" \
  -d '{"goal": "Liste os pedidos do último mês e calcule o faturamento total"}'

# Servidor MCP via HTTP (ex: Context7)
curl -X POST http://localhost:8080/api/colmeias/{id}/mcp-servers \
  -H "Content-Type: application/json" \
  -d '{"name": "context7", "transport": "http", "url": "https://mcp.context7.com/mcp", "active": true}'
```

> Documentação completa: [`docs/mcp-engine.md`](docs/mcp-engine.md)

---

## 🪝 Webhook Engine (Integrações fáceis)

Você pode conectar o Jandaira ao GitHub, Slack, etc. A IA é ativada automaticamente quando um evento acontece.

```mermaid
graph LR
    GH[GitHub / CI-CD] -->|POST Payload JSON| W[Webhook Engine]
    W -->|Valida Senha e Transforma JSON| Q[👑 Rainha]
    Q -->|Executa a Missão| E[🐝 Enxame]
    E -->|Outbound Webhook| D[Discord / Slack]
```

---

## ⚡ Por que escolher Go ao invés de Python?

| Comparação                  | NanoClaw (Python)         | Jandaira (Go) 🏆                       |
| --------------------------- | ------------------------- | -------------------------------------- |
| **Desempenho**              | Pesado, exige threads     | Super leve com Goroutines nativas      |
| **Instalação**              | Requer dependências/Docker| Um único arquivo executável!           |
| **Segurança entre Agentes** | Não existe                | Criptografia nativa AES-GCM            |
| **Banco de Dados IA**       | Requer serviços externos  | Banco Vetorial (HNSW) já embutido!     |
| **Aprovação Humana**        | Gambiarras externas       | Nativo via WebSocket em tempo real     |

---

## 🌐 Referência Rápida da API

| Ação | Rota HTTP | Descrição |
| --- | --- | --- |
| **Despachar Missão** | `POST /api/dispatch` | Envia um trabalho para a colmeia. |
| **Listar Ferramentas** | `GET /api/tools` | Veja o que as IAs podem fazer. |
| **Tempo Real** | `GET /ws` | WebSocket para acompanhar os IAs e aprovar ações. |
| **Webhooks** | `POST /api/webhooks/:slug` | Dispara um gatilho externo. |
| **MCP da Colmeia** | `GET/POST /api/colmeias/:id/mcp-servers` | Cria/lista servidores MCP de uma colmeia. |
| **MCP (detalhe)** | `GET/PUT/DELETE /api/colmeias/:id/mcp-servers/:sid` | Consulta, atualiza ou remove um servidor MCP. |

---

## 🤝 Contribuindo

Pull Requests são muito bem-vindos! Por favor, abra uma issue descrevendo o que deseja melhorar antes de começar a codificar.

_Jandaira: Autonomia, Segurança e a Força do Enxame Brasileiro._ 🐝
