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
	Graph             brain.KnowledgeGraph // optional; nil = disabled
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

func (q *Queen) IsSwarmRegistered(groupID string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	_, ok := q.Policies[groupID]
	return ok
}

func (q *Queen) AssembleSwarm(ctx context.Context, goal string, maxWorkers int) ([]Specialist, error) {
	q.logf("🧠 The Queen is consulting the manuals and designing the swarm architecture...")

	var availableToolsDesc []string
	for name, t := range q.Tools {
		availableToolsDesc = append(availableToolsDesc, fmt.Sprintf("- '%s': %s", name, t.Description()))
	}
	toolsListStr := strings.Join(availableToolsDesc, "\n")

	graphCtx := q.graphContextForGoal(ctx, goal)

	systemPrompt := fmt.Sprintf(`%s
		%s
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
  `, prompt.SkillCreatorPrompt, graphCtx, toolsListStr, maxWorkers)

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
	cleanJSON = sanitizeJSONEscapes(cleanJSON)

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

			// Pre-seed context with relevant past memories so specialists have
			// historical context even without calling search_memory explicitly.
			if q.Honeycomb != nil && q.Brain != nil {
				if vec, err := q.Brain.Embed(ctx, goal); err == nil {
					if memories, err := q.Honeycomb.Search(ctx, groupID, vec, 5); err == nil && len(memories) > 0 {
						contextAccumulator += "--- RELEVANT PAST CONTEXT (from memory) ---\n"
						for _, m := range memories {
							if content, ok := m.Metadata["content"]; ok && content != "" {
								contextAccumulator += content + "\n---\n"
							}
						}
						contextAccumulator += "\n"
					}
				}
			}

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

			q.registerWorkflowInGraph(ctx, goal, pipeline)

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

	for i := 0; i < 5; i++ {
		response, toolCalls, report, err := q.Brain.Chat(ctx, messages, availableTools)
		if err != nil {
			q.logf("❌ [%s] Brain.Chat failed (iteration %d): %v", spec.Name, i, err)
			return "", fmt.Errorf("brain error on iteration %d: %w", i, err)
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

	// Force a final summary from the specialist instead of failing the job.
	q.logf("⚠️  [%s] Reflection limit reached — requesting final summary.", spec.Name)
	// Keep only system + first user message to avoid context overflow.
	trimmedMessages := messages[:2]
	trimmedMessages = append(trimmedMessages, brain.Message{
		Role:    brain.RoleUser,
		Content: "REFLECTION LIMIT REACHED. Stop using tools. Report what was attempted and what failed.",
	})
	finalResponse, _, _, err := q.Brain.Chat(ctx, trimmedMessages, nil)
	if err != nil {
		q.logf("❌ [%s] Reflection limit summary also failed: %v", spec.Name, err)
		return "", fmt.Errorf("specialist '%s' reflection limit summary failed: %w", spec.Name, err)
	}
	encryptedFinal, err := security.Seal(sessionKey, "[REFLECTION LIMIT] "+finalResponse)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt reflection-limit response: %w", err)
	}
	return encryptedFinal, nil
}

// registerWorkflowInGraph records specialists and their topic area in the
// knowledge graph so the Queen can reference them in future swarm assemblies.
func (q *Queen) registerWorkflowInGraph(ctx context.Context, goal string, specialists []Specialist) {
	if q.Graph == nil {
		return
	}

	label := goal
	if len(label) > 80 {
		label = label[:80]
	}
	topicID := "topic-" + slugify(label)

	_ = q.Graph.AddNode(ctx, brain.Node{
		ID:    topicID,
		Type:  "topic",
		Label: label,
	})

	for _, spec := range specialists {
		agentID := "agent-" + slugify(spec.Name)
		preview := spec.SystemPrompt
		if len(preview) > 200 {
			preview = preview[:200]
		}
		_ = q.Graph.AddNode(ctx, brain.Node{
			ID:    agentID,
			Type:  "agent",
			Label: spec.Name,
			Props: map[string]string{"system_prompt_preview": preview},
		})
		_ = q.Graph.AddEdge(ctx, brain.Edge{
			From:   agentID,
			To:     topicID,
			Rel:    "expert_in",
			Weight: 1.0,
		})
	}
}

// graphContextForGoal queries the knowledge graph for past specialists whose
// topics overlap with the current goal and returns a prompt snippet the Queen
// can use as reference when designing a new swarm.
func (q *Queen) graphContextForGoal(ctx context.Context, goal string) string {
	if q.Graph == nil {
		return ""
	}

	expertSet := map[string]brain.Node{}
	for _, word := range strings.Fields(goal) {
		if len(word) < 5 {
			continue
		}
		experts, err := q.Graph.FindExperts(ctx, word)
		if err != nil {
			continue
		}
		for _, e := range experts {
			expertSet[e.ID] = e
		}
	}

	if len(expertSet) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("PAST SPECIALIST KNOWLEDGE (from knowledge graph — use as reference only):\n")
	for _, node := range expertSet {
		preview := node.Props["system_prompt_preview"]
		sb.WriteString(fmt.Sprintf("- %s: %s\n", node.Label, preview))
	}
	sb.WriteString("\n")
	return sb.String()
}

// sanitizeJSONEscapes replaces invalid JSON escape sequences (e.g. \( \$ \!)
// with their literal characters so json.Unmarshal doesn't reject LLM output.
func sanitizeJSONEscapes(s string) string {
	valid := map[byte]bool{'"': true, '\\': true, '/': true, 'b': true, 'f': true, 'n': true, 'r': true, 't': true, 'u': true}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && !valid[s[i+1]] {
			// Skip the backslash — emit only the following character.
			i++
			b.WriteByte(s[i])
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// slugify converts a string into a lowercase, hyphen-separated identifier
// safe for use as a graph node ID.
func slugify(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	return sb.String()
}
