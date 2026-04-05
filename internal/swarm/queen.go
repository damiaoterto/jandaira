package swarm

import (
	"context"
	"fmt"
	"sync"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/queue"
	"github.com/damiaoterto/jandaira/internal/security"
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
}

// NewQueen initializes the hive's sovereign.
func NewQueen(q *queue.GroupQueue, b brain.Brain) *Queen {
	return &Queen{
		Queue:       q,
		Brain:       b,
		Policies:    make(map[string]Policy),
		NectarUsage: make(map[string]int),
	}
}

// RegisterSwarm sets up the rules for a specific group of agents.
func (q *Queen) RegisterSwarm(groupID string, p Policy) {
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

	// Create a Job to be processed by the GroupQueue
	job := queue.Job{
		ID:         fmt.Sprintf("goal-%s", groupID),
		GroupID:    groupID,
		MaxRetries: 3,
		Task: func(ctx context.Context) error {
			return q.executeGoal(ctx, groupID, goal, policy)
		},
	}

	q.Queue.Submit(ctx, job)
	return nil
}

// executeGoal is the internal logic where the Queen manages the worker's flight.
func (q *Queen) executeGoal(ctx context.Context, groupID string, goal string, p Policy) error {
	fmt.Printf("[Queen] Orchestrating goal for %s using %s: %s\n", groupID, q.Brain.GetProviderName(), goal)

	// 1. AI Reasoning: The Queen thinks about how to achieve the goal
	messages := []brain.Message{
		{Role: brain.RoleSystem, Content: "You are the Queen of the Jandaira hive. Orchestrate the agents to achieve the user's goal."},
		{Role: brain.RoleUser, Content: goal},
	}

	response, report, err := q.Brain.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("brain failure: %w", err)
	}

	// 2. Track Nectar usage based on actual LLM report
	q.mu.Lock()
	q.NectarUsage[groupID] += report.TotalTokens
	currentUsage := q.NectarUsage[groupID]
	q.mu.Unlock()

	if currentUsage > p.MaxNectar {
		return fmt.Errorf("nectar limit exceeded for group %s (used %d/%d)", groupID, currentUsage, p.MaxNectar)
	}

	// 3. Initialize Sandbox if isolation is required for execution
	if p.Isolate {
		cell, err := security.NewCell(ctx)
		if err != nil {
			return fmt.Errorf("failed to create sandbox cell: %w", err)
		}
		defer cell.Close(ctx)

		fmt.Printf("[Queen] Isolated cell created for worker logic execution.\n")
		// Logic to run the worker code based on the AI response would go here
	}

	fmt.Printf("[Queen] Goal processed. Response: %s\n", response)
	return nil
}

// AskPermission implements the Human-in-the-loop (HIL) check.
func (q *Queen) AskPermission(action string) bool {
	// In a real scenario, this would trigger a message to the Web UI or CLI.
	fmt.Printf("[HIL] Queen is requesting approval for action: %s\n", action)
	// For now, we auto-approve for testing, but this is where the HIL logic lives.
	return true
}
