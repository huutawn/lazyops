package runtime

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/state"
)

func TestPostgresCompatAdapterBridgesStartup(t *testing.T) {
	upstreamListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen upstream: %v", err)
	}
	defer upstreamListener.Close()

	upstreamReady := make(chan error, 1)
	go func() {
		conn, err := upstreamListener.Accept()
		if err != nil {
			upstreamReady <- err
			return
		}
		defer conn.Close()

		params, err := readPostgresStartup(conn)
		if err != nil {
			upstreamReady <- err
			return
		}
		if params["user"] != "lazyops_managed" {
			upstreamReady <- errUnexpectedPostgresParam("user", params["user"])
			return
		}
		if params["database"] != "app" {
			upstreamReady <- errUnexpectedPostgresParam("database", params["database"])
			return
		}
		if err := writePostgresTypedMessage(conn, 'R', []byte{0, 0, 0, 3}); err != nil {
			upstreamReady <- err
			return
		}
		msgType, payload, err := readPostgresMessage(conn)
		if err != nil {
			upstreamReady <- err
			return
		}
		if msgType != 'p' {
			upstreamReady <- fmt.Errorf("expected password message, got %q", msgType)
			return
		}
		if string(payload[:len(payload)-1]) != "adapter-secret" {
			upstreamReady <- fmt.Errorf("expected upstream password adapter-secret, got %q", string(payload[:len(payload)-1]))
			return
		}
		if err := writePostgresTypedMessage(conn, 'R', []byte{0, 0, 0, 0}); err != nil {
			upstreamReady <- err
			return
		}
		if err := writePostgresTypedMessage(conn, 'S', []byte("client_encoding\x00UTF8\x00")); err != nil {
			upstreamReady <- err
			return
		}
		if err := writePostgresTypedMessage(conn, 'Z', []byte{'I'}); err != nil {
			upstreamReady <- err
			return
		}
		upstreamReady <- nil
	}()

	adapter := NewPostgresCompatAdapter(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stateKey := "adapter-bridge-state-key"
	encryptedSecret, err := state.EncryptSecret("adapter-secret", stateKey)
	if err != nil {
		t.Fatalf("encrypt adapter secret: %v", err)
	}
	t.Setenv("AGENT_STATE_ENCRYPTION_KEY", stateKey)

	listenerPort := freeTCPPort(t)
	err = adapter.Start(ctx, SidecarManagedDBAdapterContract{
		Alias:                     "postgres",
		TargetService:             "lazyops-internal-postgres",
		Protocol:                  "tcp",
		Engine:                    "postgresql",
		ListenerHost:              "127.0.0.1",
		ListenerPort:              listenerPort,
		ListenerEndpoint:          "localhost:5432",
		Upstream:                  upstreamListener.Addr().String(),
		UpstreamUsername:          "lazyops_managed",
		UpstreamDatabase:          "app",
		UpstreamPasswordEncrypted: encryptedSecret,
	})
	if err != nil {
		t.Fatalf("start adapter: %v", err)
	}
	defer adapter.StopAll()

	clientConn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(listenerPort)), time.Second)
	if err != nil {
		t.Fatalf("dial adapter: %v", err)
	}
	defer clientConn.Close()

	if err := writePostgresStartup(clientConn, map[string]string{
		"user":     "postgres",
		"database": "postgres",
	}); err != nil {
		t.Fatalf("write startup: %v", err)
	}

	msgType, payload, err := readPostgresMessage(clientConn)
	if err != nil {
		t.Fatalf("read auth ok: %v", err)
	}
	if msgType != 'R' || len(payload) < 4 || payload[3] != 0 {
		t.Fatalf("expected AuthenticationOk, got type=%q payload=%v", msgType, payload)
	}

	seenReady := false
	for i := 0; i < 4; i++ {
		msgType, _, err = readPostgresMessage(clientConn)
		if err != nil {
			t.Fatalf("read startup response: %v", err)
		}
		if msgType == 'Z' {
			seenReady = true
			break
		}
	}
	if !seenReady {
		t.Fatal("expected ReadyForQuery from compatibility adapter")
	}

	if err := <-upstreamReady; err != nil {
		t.Fatalf("upstream assertion failed: %v", err)
	}
}

func TestMaterializeManagedDBAdapterSecretSupportsEncryptedAndPlaintextFallback(t *testing.T) {
	encryptedKey := "materialize-managed-db-key"
	encryptedSecret, err := state.EncryptSecret("db-secret", encryptedKey)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}

	contract := SidecarManagedDBAdapterContract{
		Alias:                     "postgres",
		UpstreamPasswordEncrypted: encryptedSecret,
	}
	if err := materializeManagedDBAdapterSecret(&contract, encryptedKey); err != nil {
		t.Fatalf("materialize encrypted secret: %v", err)
	}
	if contract.UpstreamPassword != "db-secret" {
		t.Fatalf("expected decrypted secret db-secret, got %q", contract.UpstreamPassword)
	}

	plainContract := SidecarManagedDBAdapterContract{
		Alias:                     "postgres",
		UpstreamPasswordPlaintext: "plain-secret",
	}
	if err := materializeManagedDBAdapterSecret(&plainContract, ""); err != nil {
		t.Fatalf("materialize plaintext fallback: %v", err)
	}
	if plainContract.UpstreamPassword != "plain-secret" {
		t.Fatalf("expected plaintext fallback plain-secret, got %q", plainContract.UpstreamPassword)
	}
}

func TestRewritePostgresHostAuthContentUsesPasswordAuth(t *testing.T) {
	content := "" +
		"local all all trust\n" +
		"host all all all trust\n" +
		"host replication all all scram-sha-256\n"
	updated := rewritePostgresHostAuthContent(content, "password")
	if updated == content {
		t.Fatal("expected host auth content to change")
	}
	if got := countSubstring(updated, "password"); got != 2 {
		t.Fatalf("expected both host lines to switch to password auth, got %d in %q", got, updated)
	}
	if countSubstring(updated, "trust") != 1 {
		t.Fatalf("expected local trust line to remain untouched, got %q", updated)
	}
}

func errUnexpectedPostgresParam(key, value string) error {
	return &unexpectedPostgresParamError{Key: key, Value: value}
}

type unexpectedPostgresParamError struct {
	Key   string
	Value string
}

func (e *unexpectedPostgresParamError) Error() string {
	return "unexpected postgres startup param " + e.Key + "=" + e.Value
}

func itoa(value int) string {
	return fmt.Sprintf("%d", value)
}

func countSubstring(value, needle string) int {
	return len(strings.Split(value, needle)) - 1
}
