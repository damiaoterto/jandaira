# Projeto Jandaira: Sistema de Multiagentes Autônomos em Go

**Versão:** 1.1

**Codinome:** _Melipona_

**Status:** Planejamento Arquitetural Avançado

## 1. Visão Geral

O **Jandaira** é um framework de agentes autônomos de alto desempenho, desenvolvido em Golang, projetado para ser leve, seguro e portável. Ele opera como um único binário com interface Web e CLI integradas, utilizando **Inteligência de Enxame (Swarm Intelligence)** para resolver tarefas complexas.

## 2. Evolução Tecnológica: Jandaira vs. NanoClaw

Baseado na análise do estado da arte (NanoClaw), a Jandaira implementa as seguintes melhorias arquiteturais:

Processo

Abordagem NanoClaw

**Abordagem Jandaira (Melhoria)**

**Orquestração**

Processo único Node.js

**Single Binary Go (Static Linking):** Sem runtime externo; performance nativa.

**Isolamento**

Contêineres (Docker/Podman)

**Micro-Sandboxing Wasm:** Isolamento a nível de instrução, início em microssegundos e sem dependência de daemon.

**Concorrência**

Fila FIFO com Backoff

**Scheduler de Néctar:** Priorização baseada em tokens, urgência e saúde do enxame usando `context` nativo do Go.

**Comunicação (IPC)**

Arquivos JSON no disco

**Typed Memory IPC:** Protobuf via Wasm exports e Channels de Go. Muito mais rápido e seguro.

## 3. Arquitetura do Enxame (Agentes)

### 3.1. A Rainha (The Queen)

- **Orquestradora de Políticas:** Gerencia o ciclo de vida dos agentes e valida permissões.
- **Diferencial:** Implementa o `Policy Engine` que decide se uma Operária pode ou não "sair para coletar" (acessar I/O).

### 3.2. As Operárias (The Workers)

- **Execução Especializada:** Cada operária roda em seu próprio **Favo (Cell)** isolado em Wasm.
- **Isolamento por Grupo:** Grupos de tarefas recebem namespaces isolados de memória e banco de dados vetorial.

### 3.3. As Batedoras (The Scouts)

- **Recuperação e RAG:** Alimentam o **LanceDB** e buscam informações externas.

## 4. O "Ferrão": Pilares de Segurança e Processos

### 4.1. Micro-Isolamento por Grupo (Wasm Cells)

Diferente do NanoClaw que cria contêineres OS-level, a Jandaira cria **instâncias Wasm isoladas**.

- Cada grupo de tarefas tem seu próprio `Namespace` de memória.
- Agentes de um grupo nunca enxergam os dados ou processos de outro.
- O sistema de arquivos é virtualizado (VFS), restringindo o agente a diretórios específicos.

### 4.2. Controle de Concorrência e "Néctar"

O gerenciamento de carga é feito via `Worker Pools` em Go:

- **GroupQueue Atômica:** Limitação de execução simultânea para evitar saturação de CPU/API.
- **Backoff Exponencial:** Em caso de erro de rede ou limite de taxa da OpenAI, o agente entra em estado de "hibernação" temporária.
- **Cancelamento via Context:** Se o usuário cancelar na Web UI, o sinal de cancelamento se propaga instantaneamente por todas as Goroutines do grupo.

### 4.3. IPC de Alta Performance

Substituímos a consulta de arquivos JSON por:

- **Interface Guardada:** A comunicação entre o Host (Go) e o Agente (Wasm) ocorre via memória compartilhada estritamente validada.
- **Validação de Mensagens:** Toda instrução enviada pelo agente é validada contra o contrato `Tool` antes de ser executada pelo Host.

## 5. Especificações Técnicas (Tech Stack)

- **Core:** Go 1.22+ (Runtime de alta performance).
- **Sandboxing:** `wazero` (Zero-dependency WebAssembly runtime para Go).
- **Memória Vetorial:** `LanceDB` (Embedded) - Armazenamento colunar rápido.
- **Contratos LLM:** Interface `Linguist` (Abstração para OpenAI, Anthropic, etc).
- **Interface:** CLI (Cobra/Bubbletea) e Web UI (SvelteKit via `go:embed`).

## 6. Fluxo de Trabalho (O Voo da Abelha)

1.  **Entrada:** O usuário define uma meta via Web ou CLI.
2.  **Planejamento:** A Rainha cria um `Grupo de Tarefas` e aloca `Néctar` (Budget de tokens/tempo).
3.  **Isolamento:** Uma célula Wasm é instanciada para cada Operária necessária.
4.  **Execução:** Operárias solicitam ferramentas. O Host valida a "assinatura" da ferramenta e executa.
5.  **Persistência:** Resultados são indexados no LanceDB local para futuras consultas.
6.  **Entrega:** O resultado final é consolidado e apresentado com o log de auditoria.

_Jandaira: Eficiência da Caatinga, Segurança de Ferro._
