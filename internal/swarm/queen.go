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

// Specialist represents a bee with a specific role and a restricted set of tools.
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

// EquipTool registers a new tool with the Queen.
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

// DispatchWorkflow runs a chain of specialists in sequence (baton-pass pipeline).
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
			contextAccumulator := fmt.Sprintf("Original Goal: %s\n\n", goal)

			sessionKey, err := security.GenerateKey()
			if err != nil {
				errChan <- fmt.Errorf("failed to generate cryptographic key: %w", err)
				return err
			}

			for _, specialist := range pipeline {
				q.logf("👑 [Queen] Delegando a fase para: %s", specialist.Name)
				if q.AgentChangeFunc != nil {
					q.AgentChangeFunc(specialist.Name)
				}

				encryptedPayload, err := security.Seal(sessionKey, contextAccumulator)
				if err != nil {
					errChan <- fmt.Errorf("failed to encrypt payload: %w", err)
					return err
				}
				q.logf("🔐 Encrypted payload (%d bytes) sent to %s", len(encryptedPayload), specialist.Name)

				encryptedOutput, err := q.runSpecialist(ctx, groupID, specialist, encryptedPayload, sessionKey, policy)
				if err != nil {
					errChan <- fmt.Errorf("specialist %s failed: %w", specialist.Name, err)
					return err
				}

				// Decrypt the response received from the Specialist
				decryptedOutput, err := security.Open(sessionKey, encryptedOutput)
				if err != nil {
					errChan <- fmt.Errorf("failed to decrypt response from %s: %w", specialist.Name, err)
					return err
				}
				q.logf("🔓 Decrypted response successfully received from %s", specialist.Name)

				contextAccumulator += fmt.Sprintf("\n--- Trabalho de %s ---\n%s\n", specialist.Name, decryptedOutput)
			}

			q.logf("👑 [Queen] Workflow complete! The swarm has finished.")

			if q.Honeycomb != nil {
				q.logf("💾 Saving knowledge to vector memory...")
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

			// Send the final report to the UI
			resultChan <- contextAccumulator
			return nil
		},
	}

	q.Queue.Submit(ctx, job)
	return resultChan, errChan
}

// runSpecialist runs the agent loop for a specific specialist with its restricted tools.
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
		return "", fmt.Errorf("security failure: cannot decrypt context: %w", err)
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
			q.logf("✅ [%s] Task complete.", spec.Name)

			encryptedFinalResponse, err := security.Seal(sessionKey, response)
			if err != nil {
				return "", fmt.Errorf("failed to encrypt final response: %w", err)
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
				toolResult = "Error: tool not found."
			} else {
				approved := true
				if p.RequiresApproval {
					if q.AskPermissionFunc != nil {
						q.AskPermissionFunc(call.Name, call.ArgsJSON)
						approved = <-q.ApprovalChan // blocks until the UI responds
					}
				}

				if !approved {
					q.logf("🚫 Action blocked by the Beekeeper: %s", call.Name)
					toolResult = fmt.Sprintf("ERROR: The Beekeeper (human user) DENIED permission to execute '%s'. Abort this attempt.", call.Name)
				} else {
					if q.ToolStartFunc != nil {
						q.ToolStartFunc(spec.Name, call.Name, call.ArgsJSON)
					}
					q.logf("🐝 [%s] Executing tool: %s", spec.Name, call.Name)
					res, err := tool.Execute(ctx, call.ArgsJSON)
					if err != nil {
						toolResult = fmt.Sprintf("Error executing tool: %v", err)
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

	return "", fmt.Errorf("reflection limit reached for specialist %s", spec.Name)
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
	fmt.Printf("[Queen] Starting autonomous analysis for group %s\n", groupID)

	messages := []brain.Message{
		{Role: brain.RoleSystem, Content: "You are a worker of the Jandaira hive. Solve the goal using the provided tools. Analyse files step by step."},
		{Role: brain.RoleUser, Content: goal},
	}

	// Convert the tool map to the Brain's ToolDefinition slice
	var availableTools []brain.ToolDefinition
	for _, t := range q.Tools {
		availableTools = append(availableTools, brain.ToolDefinition{
			Name: t.Name(), Description: t.Description(), Parameters: t.Parameters(),
		})
	}

	// Agent loop: keep running while the AI wants to use tools
	for i := 0; i < 5; i++ { // max 5 steps to prevent infinite loops
		response, toolCalls, report, err := q.Brain.Chat(ctx, messages, availableTools)
		if err != nil {
			return err
		}

		q.mu.Lock()
		q.NectarUsage[groupID] += report.TotalTokens
		q.mu.Unlock()

		// If the AI replied with text and did NOT request tools, the mission is done.
		if len(toolCalls) == 0 {
			fmt.Printf("\n[Queen] Final Report:\n%s\n", response)
			return nil
		}

		// Append the AI's reasoning to the history (required to preserve context)
		messages = append(messages, brain.Message{
			Role: brain.RoleAssistant, Content: response, ToolCalls: toolCalls,
		})

		// The AI decided to use tools — execute them.
		for _, call := range toolCalls {
			fmt.Printf("🐝 [Worker] Invoking tool: %s\n", call.Name)

			tool, exists := q.Tools[call.Name]
			var toolResult string

			if !exists {
				toolResult = "Error: unknown tool."
			} else {
				// Execute the tool (ideally inside a Wasm sandbox)
				res, err := tool.Execute(ctx, call.ArgsJSON)
				if err != nil {
					toolResult = fmt.Sprintf("Error executing tool: %v", err)
				} else {
					toolResult = res
				}
			}

			fmt.Printf("📦 [Result] Tool '%s' returned data to the brain.\n", call.Name)

			// Send the tool result back to the AI for analysis
			messages = append(messages, brain.Message{
				Role:       brain.RoleTool,
				ToolCallID: call.ID,
				Content:    toolResult,
			})
		}
	}

	return fmt.Errorf("reflection limit reached. The swarm got confused.")
}

// AskPermission implements the Human-in-the-loop (HIL) check.
func (q *Queen) AskPermission(action string) bool {
	// In a real scenario, this would trigger a message to the Web UI or CLI.
	fmt.Printf("[HIL] Queen is requesting approval for action: %s\n", action)
	// For now, we auto-approve for testing, but this is where the HIL logic lives.
	return true
}
