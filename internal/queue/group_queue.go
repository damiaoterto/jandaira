package queue

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Task represents a unit of work for a worker bee.
type Task func(ctx context.Context) error

// Job wraps the task with control metadata.
type Job struct {
	ID         string
	GroupID    string
	Task       Task
	Retries    int
	MaxRetries int
}

// GroupQueue manages concurrent execution of tasks segmented by groups.
type GroupQueue struct {
	maxConcurrentPerGroup int
	mu                    sync.RWMutex
	groups                map[string]chan struct{} // Semaphores per group
	activeTasks           sync.WaitGroup
}

// NewGroupQueue creates a new queue with a concurrency limit per group.
func NewGroupQueue(maxConcurrent int) *GroupQueue {
	return &GroupQueue{
		maxConcurrentPerGroup: maxConcurrent,
		groups:                make(map[string]chan struct{}),
	}
}

// getSemaphore ensures each group has its own concurrency control channel.
func (g *GroupQueue) getSemaphore(groupID string) chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	if sem, ok := g.groups[groupID]; ok {
		return sem
	}

	sem := make(chan struct{}, g.maxConcurrentPerGroup)
	g.groups[groupID] = sem
	return sem
}

// Submit adds a task for execution respecting FIFO order and group limits.
func (g *GroupQueue) Submit(ctx context.Context, job Job) {
	g.activeTasks.Add(1)

	go func() {
		defer g.activeTasks.Done()

		sem := g.getSemaphore(job.GroupID)

		// Attempt to acquire a slot in the "honeycomb" (group concurrency)
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }() // Release the slot when finished
			g.executeWithRetry(ctx, job)
		case <-ctx.Done():
			fmt.Printf("Job %s canceled before starting due to context\n", job.ID)
		}
	}()
}

// executeWithRetry handles task execution and exponential backoff logic.
func (g *GroupQueue) executeWithRetry(ctx context.Context, job Job) {
	for i := 0; i <= job.MaxRetries; i++ {
		err := job.Task(ctx)
		if err == nil {
			return // Success
		}

		job.Retries = i
		if i == job.MaxRetries {
			fmt.Printf("Job %s failed after %d attempts: %v\n", job.ID, i, err)
			return
		}

		// Exponential Backoff calculation: 2^attempt * 500ms
		waitTime := time.Duration(math.Pow(2, float64(i))) * 500 * time.Millisecond
		fmt.Printf("Job %s failed (attempt %d). Retrying in %v...\n", job.ID, i+1, waitTime)

		select {
		case <-time.After(waitTime):
			// Continue to next attempt
		case <-ctx.Done():
			return
		}
	}
}

// Wait waits for the completion of all submitted tasks.
func (g *GroupQueue) Wait() {
	g.activeTasks.Wait()
}