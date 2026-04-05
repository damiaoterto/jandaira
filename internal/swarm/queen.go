package swarm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/tool"
)

// Policy defines the safety rules and resource limits for a swarm.
type Policy struct {
	MaxNectar        int
	Isolate          bool
	RequiresApproval bool
}

// Specialist representa uma abelha com um papel específico e ferramentas limitadas.
type Specialist struct {
	Name         string
	SystemPrompt string
	AllowedTools []string
}

// Queen is the central orchestrator of the hive.
type Queen struct {
	Queue             *queue.GroupQueue
	Brain             brain.Brain
	Honeycomb         brain.Honeycomb
	mu                sync.RWMutex
	Policies          map[string]Policy
	NectarUsage       map[string]int
	Tools             map[string]tool.Tool
	LogFunc           func(string)
	AskPermissionFunc func(toolName string, args string)
	AgentChangeFunc   func(agentName string)
	ToolStartFunc     func(agentName string, toolName string, args string)
	ApprovalChan      chan bool
}

// NewQueen initializes the hive's sovereign.
func NewQueen(q *queue.GroupQueue, b brain.Brain, h brain.Honeycomb) *Queen {
	return &Queen{
		Queue:        q,
		Brain:        b,
		Honeycomb:    h,
		Policies:     make(map[string]Policy),
		NectarUsage:  make(map[string]int),
		Tools:        make(map[string]tool.Tool),
		ApprovalChan: make(chan bool, 1), // buffered so the UI never deadlocks
	}
}

func (q *Queen) logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if q.LogFunc != nil {
		q.LogFunc(msg)
	} else {
		fmt.Println(msg)
	}
}

// EquipTool dá à Rainha o conhecimento de uma nova ferramenta
func (q *Queen) EquipTool(t tool.Tool) {
	q.Tools[t.Name()] = t
}

// RegisterSwarm sets up the rules for a specific group of agents.
func (q *Queen) RegisterSwarm(groupID string, p Policy) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Policies[groupID] = p
	q.NectarUsage[groupID] = 0
}

// DispatchWorkflow executa uma cadeia de especialistas (Passagem de Bastão)
func (q *Queen) DispatchWorkflow(ctx context.Context, groupID string, goal string, pipeline []Specialist) (<-chan string, <-chan error) {
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	q.mu.RLock()
	policy, exists := q.Policies[groupID]
	q.mu.RUnlock()

	if !exists {
		errChan <- fmt.Errorf("swarm group %s not registered", groupID)
		return resultChan, errChan
	}

	job := queue.Job{
		ID: fmt.Sprintf("workflow-%s", groupID), GroupID: groupID, MaxRetries: 1,
		Task: func(ctx context.Context) error {
			contextAccumulator := fmt.Sprintf("Objetivo Original: %s\n\n", goal)

			sessionKey, err := security.GenerateKey()
			if err != nil {
				errChan <- fmt.Errorf("falha ao gerar chave criptográfica: %w", err)
				return err
			}

			for _, specialist := range pipeline {
				q.logf("👑 [Queen] Delegando a fase para: %s", specialist.Name)
				if q.AgentChangeFunc != nil {
					q.AgentChangeFunc(specialist.Name)
				}

				encryptedPayload, err := security.Seal(sessionKey, contextAccumulator)
				if err != nil {
					errChan <- fmt.Errorf("falha ao criptografar payload: %w", err)
					return err
				}
				q.logf("🔐 Payload criptografado (%d bytes) enviado para %s", len(encryptedPayload), specialist.Name)

				encryptedOutput, err := q.runSpecialist(ctx, groupID, specialist, encryptedPayload, sessionKey, policy)
				if err != nil {
					errChan <- fmt.Errorf("a especialista %s falhou: %w", specialist.Name, err)
					return err
				}

				// Descriptografa a resposta recebida da Especialista
				decryptedOutput, err := security.Open(sessionKey, encryptedOutput)
				if err != nil {
					errChan <- fmt.Errorf("falha ao descriptografar resposta de %s: %w", specialist.Name, err)
					return err
				}
				q.logf("🔓 Resposta descriptografada recebida com sucesso de %s", specialist.Name)

				contextAccumulator += fmt.Sprintf("\n--- Trabalho de %s ---\n%s\n", specialist.Name, decryptedOutput)
			}

			q.logf("👑 [Queen] Workflow completo! O enxame finalizou.")

			if q.Honeycomb != nil {
				q.logf("💾 A guardar o conhecimento no LanceDB...")
				vector, err := q.Brain.Embed(ctx, contextAccumulator)
				if err == nil {
					docID := fmt.Sprintf("workflow-%d", time.Now().Unix())
					_ = q.Honeycomb.Store(ctx, groupID, docID, vector, map[string]string{
						"goal":    goal,
						"content": contextAccumulator,
						"type":    "multi_agent_report",
					})
				}
			}

			// Envia o relatório final para a interface UI
			resultChan <- contextAccumulator
			return nil
		},
	}

	q.Queue.Submit(ctx, job)
	return resultChan, errChan
}

