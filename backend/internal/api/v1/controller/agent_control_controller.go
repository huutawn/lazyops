package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	"lazyops-server/internal/config"
	"lazyops-server/internal/service"
)

type AgentControlController struct {
	controlHub     *service.ControlHub
	commandTracker *service.CommandTracker
	observability  *service.ObservabilityService
	cfg            config.Config
	upgrader       websocket.Upgrader
}

func NewAgentControlController(hub *service.ControlHub, commandTracker *service.CommandTracker, observability *service.ObservabilityService, cfg config.Config) *AgentControlController {
	return &AgentControlController{
		controlHub:     hub,
		commandTracker: commandTracker,
		observability:  observability,
		cfg:            cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
			CheckOrigin:     middleware.BuildWebSocketOriginChecker(cfg.Security.AllowedOrigins),
		},
	}
}

func (ctl *AgentControlController) ControlStream(c *gin.Context) {
	conn, err := ctl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	claims := middleware.MustClaims(c)
	agentID := c.Query("agent_id")
	if agentID == "" {
		agentID = c.GetHeader("X-Agent-ID")
	}
	if agentID == "" {
		_ = conn.WriteJSON(gin.H{"type": "error", "message": "agent_id query parameter is required"})
		_ = conn.Close()
		return
	}

	client := &service.ControlClient{
		AgentID:    agentID,
		InstanceID: c.Query("instance_id"),
		Conn:       conn,
		Registered: time.Now().UTC(),
	}
	ctl.controlHub.Register(client)

	_ = conn.WriteJSON(gin.H{
		"type":     "welcome",
		"message":  "agent control channel connected",
		"agent_id": agentID,
	})

	// Block the HTTP handler goroutine so the WebSocket stays open.
	// runControlLoop runs in a separate goroutine and will close the
	// connection + unregister when done.
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctl.runControlLoop(client, claims.UserID)
	}()
	<-done
}

func (ctl *AgentControlController) runControlLoop(client *service.ControlClient, userID string) {
	slog.Info("control session runControlLoop started", "agent_id", client.AgentID)
	defer func() {
		slog.Info("control session runControlLoop exited", "agent_id", client.AgentID)
		ctl.controlHub.Unregister(client.AgentID)
		_ = client.Conn.Close()
	}()

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			slog.Info("control session ReadMessage error",
				"agent_id", client.AgentID,
				"error", err,
			)
			return
		}

		var msg struct {
			Type      string `json:"type"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			_ = client.Conn.WriteMessage(1, []byte(`{"type":"error","message":"invalid message format"}`))
			continue
		}

		switch msg.Type {
		case "ping":
			_ = client.Conn.WriteMessage(1, []byte(`{"type":"pong"}`))
		case "command_response":
			ctl.handleCommandResponse(client.AgentID, message)
		case "command.ack":
			ctl.handleCommandAck(client.AgentID, message)
		case "command.error":
			ctl.handleCommandError(client.AgentID, message)
		case "agent.log_batch":
			ctl.handleLogBatch(client, message)
		case "agent.topology":
			ctl.handleTopologyReport(client, message)
		default:
			_ = client.Conn.WriteMessage(1, []byte(`{"type":"error","message":"unsupported message type"}`))
		}
	}
}

func (ctl *AgentControlController) handleCommandResponse(agentID string, raw []byte) {
	var response struct {
		Type      string         `json:"type"`
		RequestID string         `json:"request_id"`
		Status    string         `json:"status"`
		Output    map[string]any `json:"output,omitempty"`
		Error     string         `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return
	}

	if response.RequestID == "" {
		_ = ctl.controlHub.SendToAgent(agentID, gin.H{
			"type":    "error",
			"message": "command response missing request_id",
		})
		return
	}

	if ctl.commandTracker != nil {
		state := service.CommandState(response.Status)
		if state == "" {
			state = service.CommandStateDone
		}
		_ = ctl.commandTracker.UpdateState(response.RequestID, state, response.Output, response.Error)
	}
}

func (ctl *AgentControlController) handleCommandAck(agentID string, raw []byte) {
	var ack struct {
		Type      string `json:"type"`
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
		Summary   string `json:"summary,omitempty"`
	}
	if err := json.Unmarshal(raw, &ack); err != nil {
		return
	}

	if ack.RequestID == "" {
		return
	}

	if ctl.commandTracker == nil {
		return
	}

	state := service.CommandStateRunning
	if ack.Status == "done" {
		state = service.CommandStateDone
	} else if ack.Status == "accepted" {
		state = service.CommandStateRunning
	}

	output := map[string]any{}
	if ack.Summary != "" {
		output["summary"] = ack.Summary
	}

	_ = ctl.commandTracker.UpdateState(ack.RequestID, state, output, "")
}

func (ctl *AgentControlController) handleCommandError(agentID string, raw []byte) {
	var errResp struct {
		Type      string         `json:"type"`
		RequestID string         `json:"request_id"`
		Code      string         `json:"code,omitempty"`
		Message   string         `json:"message"`
		Retryable bool           `json:"retryable,omitempty"`
		Details   map[string]any `json:"details,omitempty"`
	}
	if err := json.Unmarshal(raw, &errResp); err != nil {
		return
	}

	if errResp.RequestID == "" {
		return
	}

	if ctl.commandTracker == nil {
		return
	}

	output := map[string]any{
		"code":      errResp.Code,
		"retryable": errResp.Retryable,
	}
	if errResp.Details != nil {
		for k, v := range errResp.Details {
			output[k] = v
		}
	}

	_ = ctl.commandTracker.UpdateState(errResp.RequestID, service.CommandStateFailed, output, errResp.Message)
}

