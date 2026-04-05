package controller

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/config"
	"lazyops-server/internal/service"
)

type OperatorStreamController struct {
	streamHub *service.OperatorStreamHub
	cfg       config.Config
	upgrader  websocket.Upgrader
}

func NewOperatorStreamController(hub *service.OperatorStreamHub, cfg config.Config) *OperatorStreamController {
	return &OperatorStreamController{
		streamHub: hub,
		cfg:       cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
			CheckOrigin:     middleware.BuildWebSocketOriginChecker(cfg.Security.AllowedOrigins),
		},
	}
}

func (ctl *OperatorStreamController) OperatorStream(c *gin.Context) {
	conn, err := ctl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	claims := middleware.MustClaims(c)
	client := &service.OperatorClient{
		UserID: claims.UserID,
		Send:   make(chan []byte, 256),
	}
	ctl.streamHub.Register(client)

	_ = conn.WriteJSON(map[string]any{
		"type":    "welcome",
		"message": "operator stream connected",
		"user":    claims.Email,
	})

	go ctl.writeLoop(conn, client)
	ctl.readLoop(conn, client, claims.UserID)
}

func (ctl *OperatorStreamController) writeLoop(conn *websocket.Conn, client *service.OperatorClient) {
	ticker := time.NewTicker(ctl.cfg.WebSocket.PingPeriod)
	defer func() {
		ticker.Stop()
		ctl.streamHub.Unregister(client)
		_ = conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (ctl *OperatorStreamController) readLoop(conn *websocket.Conn, client *service.OperatorClient, userID string) {
	defer func() {
		ctl.streamHub.Unregister(client)
		_ = conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(ctl.cfg.WebSocket.PongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(ctl.cfg.WebSocket.PongWait))
	})

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}

		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "ping":
			_ = conn.WriteJSON(map[string]any{"type": "pong"})
		default:
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": "operator stream is read-only for events"})
		}
	}
}