// runSpecialist executa o Loop de Agente para uma abelha específica com as suas ferramentas restritas
func (q *Queen) runSpecialist(ctx context.Context, groupID string, spec Specialist, encryptedTaskContext string, sessionKey []byte, p Policy) (string, error) {
	var availableTools []brain.ToolDefinition
	for _, toolName := range spec.AllowedTools {
		if t, ok := q.Tools[toolName]; ok {
			availableTools = append(availableTools, brain.ToolDefinition{
				Name: t.Name(), Description: t.Description(), Parameters: t.Parameters(),
			})
		}
	}

	taskContext, err := security.Open(sessionKey, encryptedTaskContext)
	if err != nil {
		return "", fmt.Errorf("falha de segurança: impossível descriptografar o contexto: %w", err)
	}

	messages := []brain.Message{
		{Role: brain.RoleSystem, Content: spec.SystemPrompt},
		{Role: brain.RoleUser, Content: taskContext},
	}

	for i := 0; i < 10; i++ {
		response, toolCalls, report, err := q.Brain.Chat(ctx, messages, availableTools)
		if err != nil {
			return "", err
		}

		q.mu.Lock()
		q.NectarUsage[groupID] += report.TotalTokens
		q.mu.Unlock()

		if len(toolCalls) == 0 {
			q.logf("✅ [%s] Tarefa concluída.", spec.Name)

			encryptedFinalResponse, err := security.Seal(sessionKey, response)
			if err != nil {
				return "", fmt.Errorf("falha ao criptografar resposta final: %w", err)
			}

			return encryptedFinalResponse, nil
		}

		messages = append(messages, brain.Message{
			Role: brain.RoleAssistant, Content: response, ToolCalls: toolCalls,
		})

		for _, call := range toolCalls {
			tool, exists := q.Tools[call.Name]
			var toolResult string

			if !exists {
				toolResult = "Erro: Ferramenta não encontrada."
			} else {
				approved := true
				if p.RequiresApproval {
					if q.AskPermissionFunc != nil {
						q.AskPermissionFunc(call.Name, call.ArgsJSON)
						approved = <-q.ApprovalChan
					}
				}

				if !approved {
					q.logf("🚫 Ação bloqueada pelo Apicultor: %s", call.Name)
					toolResult = fmt.Sprintf("ERRO: O Apicultor (usuário humano) NEGOU a permissão para você executar '%s'. Aborte esta tentativa.", call.Name)
				} else {
					if q.ToolStartFunc != nil {
						q.ToolStartFunc(spec.Name, call.Name, call.ArgsJSON)
					}
					q.logf("🐝 [%s] Executando a ferramenta: %s", spec.Name, call.Name)
					res, err := tool.Execute(ctx, call.ArgsJSON)
					if err != nil {
						toolResult = fmt.Sprintf("Erro ao executar: %v", err)
					} else {
						toolResult = res
					}
				}
			}

			messages = append(messages, brain.Message{
				Role: brain.RoleTool, ToolCallID: call.ID, Content: toolResult,
			})
		}
	}

	return "", fmt.Errorf("limite de reflexões atingido pela especialista %s", spec.Name)
}

