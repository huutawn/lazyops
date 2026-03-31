package hub

import (
	"encoding/json"
	"log/slog"
	"sync"
)

type Hub struct {
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMessage
	once       sync.Once
}

type broadcastMessage struct {
	userID  string
	payload []byte
}

func New() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMessage, 256),
	}
}

func (h *Hub) Start() {
	h.once.Do(func() {
		go h.run()
	})
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Broadcast(payload any) error {
	return h.broadcastPayload("", payload)
}

func (h *Hub) BroadcastToUser(userID string, payload any) error {
	return h.broadcastPayload(userID, payload)
}

func (h *Hub) broadcastPayload(userID string, payload any) error {
	message, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	h.broadcast <- broadcastMessage{
		userID:  userID,
		payload: message,
	}
	return nil
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				if message.userID != "" && client.userID != message.userID {
					continue
				}
				select {
				case client.send <- message.payload:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}
		}
	}
}

func LogBroadcastFailure(err error) {
	slog.Error("hub broadcast failed", "error", err)
}
