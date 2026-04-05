package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type CommandState string

const (
	CommandStatePending   CommandState = "pending"
	CommandStateRunning   CommandState = "running"
	CommandStateDone      CommandState = "done"
	CommandStateFailed    CommandState = "failed"
	CommandStateCancelled CommandState = "cancelled"
)

type TrackedCommand struct {
	RequestID   string         `json:"request_id"`
	AgentID     string         `json:"agent_id"`
	CommandType string         `json:"command_type"`
	State       CommandState   `json:"state"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	SentAt      time.Time      `json:"sent_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CommandTracker struct {
	mu       sync.RWMutex
	commands map[string]*TrackedCommand
	ready    map[string]chan struct{}
}

func NewCommandTracker() *CommandTracker {
	return &CommandTracker{
		commands: make(map[string]*TrackedCommand),
		ready:    make(map[string]chan struct{}),
	}
}

func (t *CommandTracker) Register(requestID, agentID, commandType string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC()
	t.commands[requestID] = &TrackedCommand{
		RequestID:   requestID,
		AgentID:     agentID,
		CommandType: commandType,
		State:       CommandStatePending,
		SentAt:      now,
		UpdatedAt:   now,
	}
	t.ready[requestID] = make(chan struct{})
}

func (t *CommandTracker) UpdateState(requestID string, state CommandState, output map[string]any, errMsg string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	cmd, ok := t.commands[requestID]
	if !ok {
		return fmt.Errorf("command %q not found", requestID)
	}

	cmd.State = state
	cmd.Output = output
	cmd.Error = errMsg
	cmd.UpdatedAt = time.Now().UTC()

	if ch, exists := t.ready[requestID]; exists && (state == CommandStateDone || state == CommandStateFailed) {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}

	return nil
}

func (t *CommandTracker) Get(requestID string) (*TrackedCommand, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cmd, ok := t.commands[requestID]
	if !ok {
		return nil, fmt.Errorf("command %q not found", requestID)
	}

	cloned := *cmd
	return &cloned, nil
}

func (t *CommandTracker) WaitForResult(ctx context.Context, requestID string) (*TrackedCommand, error) {
	t.mu.RLock()
	ch, ok := t.ready[requestID]
	if !ok {
		t.mu.RUnlock()
		return nil, fmt.Errorf("command %q not tracked", requestID)
	}
	t.mu.RUnlock()

	select {
	case <-ch:
		return t.Get(requestID)
	case <-ctx.Done():
		_ = t.UpdateState(requestID, CommandStateCancelled, nil, "context cancelled or timed out")
		return t.Get(requestID)
	}
}

func (t *CommandTracker) Cleanup(requestID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.commands, requestID)
	delete(t.ready, requestID)
}

func (t *CommandTracker) ListByAgent(agentID string) []*TrackedCommand {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var results []*TrackedCommand
	for _, cmd := range t.commands {
		if cmd.AgentID == agentID {
			cloned := *cmd
			results = append(results, &cloned)
		}
	}
	return results
}
