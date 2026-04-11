package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"lazyops-server/internal/runtime"
	"lazyops-server/pkg/logger"
	"lazyops-server/pkg/utils"
)

type ControlClient struct {
	AgentID    string
	InstanceID string
	Conn       ControlConn
	Registered time.Time
}

type ControlConn interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, data []byte, err error)
	Close() error
}

type ControlHub struct {
	mu         sync.RWMutex
	clients    map[string]*ControlClient
	register   chan *ControlClient
	unregister chan string
	broadcast  chan controlBroadcast
	once       sync.Once
}

type controlBroadcast struct {
	agentID string
	payload []byte
}

func NewControlHub() *ControlHub {
	return &ControlHub{
		clients:    make(map[string]*ControlClient),
		register:   make(chan *ControlClient, 64),
		unregister: make(chan string, 64),
		broadcast:  make(chan controlBroadcast, 256),
	}
}

func (h *ControlHub) Start() {
	h.once.Do(func() {
		go h.run()
	})
}

func (h *ControlHub) Register(client *ControlClient) {
	h.register <- client
}

func (h *ControlHub) Unregister(agentID string) {
	h.unregister <- agentID
}

func (h *ControlHub) IsConnected(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[agentID]
	return ok
}

func (h *ControlHub) SendToAgent(agentID string, payload any) error {
	h.mu.RLock()
	client, ok := h.clients[agentID]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("agent %q not connected", agentID)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return client.Conn.WriteMessage(1, data)
}

func (h *ControlHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.AgentID] = client
			h.mu.Unlock()
		case agentID := <-h.unregister:
			h.mu.Lock()
			if client, ok := h.clients[agentID]; ok {
				delete(h.clients, agentID)
				_ = client.Conn.Close()
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			if msg.agentID != "" {
				if client, ok := h.clients[msg.agentID]; ok {
					_ = client.Conn.WriteMessage(1, msg.payload)
				}
			} else {
				for _, client := range h.clients {
					_ = client.Conn.WriteMessage(1, msg.payload)
				}
			}
			h.mu.RUnlock()
		}
	}
}

type ControlService struct {
	hub            *ControlHub
	commandTracker *CommandTracker
	registry       *runtime.Registry
	instances      InstanceStore
	agents         AgentStore
}

func NewControlService(hub *ControlHub, commandTracker *CommandTracker, registry *runtime.Registry, instances InstanceStore, agents AgentStore) *ControlService {
	return &ControlService{
		hub:            hub,
		commandTracker: commandTracker,
		registry:       registry,
		instances:      instances,
		agents:         agents,
	}
}

func (s *ControlService) RegisterAgent(agentID, instanceID string, conn ControlConn) {
	client := &ControlClient{
		AgentID:    agentID,
		InstanceID: instanceID,
		Conn:       conn,
		Registered: time.Now().UTC(),
	}
	s.hub.Register(client)
}

func (s *ControlService) UnregisterAgent(agentID string) {
	s.hub.Unregister(agentID)
}

func (s *ControlService) DispatchCommand(ctx context.Context, agentID string, cmd runtime.AgentCommand) (*runtime.CommandResult, error) {
	if !s.hub.IsConnected(agentID) {
		logger.Warn("control_dispatch_agent_not_connected",
			"agent_id", agentID,
			"command_type", cmd.Type,
		)
		return nil, fmt.Errorf("agent %q is not connected", agentID)
	}

	if cmd.RequestID == "" {
		cmd.RequestID = utils.NewPrefixedID("req")
	}
	if cmd.OccurredAt == "" {
		cmd.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	if s.commandTracker != nil {
		s.commandTracker.Register(cmd.RequestID, agentID, cmd.Type)
	}

	envelope := runtime.NewCommandEnvelope(
		cmd.Type,
		cmd.RequestID,
		cmd.CorrelationID,
		agentID,
		cmd.ProjectID,
		cmd.Source,
		cmd.Payload,
	)

	logger.Info("control_envelope_debug",
		"agent_id", agentID,
		"command_type", cmd.Type,
		"request_id", cmd.RequestID,
		"source", envelope.Source,
		"envelope_json", fmt.Sprintf("%+v", envelope),
	)

	if err := s.hub.SendToAgent(agentID, envelope); err != nil {
		logger.Error("control_dispatch_send_failed",
			"agent_id", agentID,
			"command_type", cmd.Type,
			"request_id", cmd.RequestID,
			"error", err.Error(),
		)
		return nil, err
	}

	logger.Info("control_dispatched",
		"agent_id", agentID,
		"command_type", cmd.Type,
		"request_id", cmd.RequestID,
		"correlation_id", cmd.CorrelationID,
	)

	return &runtime.CommandResult{
		RequestID: cmd.RequestID,
		Status:    "dispatched",
	}, nil
}

func (s *ControlService) WaitForCommand(ctx context.Context, requestID string) (*TrackedCommand, error) {
	if s.commandTracker == nil {
		return nil, fmt.Errorf("command tracker not configured")
	}
	return s.commandTracker.WaitForResult(ctx, requestID)
}

func (s *ControlService) HandleAgentResponse(agentID string, raw []byte) error {
	var response struct {
		Type      string         `json:"type"`
		RequestID string         `json:"request_id"`
		Status    string         `json:"status"`
		Output    map[string]any `json:"output,omitempty"`
		Error     string         `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("invalid agent response: %w", err)
	}

	if response.RequestID == "" {
		return fmt.Errorf("agent response missing request_id")
	}

	return nil
}
