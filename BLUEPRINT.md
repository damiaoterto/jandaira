# Projeto Jandaira: Framework de Multiagentes Autônomos em Go

**Versão:** 1.4

**Codinome:** _Melipona_ (Inspirado na abelha resiliente da Caatinga)

**Status:** Definição Arquitetural Avançada

## 1. Visão Geral

O **Jandaira** é um sistema de agentes autônomos de alto desempenho projetado para ser a alternativa mais segura, rápida e portável aos frameworks atuais. Desenvolvido em **Golang**, ele utiliza uma arquitetura baseada em **Micro-Sandboxing Wasm**, permitindo que agentes operem de forma totalmente isolada sem a necessidade de Docker ou runtimes externos.

## 2. Diferenciais e Melhorias de Processo (Jandaira vs. NanoClaw)

Baseado nos pilares do NanoClaw, a Jandaira evolui os processos para o ecossistema Go:

### 2.1. Processo Único e Binário Estático

Diferente de sistemas que exigem múltiplos serviços, a Jandaira é um **processo único em Go**.

- **Vantagem:** Gerencia mensagens, filas de tarefas, instâncias Wasm e IPC sem necessidade de message brokers (Redis/RabbitMQ) ou microsserviços. Tudo ocorre em memória, garantindo latência mínima e facilidade de deploy (um único executável).

### 2.2. Isolamento por Grupo (Wasm Namespaces)

Cada "Enxame" ou grupo de tarefas recebe um isolamento rigoroso:

- **Células Wasm:** Em vez de contêineres Docker, cada grupo recebe instâncias Wasm isoladas.
- **Namespace Privado:** Cada grupo possui seu próprio sistema de arquivos virtual (VFS), memória isolada e sessão de LLM. Dados de um grupo são inacessíveis para outros.

### 2.3. Controle de Concorrência e "Néctar"

Implementação de um escalonador de tarefas robusto:

- **GroupQueue:** Limita a execução simultânea de agentes (default: 3 por grupo) para preservar recursos.
- **Ordenação FIFO:** Tarefas são processadas na ordem de chegada por grupo.
- **Resiliência:** Sistema de _backoff_ exponencial para tentativas de reexecução em caso de falhas de API ou recursos.
- **Budget de Néctar:** Monitoramento em tempo real do custo de tokens e tempo de CPU, interrompendo agentes que excedam o orçamento.

### 2.4. IPC via Memória e Host Functions

Superamos a comunicação via arquivos JSON no disco (lenta e insegura):

- **Chamadas Tipadas:** O agente (Wasm) se comunica com o Host (Go) através de funções exportadas em memória.
- **Validação em Tempo Real:** O Host valida cada solicitação, verifica permissões e executa a tarefa, retornando o resultado diretamente para a memória do agente.

## 3. O Ecossistema do Enxame

### 3.1. A Rainha (The Queen - Orchestrator)

- **Gestão de Políticas:** Valida se uma tarefa é segura e se há "Néctar" disponível.
- **HIL (Human-in-the-loop):** Intervém em ações críticas solicitando assinatura do usuário via Web ou CLI.

### 3.2. As Operárias (The Workers - Agents)

- **Execução em Células:** Rodam em ambiente `wazero` isolado.
- **Interface de Ferramentas:** Utilizam o contrato `Tool` para interagir com o mundo real.

### 3.3. As Batedoras (The Scouts - RAG)

- **Recuperação:** Consultam o **LanceDB** local (Embedded) para buscar contexto relevante.

## 4. Especificações Técnicas (Tech Stack)

- **Linguagem:** Go 1.22+ (Performance e concorrência nativa).
- **Wasm Engine:** `github.com/tetratelabs/wazero` (Runtime 100% Go).
- **Vector DB:** **LanceDB** (Embedded) - Armazenamento colunar rápido e local.
- **LLM Contract:** Interface `Linguist` (Abstração para OpenAI, Anthropic, Gemini, Ollama).
- **Interfaces:** CLI (Bubbletea) e Web UI (Svelte/React via `go:embed`).

## 5. O Contrato de Inteligência (`pkg/llm`)

```
type Brain interface {
    // Chat processa mensagens e retorna a resposta da IA
    Chat(ctx context.Context, prompt string, history []Message) (string, error)
    // Embed gera vetores para o LanceDB
    Embed(ctx context.Context, text string) ([]float32, error)
    // GetNectarStats retorna consumo de tokens do modelo
    GetNectarStats() ConsumptionReport
}

```

## 6. Fluxo de Execução Seguro

1.  **Requisição:** Usuário envia comando via CLI ou Web.
2.  **Célula:** A Rainha instancia uma Célula Wasm e um VFS isolado.
3.  **Voo:** A Operária solicita ferramentas via Host Functions.
4.  **Mediação:** O Go valida a chamada e o contexto de segurança.
5.  **Reflexão:** Resultados são indexados no LanceDB para persistência de memória de curto/longo prazo.
6.  **Conclusão:** O relatório final é consolidado e o "Néctar" restante é computado.

_Jandaira: Autonomia, Segurança e a Força do Enxame Brasileira._
