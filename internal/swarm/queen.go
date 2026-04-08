package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/prompt"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/tool"
)

type Policy struct {
	MaxNectar        int
	Isolate          bool
	RequiresApproval bool
}

type Specialist struct {
	Name         string   `json:"Name"`
	SystemPrompt string   `json:"SystemPrompt"`
	AllowedTools []string `json:"AllowedTools"`
}

type SwarmPlan struct {
	Specialists []Specialist `json:"specialists"`
}

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

func NewQueen(q *queue.GroupQueue, b brain.Brain, h brain.Honeycomb) *Queen {
	return &Queen{
		Queue:        q,
		Brain:        b,
		Honeycomb:    h,
		Policies:     make(map[string]Policy),
		NectarUsage:  make(map[string]int),
		Tools:        make(map[string]tool.Tool),
		ApprovalChan: make(chan bool, 1),
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

func (q *Queen) EquipTool(t tool.Tool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Tools[t.Name()] = t
}

func (q *Queen) RegisterSwarm(groupID string, p Policy) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Policies[groupID] = p
	q.NectarUsage[groupID] = 0
}

func (q *Queen) AssembleSwarm(ctx context.Context, goal string, maxWorkers int) ([]Specialist, error) {
	q.logf("🧠 The Queen is consulting the manuals and designing the swarm architecture...")

	var availableToolsDesc []string
	for name, t := range q.Tools {
		availableToolsDesc = append(availableToolsDesc, fmt.Sprintf("- '%s': %s", name, t.Description()))
	}
	toolsListStr := strings.Join(availableToolsDesc, "\n")

	systemPrompt := fmt.Sprintf(`%s
		======================================================================
		SWARM ARCHITECTURE INSTRUCTIONS (META-PLANNING)
		======================================================================
		Use all the Skill Creator knowledge above as your bible for creating worker bees.
		Right now, you are not talking to the user. You are acting as the Queen Orchestrator and must CREATE the ideal swarm to solve the current objective.

		TOOLS AVAILABLE IN THE HIVE FOR DELEGATION:
		%s

		OUTPUT RULES:
		1. You may create at most %d workers. Create only as many as the objective truly requires.
		2. Each worker acts in sequence (the result of one is passed to the next).
		3. Apply the Skill Writing Guide rules to the 'SystemPrompt' field of EACH worker you create.
		4. You MUST return ONLY a valid JSON. Do not include markdown blocks, greetings, or explanations outside the JSON.

		EXPECTED OUTPUT FORMAT:
		{
		"specialists": [
			{
			"Name": "Creative Worker Name",
			"SystemPrompt": "Deep, well-crafted instructions for how the worker should act, which tools to use and why...",
			"AllowedTools": ["tool_1", "tool_2"]
			}
		]
		}
  `, prompt.SkillCreatorPrompt, toolsListStr, maxWorkers)

	messages := []brain.Message{
		{Role: brain.RoleSystem, Content: systemPrompt},
		{Role: brain.RoleUser, Content: fmt.Sprintf("User objective: %s", goal)},
	}

	response, _, _, err := q.Brain.Chat(ctx, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("Queen meta-planning failed: %w", err)
	}

	cleanJSON := response
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var plan SwarmPlan
	if err := json.Unmarshal([]byte(cleanJSON), &plan); err != nil {
		return nil, fmt.Errorf("Queen generated an invalid plan (JSON error): %w\nGenerated response: %s", err, cleanJSON)
	}

	var workerNames []string
	for _, spec := range plan.Specialists {
		workerNames = append(workerNames, spec.Name)
	}
	q.logf("👑 Swarm planned with %d super-trained bees: %s", len(plan.Specialists), strings.Join(workerNames, " -> "))

	return plan.Specialists, nil
}

func (q *Queen) DispatchWorkflow(ctx context.Context, groupID string, goal string, pipeline []Specialist) (<-chan string, <-chan error) {
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	q.mu.RLock()
	policy, exists := q.Policies[groupID]
	q.mu.RUnlock()

	if !exists {
		errChan <- fmt.Errorf("swarm group '%s' is not registered in the hive", groupID)
		return resultChan, errChan
	}

	job := queue.Job{
		ID: fmt.Sprintf("workflow-%s-%d", groupID, time.Now().Unix()), GroupID: groupID, MaxRetries: 1,
		Task: func(ctx context.Context) error {
			contextAccumulator := fmt.Sprintf("Original Goal: %s\n\n", goal)

			sessionKey, err := security.GenerateKey()
			if err != nil {
				errChan <- fmt.Errorf("failed to generate cryptographic key: %w", err)
				return err
			}

			for _, specialist := range pipeline {
				if q.AgentChangeFunc != nil {
					q.AgentChangeFunc(specialist.Name)
				}
				q.logf("👑 [Queen] Passing the baton to: %s", specialist.Name)

				encryptedPayload, err := security.Seal(sessionKey, contextAccumulator)
				if err != nil {
					errChan <- fmt.Errorf("failed to encrypt payload: %w", err)
					return err
				}

				encryptedOutput, err := q.runSpecialist(ctx, specialist, encryptedPayload, sessionKey, policy)
				if err != nil {
					errChan <- fmt.Errorf("worker '%s' failed: %w", specialist.Name, err)
					return err
				}

				decryptedOutput, err := security.Open(sessionKey, encryptedOutput)
				if err != nil {
					errChan <- fmt.Errorf("failed to decrypt response from '%s': %w", specialist.Name, err)
					return err
				}

				contextAccumulator += fmt.Sprintf("\n--- Report from %s ---\n%s\n", specialist.Name, decryptedOutput)
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

			resultChan <- contextAccumulator
			return nil
		},
	}

	q.Queue.Submit(ctx, job)
	return resultChan, errChan
}

func (q *Queen) runSpecialist(ctx context.Context, spec Specialist, encryptedTaskContext string, sessionKey []byte, p Policy) (string, error) {
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
		q.NectarUsage[spec.Name] += report.TotalTokens
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
			t, exists := q.Tools[call.Name]
			var toolResult string

			if !exists {
				toolResult = "Error: tool not found."
			} else {
				approved := true
				if p.RequiresApproval {
					if q.AskPermissionFunc != nil {
						q.AskPermissionFunc(call.Name, call.ArgsJSON)
						approved = <-q.ApprovalChan
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
					res, err := t.Execute(ctx, call.ArgsJSON)
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

	return "", fmt.Errorf("reflection limit reached for specialist '%s'", spec.Name)
}
