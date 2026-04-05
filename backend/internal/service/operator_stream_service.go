package service

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"
)

type OperatorClient struct {
	UserID string
	Send   chan []byte
}

type OperatorStreamHub struct {
	mu         sync.RWMutex
	clients    map[string][]*OperatorClient
	register   chan *OperatorClient
	unregister chan *OperatorClient
	broadcast  chan operatorBroadcast
	once       sync.Once
}

type operatorBroadcast struct {
	userID    string
	eventType string
	payload   []byte
}

func NewOperatorStreamHub() *OperatorStreamHub {
	return &OperatorStreamHub{
		clients:    make(map[string][]*OperatorClient),
		register:   make(chan *OperatorClient, 64),
		unregister: make(chan *OperatorClient, 64),
		broadcast:  make(chan operatorBroadcast, 256),
	}
}

func (h *OperatorStreamHub) Start() {
	h.once.Do(func() {
		go h.run()
	})
}

func (h *OperatorStreamHub) Register(client *OperatorClient) {
	h.register <- client
}

func (h *OperatorStreamHub) Unregister(client *OperatorClient) {
	h.unregister <- client
}

func (h *OperatorStreamHub) BroadcastEvent(eventType string, payload any) error {
	data, err := serializeOperatorEvent(eventType, payload)
	if err != nil {
		return err
	}

	h.broadcast <- operatorBroadcast{
		userID:    "",
		eventType: eventType,
		payload:   data,
	}
	return nil
}

func (h *OperatorStreamHub) BroadcastEventToUser(userID string, eventType string, payload any) error {
	data, err := serializeOperatorEvent(eventType, payload)
	if err != nil {
		return err
	}

	h.broadcast <- operatorBroadcast{
		userID:    userID,
		eventType: eventType,
		payload:   data,
	}
	return nil
}

func (h *OperatorStreamHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserID] = append(h.clients[client.UserID], client)
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			clients := h.clients[client.UserID]
			for i, c := range clients {
				if c == client {
					h.clients[client.UserID] = append(clients[:i], clients[i+1:]...)
					close(client.Send)
					break
				}
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			for userID, clients := range h.clients {
				if msg.userID != "" && userID != msg.userID {
					continue
				}
				for _, client := range clients {
					select {
					case client.Send <- msg.payload:
					default:
						h.mu.RUnlock()
						h.mu.Lock()
						remaining := h.clients[userID]
						for i, c := range remaining {
							if c == client {
								h.clients[userID] = append(remaining[:i], remaining[i+1:]...)
								close(client.Send)
								break
							}
						}
						h.mu.Unlock()
						h.mu.RLock()
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func serializeOperatorEvent(eventType string, payload any) ([]byte, error) {
	event := map[string]any{
		"type":        eventType,
		"payload":     payload,
		"occurred_at": time.Now().UTC(),
	}
	if correlationID := extractOperatorCorrelationID(payload); correlationID != "" {
		event["correlation_id"] = correlationID
	}
	return json.Marshal(event)
}

func extractOperatorCorrelationID(payload any) string {
	if payload == nil {
		return ""
	}
	switch value := payload.(type) {
	case map[string]any:
		for _, key := range []string{"correlation_id", "x_correlation_id"} {
			if raw, ok := value[key].(string); ok && raw != "" {
				return raw
			}
		}
	}

	ref := reflect.ValueOf(payload)
	if ref.Kind() == reflect.Pointer && !ref.IsNil() {
		ref = ref.Elem()
	}
	if ref.Kind() != reflect.Struct {
		return ""
	}
	for _, fieldName := range []string{"CorrelationID", "XCorrelationID"} {
		field := ref.FieldByName(fieldName)
		if field.IsValid() && field.Kind() == reflect.String && field.String() != "" {
			return field.String()
		}
	}
	return ""
}