func (ctl *AgentControlController) handleLogBatch(client *service.ControlClient, raw []byte) {
	if ctl.observability == nil {
		return
	}

	var envelope struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return
	}

	var payload struct {
		ProjectID   string    `json:"project_id"`
		BindingID   string    `json:"binding_id"`
		RevisionID  string    `json:"revision_id"`
		ServiceName string    `json:"service_name"`
		CollectedAt time.Time `json:"collected_at"`
		Entries     []struct {
			Timestamp time.Time         `json:"timestamp"`
			Severity  string            `json:"severity"`
			Source    string            `json:"source"`
			Message   string            `json:"message"`
			Excerpt   string            `json:"excerpt"`
			Labels    map[string]string `json:"labels"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		writeControlJSON(client.Conn, gin.H{"type": "error", "message": "invalid log batch payload"})
		return
	}

	entries := make([]service.LogBatchEntry, 0, len(payload.Entries))
	for _, entry := range payload.Entries {
		entries = append(entries, service.LogBatchEntry{
			Timestamp: entry.Timestamp,
			Severity:  entry.Severity,
			Source:    entry.Source,
			Message:   entry.Message,
			Excerpt:   entry.Excerpt,
			Labels:    entry.Labels,
		})
	}

	if _, err := ctl.observability.IngestLogBatch(context.Background(), service.IngestLogBatchCommand{
		ProjectID:   payload.ProjectID,
		BindingID:   payload.BindingID,
		RevisionID:  payload.RevisionID,
		ServiceName: payload.ServiceName,
		Entries:     entries,
		CollectedAt: payload.CollectedAt,
	}); err != nil {
		writeControlJSON(client.Conn, gin.H{"type": "error", "message": "failed to ingest log batch"})
	}
}

func (ctl *AgentControlController) handleTopologyReport(client *service.ControlClient, raw []byte) {
	if ctl.observability == nil {
		return
	}

	var envelope struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return
	}

	var payload struct {
		ProjectID  string    `json:"project_id"`
		SnapshotAt time.Time `json:"snapshot_at"`
		Nodes      []struct {
			NodeRef  string         `json:"node_ref"`
			NodeType string         `json:"node_type"`
			Status   string         `json:"status"`
			Metadata map[string]any `json:"metadata,omitempty"`
		} `json:"nodes"`
		Edges []struct {
			SourceNodeRef string `json:"source_node_ref"`
			TargetNodeRef string `json:"target_node_ref"`
			EdgeType      string `json:"edge_type"`
			Status        string `json:"status"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return
	}

	if payload.ProjectID == "" {
		return
	}

	for _, node := range payload.Nodes {
		metadataJSON := "{}"
		if node.Metadata != nil {
			raw, _ := json.Marshal(node.Metadata)
			metadataJSON = string(raw)
		}
		_ = ctl.observability.UpsertTopologyNode(context.Background(), service.TopologyNodeUpsertCommand{
			ProjectID: payload.ProjectID,
			NodeRef:   node.NodeRef,
			NodeKind:  node.NodeType,
			Name:      node.NodeRef,
			Status:    node.Status,
			Metadata:  metadataJSON,
		})
	}

	for _, edge := range payload.Edges {
		metadataJSON := "{}"
		_ = ctl.observability.UpsertTopologyEdge(context.Background(), service.TopologyEdgeUpsertCommand{
			ProjectID: payload.ProjectID,
			SourceID:  edge.SourceNodeRef,
			TargetID:  edge.TargetNodeRef,
			EdgeKind:  edge.EdgeType,
			Protocol:  "http",
			Metadata:  metadataJSON,
		})
	}
}

func writeControlJSON(conn service.ControlConn, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (ctl *AgentControlController) DispatchCommand(c *gin.Context) {
	agentID := c.Param("agent_id")
	if agentID == "" {
		response.Error(c, http.StatusBadRequest, "agent_id is required", "missing_agent_id", nil)
		return
	}

	var req struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body", "invalid_payload", nil)
		return
	}

	if req.Type == "" {
		response.Error(c, http.StatusBadRequest, "command type is required", "missing_command_type", nil)
		return
	}

	claims := middleware.MustClaims(c)
	cmd := struct {
		Type          string         `json:"type"`
		RequestID     string         `json:"request_id"`
		CorrelationID string         `json:"correlation_id"`
		AgentID       string         `json:"agent_id"`
		ProjectID     string         `json:"project_id"`
		Source        string         `json:"source"`
		OccurredAt    string         `json:"occurred_at"`
		Payload       map[string]any `json:"payload"`
	}{
		Type:          req.Type,
		AgentID:       agentID,
		ProjectID:     c.Query("project_id"),
		Source:        "api",
		CorrelationID: c.GetHeader("X-Correlation-ID"),
		Payload:       req.Payload,
	}

	if err := ctl.controlHub.SendToAgent(agentID, cmd); err != nil {
		response.Error(c, http.StatusNotFound, "agent not connected", "agent_not_connected", nil)
		return
	}

	response.JSON(c, http.StatusAccepted, "command dispatched", gin.H{
		"agent_id": agentID,
		"type":     req.Type,
		"source":   claims.UserID,
	})
}
