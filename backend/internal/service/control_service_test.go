package service

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type mockControlConn struct {
	mu      sync.Mutex
	written [][]byte
	readMsg []byte
	readIdx int
	closed  bool
}

func (m *mockControlConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.written = append(m.written, data)
	return nil
}

func (m *mockControlConn) ReadMessage() (messageType int, data []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readIdx >= len(m.readMsg) {
		return 0, nil, nil
	}
	data = m.readMsg[m.readIdx:]
	m.readIdx = len(m.readMsg)
	return 1, data, nil
}

func (m *mockControlConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockControlConn) lastWritten() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.written) == 0 {
		return nil
	}
	return m.written[len(m.written)-1]
}

func TestControlHubRegisterAndUnregister(t *testing.T) {
	hub := NewControlHub()
	hub.Start()

	conn := &mockControlConn{}
	client := &ControlClient{
		AgentID:    "agent_123",
		InstanceID: "inst_123",
		Conn:       conn,
		Registered: time.Now().UTC(),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	if !hub.IsConnected("agent_123") {
		t.Fatal("expected agent to be connected")
	}

	hub.Unregister("agent_123")
	time.Sleep(10 * time.Millisecond)

	if hub.IsConnected("agent_123") {
		t.Fatal("expected agent to be disconnected after unregister")
	}
}

func TestControlHubSendToAgent(t *testing.T) {
	hub := NewControlHub()
	hub.Start()

	conn := &mockControlConn{}
	client := &ControlClient{
		AgentID:    "agent_123",
		InstanceID: "inst_123",
		Conn:       conn,
		Registered: time.Now().UTC(),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	payload := map[string]any{"type": "reconcile_revision", "project_id": "prj_123"}
	if err := hub.SendToAgent("agent_123", payload); err != nil {
		t.Fatalf("send to agent: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	written := conn.lastWritten()
	if written == nil {
		t.Fatal("expected message to be written")
	}

	var decoded map[string]any
	if err := json.Unmarshal(written, &decoded); err != nil {
		t.Fatalf("decode written message: %v", err)
	}
	if decoded["type"] != "reconcile_revision" {
		t.Fatalf("expected type reconcile_revision, got %v", decoded["type"])
	}
}

func TestControlHubSendToDisconnectedAgent(t *testing.T) {
	hub := NewControlHub()
	hub.Start()

	err := hub.SendToAgent("agent_missing", map[string]any{"type": "test"})
	if err == nil {
		t.Fatal("expected error for disconnected agent")
	}
}

func TestOperatorStreamHubRegisterAndUnregister(t *testing.T) {
	hub := NewOperatorStreamHub()
	hub.Start()

	client := &OperatorClient{
		UserID: "usr_123",
		Send:   make(chan []byte, 16),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	select {
	case _, ok := <-client.Send:
		if ok {
			t.Fatal("expected client send channel to be closed")
		}
	default:
		t.Fatal("expected client send channel to be closed")
	}
}

func TestOperatorStreamHubBroadcastEvent(t *testing.T) {
	hub := NewOperatorStreamHub()
	hub.Start()

	client := &OperatorClient{
		UserID: "usr_123",
		Send:   make(chan []byte, 16),
	}
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	payload := map[string]any{"deployment_id": "dep_123", "status": "started"}
	if err := hub.BroadcastEvent("deployment.started", payload); err != nil {
		t.Fatalf("broadcast event: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	select {
	case data := <-client.Send:
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if decoded["type"] != "deployment.started" {
			t.Fatalf("expected type deployment.started, got %v", decoded["type"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestOperatorStreamHubBroadcastToSpecificUser(t *testing.T) {
	hub := NewOperatorStreamHub()
	hub.Start()

	clientA := &OperatorClient{
		UserID: "usr_a",
		Send:   make(chan []byte, 16),
	}
	clientB := &OperatorClient{
		UserID: "usr_b",
		Send:   make(chan []byte, 16),
	}
	hub.Register(clientA)
	hub.Register(clientB)
	time.Sleep(10 * time.Millisecond)

	payload := map[string]any{"incident_id": "inc_123"}
	if err := hub.BroadcastEventToUser("usr_a", "incident.created", payload); err != nil {
		t.Fatalf("broadcast to user: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	select {
	case data := <-clientA.Send:
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decode event for user A: %v", err)
		}
		if decoded["type"] != "incident.created" {
			t.Fatalf("expected type incident.created for user A, got %v", decoded["type"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event for user A")
	}

	select {
	case data := <-clientB.Send:
		t.Fatalf("user B should not receive event targeted at user A, got: %s", string(data))
	default:
	}
}

func TestSerializeOperatorEvent(t *testing.T) {
	data, err := serializeOperatorEvent("deployment.promoted", map[string]any{"deployment_id": "dep_123"})
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["type"] != "deployment.promoted" {
		t.Fatalf("expected type deployment.promoted, got %v", decoded["type"])
	}
	if decoded["occurred_at"] == nil {
		t.Fatal("expected occurred_at to be set")
	}
}
