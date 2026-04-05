package swarm

import (
	"context"
	"fmt"
	"sync"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/tools"
)

// Policy defines the safety rules and resource limits for a swarm.
type Policy struct {
	MaxNectar        int  // Maximum token/cost budget
	Isolate          bool // Whether to use a strict Wasm sandbox
	RequiresApproval bool // If human-in-the-loop is mandatory for sensitive tools
}

// Queen is the central orchestrator of the hive.
type Queen struct {
	Queue       *queue.GroupQueue
	Brain       brain.Brain // The AI brain integrated into the Queen
	mu          sync.RWMutex
	Policies    map[string]Policy // Policies indexed by GroupID
	NectarUsage map[string]int    // Current consumption per group
	Tools       map[string]tools.Tool
}

// NewQueen initializes the hive's sovereign.
func NewQueen(q *queue.GroupQueue, b brain.Brain) *Queen {
	return &Queen{
		Queue:       q,
		Brain:       b,
		Policies:    make(map[string]Policy),
		NectarUsage: make(map[string]int),
		Tools:       make(map[string]tools.Tool),
	}
}

// EquipTool dá à Rainha o conhecimento de uma nova ferramenta
func (q *Queen) EquipTool(t tools.Tool) {
	q.Tools[t.Name()] = t
}

// RegisterSwarm sets up the rules for a specific group of agents.
func (q *Queen) RegisterSwarm(groupID string, p Policy) { /* Mantém-se */
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Policies[groupID] = p
	q.NectarUsage[groupID] = 0
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
