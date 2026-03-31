package control

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"lazyops-agent/internal/contracts"
)

type WebSocketClientConfig struct {
	ControlPlaneURL     string
	DialTimeout         time.Duration
	WriteTimeout        time.Duration
	PongWait            time.Duration
	PingPeriod          time.Duration
	ReconnectMinBackoff time.Duration
	ReconnectMaxBackoff time.Duration
	ReconnectJitter     time.Duration
	SendBufferSize      int
}

type queuedMessage struct {
	transcript *contracts.CommandEnvelope
	raw        []byte
}

type WebSocketClient struct {
	logger    *slog.Logger
	cfg       WebSocketClientConfig
	bootstrap *bootstrapRegistry

	mu              sync.RWMutex
	started         bool
	closing         bool
	connected       bool
	auth            contracts.SessionAuthPayload
	commandHandler  CommandHandler
	transcript      []contracts.CommandEnvelope
	cachedHandshake *queuedMessage
	sendCh          chan queuedMessage
	closeCh         chan struct{}
	doneCh          chan struct{}
	readyCh         chan struct{}
}

func NewWebSocketClient(logger *slog.Logger, cfg WebSocketClientConfig) *WebSocketClient {
	bufferSize := cfg.SendBufferSize
	if bufferSize <= 0 {
		bufferSize = 64
	}
	cfg.SendBufferSize = bufferSize

	return &WebSocketClient{
		logger:    logger,
		cfg:       cfg,
		bootstrap: newDefaultBootstrapRegistry(),
	}
}

func (c *WebSocketClient) Enroll(ctx context.Context, req contracts.EnrollAgentRequest) (contracts.EnrollAgentResponse, error) {
	response, err := c.bootstrap.Enroll(ctx, req)
	if err != nil {
		return contracts.EnrollAgentResponse{}, err
	}

	c.logger.Info("control enrollment satisfied by locked bootstrap stub",
		"target_ref", req.Machine.Labels["target_ref"],
		"runtime_mode", req.RuntimeMode,
		"agent_kind", req.AgentKind,
	)

	return response, nil
}

func (c *WebSocketClient) Connect(ctx context.Context, auth contracts.SessionAuthPayload) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}

	c.started = true
	c.auth = auth
	c.sendCh = make(chan queuedMessage, c.cfg.SendBufferSize)
	c.closeCh = make(chan struct{})
	c.doneCh = make(chan struct{})
	c.readyCh = make(chan struct{})
	c.mu.Unlock()

	go c.run()

	select {
	case <-c.readyCh:
		return nil
	case <-ctx.Done():
		_ = c.Close(context.Background())
		return ctx.Err()
	case <-c.doneCh:
		if c.isClosing() {
			return ErrControlClientClosed
		}
		return ErrControlClientNotConnected
	}
}

func (c *WebSocketClient) RegisterCommandHandler(handler CommandHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commandHandler = handler
}

func (c *WebSocketClient) SendHandshake(ctx context.Context, payload contracts.AgentHandshakePayload) error {
	envelope, raw, err := buildEnvelope(handshakeEnvelopeType, payload.Auth.AgentID, payload)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.cachedHandshake = &queuedMessage{
		transcript: &envelope,
		raw:        raw,
	}
	c.mu.Unlock()

	return c.enqueue(ctx, queuedMessage{transcript: &envelope, raw: raw})
}

func (c *WebSocketClient) SendHeartbeat(ctx context.Context, payload contracts.HeartbeatPayload) error {
	envelope, raw, err := buildEnvelope(heartbeatEnvelopeType, payload.AgentID, payload)
	if err != nil {
		return err
	}

	return c.enqueue(ctx, queuedMessage{transcript: &envelope, raw: raw})
}

func (c *WebSocketClient) SendCommandAck(ctx context.Context, envelope contracts.CommandAckEnvelope) error {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return c.enqueue(ctx, queuedMessage{raw: raw})
}

