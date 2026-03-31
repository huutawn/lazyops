package control

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"lazyops-agent/internal/contracts"
)

func TestWebSocketClientConnectsAndSendsHandshakeAndHeartbeat(t *testing.T) {
	headersCh := make(chan http.Header, 1)
	messagesCh := make(chan []byte, 4)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contracts.ControlWebSocketPath {
			http.NotFound(w, r)
			return
		}

		headersCh <- r.Header.Clone()

		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			messagesCh <- append([]byte(nil), message...)
		}
	}))
	defer server.Close()

	client := NewWebSocketClient(testLogger(), WebSocketClientConfig{
		ControlPlaneURL:     wsBaseURL(t, server.URL),
		DialTimeout:         time.Second,
		WriteTimeout:        time.Second,
		PongWait:            3 * time.Second,
		PingPeriod:          time.Second,
		ReconnectMinBackoff: 20 * time.Millisecond,
		ReconnectMaxBackoff: 40 * time.Millisecond,
		ReconnectJitter:     0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auth := contracts.SessionAuthPayload{
		AgentID:      "agt_ws_test",
		AgentToken:   "agt-token-value",
		SessionID:    "sess_ws_test",
		RuntimeMode:  contracts.RuntimeModeStandalone,
		AgentKind:    contracts.AgentKindInstance,
		HandshakeVer: "v0",
		SentAt:       time.Now().UTC(),
	}

	if err := client.Connect(ctx, auth); err != nil {
		t.Fatalf("connect control client: %v", err)
	}

	handshake := contracts.AgentHandshakePayload{
		Auth: auth,
		Machine: contracts.MachineInfo{
			Hostname: "local-dev",
			OS:       "linux",
			Arch:     "amd64",
		},
		State: contracts.AgentStateConnected,
		Capabilities: contracts.CapabilityReportPayload{
			AgentKind:   contracts.AgentKindInstance,
			RuntimeMode: contracts.RuntimeModeStandalone,
		},
	}
	if err := client.SendHandshake(ctx, handshake); err != nil {
		t.Fatalf("send handshake: %v", err)
	}

	heartbeat := contracts.HeartbeatPayload{
		AgentID:     auth.AgentID,
		SessionID:   auth.SessionID,
		State:       contracts.AgentStateConnected,
		RuntimeMode: auth.RuntimeMode,
		AgentKind:   auth.AgentKind,
		SentAt:      time.Now().UTC(),
	}
	if err := client.SendHeartbeat(ctx, heartbeat); err != nil {
		t.Fatalf("send heartbeat: %v", err)
	}

	headers := mustReceiveHeader(t, headersCh)
	if got := headers.Get("Authorization"); got != "Bearer "+auth.AgentToken {
		t.Fatalf("unexpected authorization header %q", got)
	}
	if got := headers.Get("X-Agent-Session-ID"); got != auth.SessionID {
		t.Fatalf("unexpected session ID header %q", got)
	}
	if got := headers.Get("X-Agent-Handshake-Version"); got != auth.HandshakeVer {
		t.Fatalf("unexpected handshake version header %q", got)
	}

	first := decodeEnvelope(t, mustReceiveMessage(t, messagesCh))
	second := decodeEnvelope(t, mustReceiveMessage(t, messagesCh))

	if first.Type != handshakeEnvelopeType {
		t.Fatalf("unexpected first envelope type %q", first.Type)
	}
	if second.Type != heartbeatEnvelopeType {
		t.Fatalf("unexpected second envelope type %q", second.Type)
	}

	var gotHandshake contracts.AgentHandshakePayload
	if err := json.Unmarshal(first.Payload, &gotHandshake); err != nil {
		t.Fatalf("unmarshal handshake payload: %v", err)
	}
	if gotHandshake.Auth.SessionID != auth.SessionID {
		t.Fatalf("unexpected handshake session ID %q", gotHandshake.Auth.SessionID)
	}

	var gotHeartbeat contracts.HeartbeatPayload
	if err := json.Unmarshal(second.Payload, &gotHeartbeat); err != nil {
		t.Fatalf("unmarshal heartbeat payload: %v", err)
	}
	if gotHeartbeat.SessionID != auth.SessionID {
		t.Fatalf("unexpected heartbeat session ID %q", gotHeartbeat.SessionID)
	}

	if err := client.Close(ctx); err != nil {
		t.Fatalf("close control client: %v", err)
	}
}

func TestWebSocketClientReconnectsAndReplaysHandshake(t *testing.T) {
	firstConnCh := make(chan http.Header, 1)
	secondConnCh := make(chan http.Header, 1)
	replayedHandshakeCh := make(chan contracts.CommandEnvelope, 1)
	serverDone := make(chan struct{})

	var connectionCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contracts.ControlWebSocketPath {
			http.NotFound(w, r)
			return
		}

		currentConnection := int(connectionCount.Add(1))
		if currentConnection == 1 {
			firstConnCh <- r.Header.Clone()
		} else {
			secondConnCh <- r.Header.Clone()
		}

		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			envelope := decodeEnvelope(t, message)
			if currentConnection == 1 {
				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "rotate"),
					time.Now().Add(time.Second),
				)
				return
			}

			replayedHandshakeCh <- envelope
			<-serverDone
			return
		}
	}))
	defer server.Close()
	defer close(serverDone)

	client := NewWebSocketClient(testLogger(), WebSocketClientConfig{
		ControlPlaneURL:     wsBaseURL(t, server.URL),
		DialTimeout:         time.Second,
		WriteTimeout:        time.Second,
		PongWait:            3 * time.Second,
		PingPeriod:          time.Second,
		ReconnectMinBackoff: 20 * time.Millisecond,
		ReconnectMaxBackoff: 40 * time.Millisecond,
		ReconnectJitter:     0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auth := contracts.SessionAuthPayload{
		AgentID:      "agt_ws_resume",
		AgentToken:   "agt-token-resume",
		SessionID:    "sess_resume",
		RuntimeMode:  contracts.RuntimeModeStandalone,
		AgentKind:    contracts.AgentKindInstance,
		HandshakeVer: "v0",
		SentAt:       time.Now().UTC(),
	}

	if err := client.Connect(ctx, auth); err != nil {
		t.Fatalf("connect control client: %v", err)
	}

	if err := client.SendHandshake(ctx, contracts.AgentHandshakePayload{
		Auth: auth,
		Machine: contracts.MachineInfo{
			Hostname: "local-dev",
			OS:       "linux",
			Arch:     "amd64",
		},
		State: contracts.AgentStateConnected,
	}); err != nil {
		t.Fatalf("send handshake: %v", err)
	}

	firstHeaders := mustReceiveHeader(t, firstConnCh)
	secondHeaders := mustReceiveHeader(t, secondConnCh)
	replayed := mustReceiveEnvelope(t, replayedHandshakeCh)

	if got := firstHeaders.Get("X-Agent-Session-ID"); got != auth.SessionID {
		t.Fatalf("unexpected first connection session ID %q", got)
	}
	if got := secondHeaders.Get("X-Agent-Session-ID"); got != auth.SessionID {
		t.Fatalf("unexpected second connection session ID %q", got)
	}
	if replayed.Type != handshakeEnvelopeType {
		t.Fatalf("unexpected replayed envelope type %q", replayed.Type)
	}

	var payload contracts.AgentHandshakePayload
	if err := json.Unmarshal(replayed.Payload, &payload); err != nil {
		t.Fatalf("unmarshal replayed handshake payload: %v", err)
	}
	if payload.Auth.SessionID != auth.SessionID {
		t.Fatalf("expected replayed handshake session %q, got %q", auth.SessionID, payload.Auth.SessionID)
	}

	if err := client.Close(ctx); err != nil {
		t.Fatalf("close control client: %v", err)
	}
}

func TestWebSocketClientDispatchesInboundCommandEnvelope(t *testing.T) {
	receivedCh := make(chan contracts.CommandEnvelope, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contracts.ControlWebSocketPath {
			http.NotFound(w, r)
			return
		}

		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		command := contracts.CommandEnvelope{
			Type:          contracts.CommandPrepareReleaseWorkspace,
			RequestID:     "req_dispatch",
			CorrelationID: "corr_dispatch",
			AgentID:       "agt_ws_dispatch",
			Source:        contracts.EnvelopeSourceBackend,
			OccurredAt:    time.Now().UTC(),
			Payload:       json.RawMessage(`{"revision_id":"rev_123"}`),
		}
		raw, err := json.Marshal(command)
		if err != nil {
			t.Errorf("marshal command envelope: %v", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
			t.Errorf("write command envelope: %v", err)
			return
		}

		<-time.After(100 * time.Millisecond)
	}))
	defer server.Close()

	client := NewWebSocketClient(testLogger(), WebSocketClientConfig{
		ControlPlaneURL:     wsBaseURL(t, server.URL),
		DialTimeout:         time.Second,
		WriteTimeout:        time.Second,
		PongWait:            3 * time.Second,
		PingPeriod:          time.Second,
		ReconnectMinBackoff: 20 * time.Millisecond,
		ReconnectMaxBackoff: 40 * time.Millisecond,
		ReconnectJitter:     0,
	})
	client.RegisterCommandHandler(func(_ context.Context, envelope contracts.CommandEnvelope) {
		receivedCh <- envelope
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auth := contracts.SessionAuthPayload{
		AgentID:      "agt_ws_dispatch",
		AgentToken:   "agt-token-dispatch",
		SessionID:    "sess_dispatch",
		RuntimeMode:  contracts.RuntimeModeStandalone,
		AgentKind:    contracts.AgentKindInstance,
		HandshakeVer: "v0",
		SentAt:       time.Now().UTC(),
	}

	if err := client.Connect(ctx, auth); err != nil {
		t.Fatalf("connect control client: %v", err)
	}

	select {
	case envelope := <-receivedCh:
		if envelope.Type != contracts.CommandPrepareReleaseWorkspace {
			t.Fatalf("unexpected command type %q", envelope.Type)
		}
		if envelope.RequestID != "req_dispatch" {
			t.Fatalf("unexpected request ID %q", envelope.RequestID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for inbound command dispatch")
	}

	if err := client.Close(ctx); err != nil {
		t.Fatalf("close control client: %v", err)
	}
}

func wsBaseURL(t *testing.T, raw string) string {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	parsed.Scheme = "ws"
	parsed.Path = ""
	return parsed.String()
}

func mustReceiveHeader(t *testing.T, ch <-chan http.Header) http.Header {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for websocket headers")
		return nil
	}
}

func mustReceiveMessage(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for websocket message")
		return nil
	}
}

func mustReceiveEnvelope(t *testing.T, ch <-chan contracts.CommandEnvelope) contracts.CommandEnvelope {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for replayed handshake")
		return contracts.CommandEnvelope{}
	}
}

func decodeEnvelope(t *testing.T, raw []byte) contracts.CommandEnvelope {
	t.Helper()

	var envelope contracts.CommandEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode command envelope: %v", err)
	}
	return envelope
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
