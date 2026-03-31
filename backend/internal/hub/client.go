package hub

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	userID     string
	email      string
	role       string
	pongWait   time.Duration
	pingPeriod time.Duration
}

func NewClient(
	hub *Hub,
	conn *websocket.Conn,
	userID string,
	email, role string,
	pongWait, pingPeriod time.Duration,
) *Client {
	return &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan []byte, 256),
		userID:     userID,
		email:      email,
		role:       role,
		pongWait:   pongWait,
		pingPeriod: pingPeriod,
	}
}

func (c *Client) UserID() string {
	return c.userID
}

func (c *Client) ReadLoop(onMessage func([]byte), onClose func()) {
	defer func() {
		onClose()
		_ = c.conn.Close()
	}()

	_ = c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		onMessage(message)
	}
}

func (c *Client) WriteLoop() {
	ticker := time.NewTicker(c.pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendJSON(payload any) error {
	message, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	c.send <- message
	return nil
}