func (c *WebSocketClient) SendCommandNack(ctx context.Context, envelope contracts.CommandNackEnvelope) error {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return c.enqueue(ctx, queuedMessage{raw: raw})
}

func (c *WebSocketClient) SendCommandError(ctx context.Context, envelope contracts.CommandErrorEnvelope) error {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return c.enqueue(ctx, queuedMessage{raw: raw})
}

func (c *WebSocketClient) Close(ctx context.Context) error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	if !c.closing {
		c.closing = true
		close(c.closeCh)
	}
	doneCh := c.doneCh
	c.mu.Unlock()

	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *WebSocketClient) Transcript() []contracts.CommandEnvelope {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]contracts.CommandEnvelope, len(c.transcript))
	copy(out, c.transcript)
	return out
}

func (c *WebSocketClient) run() {
	defer close(c.doneCh)

	backoff := c.cfg.ReconnectMinBackoff
	hasConnectedBefore := false

	for {
		if c.isClosing() {
			return
		}

		conn, endpoint, err := c.dial()
		if err != nil {
			if c.isClosing() {
				return
			}
			c.logger.Warn("control session dial failed",
				"endpoint", endpoint,
				"error", err,
			)
			if !c.sleepReconnect(backoff) {
				return
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		replayHandshake := hasConnectedBefore
		hasConnectedBefore = true
		backoff = c.cfg.ReconnectMinBackoff
		c.setConnected(true)
		c.signalReady()

		c.logger.Info("control session connected",
			"endpoint", endpoint,
			"path", contracts.ControlWebSocketPath,
			"agent_id", c.auth.AgentID,
			"session_id", c.auth.SessionID,
			"replay_handshake", replayHandshake,
		)

		err = c.serveConnection(conn, replayHandshake)
		c.setConnected(false)
		_ = conn.Close()

		if c.isClosing() {
			return
		}

		c.logger.Warn("control session disconnected",
			"agent_id", c.auth.AgentID,
			"session_id", c.auth.SessionID,
			"error", err,
		)
		if !c.sleepReconnect(backoff) {
			return
		}
		backoff = c.nextBackoff(backoff)
	}
}

func (c *WebSocketClient) dial() (*websocket.Conn, string, error) {
	endpoint, err := controlWebSocketURL(c.cfg.ControlPlaneURL)
	if err != nil {
		return nil, "", err
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.auth.AgentToken)
	header.Set("X-Agent-ID", c.auth.AgentID)
	header.Set("X-Agent-Session-ID", c.auth.SessionID)
	header.Set("X-Agent-Runtime-Mode", string(c.auth.RuntimeMode))
	header.Set("X-Agent-Kind", string(c.auth.AgentKind))
	header.Set("X-Agent-Handshake-Version", c.auth.HandshakeVer)

	dialer := websocket.Dialer{
		HandshakeTimeout: c.cfg.DialTimeout,
	}

	conn, resp, err := dialer.Dial(endpoint, header)
	if err != nil {
		if resp != nil {
			return nil, endpoint, fmt.Errorf("dial status %s: %w", resp.Status, err)
		}
		return nil, endpoint, err
	}
	return conn, endpoint, nil
}

func (c *WebSocketClient) serveConnection(conn *websocket.Conn, replayHandshake bool) error {
	errCh := make(chan error, 2)

	go c.readLoop(conn, errCh)
	go c.writeLoop(conn, replayHandshake, errCh)

	err := <-errCh
	if err == nil {
		return nil
	}
	return err
}

func (c *WebSocketClient) readLoop(conn *websocket.Conn, errCh chan<- error) {
	_ = conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	})

	for {
		messageType, raw, err := conn.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		c.handleInbound(raw)
	}
}