// DispatchGoal receives a high-level goal and assigns it to the workers via the queue.
func (q *Queen) DispatchGoal(ctx context.Context, groupID string, goal string) error {
	q.mu.RLock()
	policy, exists := q.Policies[groupID]
	q.mu.RUnlock()

	if !exists {
		return fmt.Errorf("swarm group %s not registered", groupID)
	}

	job := queue.Job{
		ID: fmt.Sprintf("goal-%s", groupID), GroupID: groupID, MaxRetries: 1,
		Task: func(ctx context.Context) error { return q.executeGoal(ctx, groupID, goal, policy) },
	}
	q.Queue.Submit(ctx, job)
	return nil
}

// executeGoal is the internal logic where the Queen manages the worker's flight.
func (q *Queen) executeGoal(ctx context.Context, groupID string, goal string, p Policy) error {
	fmt.Printf("[Queen] Inciando análise autónoma para o grupo %s\n", groupID)

	messages := []brain.Message{
		{Role: brain.RoleSystem, Content: "És a operária da colmeia Jandaira. Deves resolver o objetivo usando as ferramentas fornecidas. Analisa os ficheiros passo a passo."},
		{Role: brain.RoleUser, Content: goal},
	}

	// Converter o mapa de ferramentas para a estrutura do Brain
	var availableTools []brain.ToolDefinition
	for _, t := range q.Tools {
		availableTools = append(availableTools, brain.ToolDefinition{
			Name: t.Name(), Description: t.Description(), Parameters: t.Parameters(),
		})
	}

	// Loop de Agente: Continuar enquanto a IA quiser usar ferramentas
	for i := 0; i < 5; i++ { // Limite de 5 passos para evitar loops infinitos
		response, toolCalls, report, err := q.Brain.Chat(ctx, messages, availableTools)
		if err != nil {
			return err
		}

		q.mu.Lock()
		q.NectarUsage[groupID] += report.TotalTokens
		q.mu.Unlock()

		// Se a IA respondeu com texto e NÃO pediu ferramentas, a missão está cumprida.
		if len(toolCalls) == 0 {
			fmt.Printf("\n[Queen] Relatório Final:\n%s\n", response)
			return nil
		}

		// Adicionar o pensamento da IA ao histórico (importante para manter o contexto)
		messages = append(messages, brain.Message{
			Role: brain.RoleAssistant, Content: response, ToolCalls: toolCalls,
		})

		// A IA decidiu usar ferramentas! Vamos executá-las.
		for _, call := range toolCalls {
			fmt.Printf("🐝 [Operária] Acionando ferramenta: %s\n", call.Name)

			tool, exists := q.Tools[call.Name]
			var toolResult string

			if !exists {
				toolResult = "Erro: Ferramenta desconhecida."
			} else {
				// Aqui executariamos a ferramenta (Idealmente dentro do sandbox Wasm!)
				res, err := tool.Execute(ctx, call.ArgsJSON)
				if err != nil {
					toolResult = fmt.Sprintf("Erro ao executar: %v", err)
				} else {
					toolResult = res
				}
			}

			fmt.Printf("📦 [Resultado] Ferramenta '%s' devolveu dados ao cérebro.\n", call.Name)

			// Enviar o resultado da ferramenta de volta para a IA analisar
			messages = append(messages, brain.Message{
				Role:       brain.RoleTool,
				ToolCallID: call.ID,
				Content:    toolResult,
			})
		}
	}

	return fmt.Errorf("Limite de reflexões atingido. O enxame ficou confuso.")
}

// AskPermission implements the Human-in-the-loop (HIL) check.
func (q *Queen) AskPermission(action string) bool {
	// In a real scenario, this would trigger a message to the Web UI or CLI.
	fmt.Printf("[HIL] Queen is requesting approval for action: %s\n", action)
	// For now, we auto-approve for testing, but this is where the HIL logic lives.
	return true
}
