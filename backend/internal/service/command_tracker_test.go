package service

import (
	"context"
	"testing"
	"time"
)

func TestCommandTrackerRegisterAndGet(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.Register("req_123", "agent_1", "prepare_release_workspace")

	cmd, err := tracker.Get("req_123")
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if cmd.State != CommandStatePending {
		t.Fatalf("expected state pending, got %s", cmd.State)
	}
	if cmd.AgentID != "agent_1" {
		t.Fatalf("expected agent agent_1, got %s", cmd.AgentID)
	}
	if cmd.CommandType != "prepare_release_workspace" {
		t.Fatalf("expected command type prepare_release_workspace, got %s", cmd.CommandType)
	}
}

func TestCommandTrackerUpdateState(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.Register("req_123", "agent_1", "reconcile_revision")

	err := tracker.UpdateState("req_123", CommandStateRunning, nil, "")
	if err != nil {
		t.Fatalf("update state: %v", err)
	}

	cmd, err := tracker.Get("req_123")
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if cmd.State != CommandStateRunning {
		t.Fatalf("expected state running, got %s", cmd.State)
	}
}

func TestCommandTrackerWaitForResultDone(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.Register("req_123", "agent_1", "run_health_gate")

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = tracker.UpdateState("req_123", CommandStateDone, map[string]any{"summary": "healthy"}, "")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd, err := tracker.WaitForResult(ctx, "req_123")
	if err != nil {
		t.Fatalf("wait for result: %v", err)
	}
	if cmd.State != CommandStateDone {
		t.Fatalf("expected state done, got %s", cmd.State)
	}
	if cmd.Output["summary"] != "healthy" {
		t.Fatalf("expected summary healthy, got %v", cmd.Output["summary"])
	}
}

func TestCommandTrackerWaitForResultFailed(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.Register("req_456", "agent_2", "start_release_candidate")

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = tracker.UpdateState("req_456", CommandStateFailed, nil, "workload failed to start")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd, err := tracker.WaitForResult(ctx, "req_456")
	if err != nil {
		t.Fatalf("wait for result: %v", err)
	}
	if cmd.State != CommandStateFailed {
		t.Fatalf("expected state failed, got %s", cmd.State)
	}
	if cmd.Error != "workload failed to start" {
		t.Fatalf("expected error message, got %s", cmd.Error)
	}
}

func TestCommandTrackerWaitForResultTimeout(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.Register("req_789", "agent_3", "promote_release")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cmd, err := tracker.WaitForResult(ctx, "req_789")
	if err != nil {
		t.Fatalf("wait for result: %v", err)
	}
	if cmd.State != CommandStateCancelled {
		t.Fatalf("expected state cancelled, got %s", cmd.State)
	}
}

func TestCommandTrackerWaitForResultUntracked(t *testing.T) {
	tracker := NewCommandTracker()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := tracker.WaitForResult(ctx, "req_nonexistent")
	if err == nil {
		t.Fatal("expected error for untracked command")
	}
}

func TestCommandTrackerCleanup(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.Register("req_123", "agent_1", "prepare_release_workspace")

	tracker.Cleanup("req_123")

	_, err := tracker.Get("req_123")
	if err == nil {
		t.Fatal("expected error after cleanup")
	}
}

func TestCommandTrackerListByAgent(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.Register("req_1", "agent_1", "prepare_release_workspace")
	tracker.Register("req_2", "agent_1", "reconcile_revision")
	tracker.Register("req_3", "agent_2", "run_health_gate")

	results := tracker.ListByAgent("agent_1")
	if len(results) != 2 {
		t.Fatalf("expected 2 commands for agent_1, got %d", len(results))
	}
}