func (c *WebSocketClient) writeLoop(conn *websocket.Conn, replayHandshake bool, errCh chan<- error) {
	pingTicker := time.NewTicker(c.cfg.PingPeriod)
	defer pingTicker.Stop()

	if replayHandshake {
		if message, ok := c.cachedHandshakeSnapshot(); ok {
			if err := c.writeFrame(conn, websocket.TextMessage, message.raw); err != nil {
				errCh <- err
				return
			}
			agentID := c.auth.AgentID
			if message.transcript != nil {
				agentID = message.transcript.AgentID
			}
			c.logger.Info("replayed cached agent handshake",
				"agent_id", agentID,
				"session_id", c.auth.SessionID,
			)
		}
	}

	for {
		select {
		case <-c.closeCh:
			_ = c.writeFrame(conn, websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"))
			errCh <- ErrControlClientClosed
			return
		case message := <-c.sendCh:
			if err := c.writeFrame(conn, websocket.TextMessage, message.raw); err != nil {
				errCh <- err
				return
			}
		case <-pingTicker.C:
			if err := c.writeFrame(conn, websocket.PingMessage, nil); err != nil {
				errCh <- err
				return
			}
		}
	}
}

func (c *WebSocketClient) handleInbound(raw []byte) {
	var envelope contracts.CommandEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil {
		c.logger.Debug("received control envelope",
			"type", envelope.Type,
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
		)
		if c.shouldDispatch(envelope.Type) {
			handler := c.commandHandlerSnapshot()
			if handler != nil {
				go handler(context.Background(), envelope)
			}
		}
		return
	}

	c.logger.Debug("received control message",
		"bytes", len(raw),
	)
}

func (c *WebSocketClient) enqueue(ctx context.Context, message queuedMessage) error {
	c.mu.RLock()
	started := c.started
	closing := c.closing
	sendCh := c.sendCh
	closeCh := c.closeCh
	c.mu.RUnlock()

	if !started {
		return ErrControlClientNotConnected
	}
	if closing {
		return ErrControlClientClosed
	}

	select {
	case sendCh <- message:
		if message.transcript != nil {
			c.appendTranscript(*message.transcript)
		}
		return nil
	case <-closeCh:
		return ErrControlClientClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *WebSocketClient) appendTranscript(envelope contracts.CommandEnvelope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transcript = append(c.transcript, envelope)
}

func (c *WebSocketClient) cachedHandshakeSnapshot() (queuedMessage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cachedHandshake == nil {
		return queuedMessage{}, false
	}
	return *c.cachedHandshake, true
}

func (c *WebSocketClient) commandHandlerSnapshot() CommandHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.commandHandler
}

func (c *WebSocketClient) shouldDispatch(messageType contracts.CommandType) bool {
	_, ok := contracts.CommandHandlerBindings[messageType]
	return ok
}

func (c *WebSocketClient) signalReady() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.readyCh:
		return
	default:
		close(c.readyCh)
	}
}

func (c *WebSocketClient) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

func (c *WebSocketClient) isClosing() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closing
}

func (c *WebSocketClient) writeFrame(conn *websocket.Conn, messageType int, payload []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout)); err != nil {
		return err
	}
	return conn.WriteMessage(messageType, payload)
}

func (c *WebSocketClient) sleepReconnect(base time.Duration) bool {
	delay := base
	if c.cfg.ReconnectJitter > 0 {
		delay += time.Duration(rand.Int63n(int64(c.cfg.ReconnectJitter) + 1))
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-c.closeCh:
		return false
	case <-timer.C:
		return true
	}
}

func (c *WebSocketClient) nextBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return c.cfg.ReconnectMinBackoff
	}
	next := current * 2
	if next > c.cfg.ReconnectMaxBackoff {
		return c.cfg.ReconnectMaxBackoff
	}
	return next
}

func controlWebSocketURL(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("control plane URL host is required")
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported control plane URL scheme %q", parsed.Scheme)
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = contracts.ControlWebSocketPath
	} else if !strings.HasSuffix(parsed.Path, contracts.ControlWebSocketPath) {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + contracts.ControlWebSocketPath
	}

	return parsed.String(), nil
}
