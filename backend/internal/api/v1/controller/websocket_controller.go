package controller

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"lazyops-server/internal/api/middleware"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/config"
	"lazyops-server/internal/hub"
	"lazyops-server/internal/service"
)

type WebSocketController struct {
	hub      *hub.Hub
	agents   *service.AgentService
	upgrader websocket.Upgrader
	cfg      config.Config
}

func NewWebSocketController(h *hub.Hub, agents *service.AgentService, cfg config.Config) *WebSocketController {
	return &WebSocketController{
		hub:    h,
		agents: agents,
		cfg:    cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
			CheckOrigin:     middleware.BuildWebSocketOriginChecker(cfg.Security.AllowedOrigins),
		},
	}
}

func (ctl *WebSocketController) AgentStream(c *gin.Context) {
	conn, err := ctl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	claims := middleware.MustClaims(c)
	client := hub.NewClient(
		ctl.hub,
		conn,
		claims.UserID,
		claims.Email,
		claims.Role,
		ctl.cfg.WebSocket.PongWait,
		ctl.cfg.WebSocket.PingPeriod,
	)
	ctl.hub.Register(client)

	_ = client.SendJSON(gin.H{
		"type":    "welcome",
		"message": "websocket connected",
		"user":    claims.Email,
	})

	go client.WriteLoop()
	client.ReadLoop(func(message []byte) {
		ctl.handleAgentMessage(client, message)
	}, func() {
		ctl.hub.Unregister(client)
	})
}

func (ctl *WebSocketController) handleAgentMessage(client *hub.Client, raw []byte) {
	var incoming requestdto.AgentStatusWSMessage
	if err := json.Unmarshal(raw, &incoming); err != nil {
		_ = client.SendJSON(gin.H{"type": "error", "message": "invalid websocket payload"})
		return
	}

	if incoming.Type != "agent.status.update" {
		_ = client.SendJSON(gin.H{"type": "ignored", "message": "unsupported event type"})
		return
	}

	updated, err := ctl.agents.UpdateStatus(mapper.ToAgentStatusWSCommand(incoming, "websocket"))
	if err != nil {
		_ = client.SendJSON(gin.H{"type": "error", "message": err.Error()})
		return
	}

	if err := ctl.hub.Broadcast(mapper.ToAgentRealtimeEventResponse(ctl.agents.BuildRealtimeEvent(*updated, "websocket"))); err != nil {
		hub.LogBroadcastFailure(err)
	}
}
