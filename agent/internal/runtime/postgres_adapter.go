package runtime

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"lazyops-agent/internal/state"
)

const (
	postgresSSLRequestCode = 80877103
	postgresCancelCode     = 80877102
	postgresProtocolV3     = 196608
)

type PostgresCompatAdapter struct {
	logger    *slog.Logger
	mu        sync.Mutex
	listeners map[string]*postgresAdapterInstance
}

type postgresAdapterInstance struct {
	contract SidecarManagedDBAdapterContract
	listener net.Listener
	cancel   context.CancelFunc
	started  time.Time
}

func NewPostgresCompatAdapter(logger *slog.Logger) *PostgresCompatAdapter {
	return &PostgresCompatAdapter{
		logger:    logger,
		listeners: make(map[string]*postgresAdapterInstance),
	}
}

func (p *PostgresCompatAdapter) Start(ctx context.Context, contract SidecarManagedDBAdapterContract) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := materializeManagedDBAdapterSecret(&contract, strings.TrimSpace(os.Getenv("AGENT_STATE_ENCRYPTION_KEY"))); err != nil {
		return err
	}

	key := postgresAdapterKey(contract)
	if existing, ok := p.listeners[key]; ok {
		p.stopLocked(existing)
		delete(p.listeners, key)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(contract.ListenerHost, fmt.Sprintf("%d", contract.ListenerPort)))
	if err != nil {
		return fmt.Errorf("listen postgres adapter %s: %w", key, err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	instance := &postgresAdapterInstance{
		contract: contract,
		listener: listener,
		cancel:   cancel,
		started:  time.Now().UTC(),
	}
	p.listeners[key] = instance

	go p.acceptLoop(runCtx, instance)
	return nil
}

func (p *PostgresCompatAdapter) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, instance := range p.listeners {
		p.stopLocked(instance)
		delete(p.listeners, key)
	}
}

func (p *PostgresCompatAdapter) acceptLoop(ctx context.Context, instance *postgresAdapterInstance) {
	for {
		conn, err := instance.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				if p.logger != nil {
					p.logger.Warn("postgres compatibility accept failed",
						"alias", instance.contract.Alias,
						"error", err.Error(),
					)
				}
				continue
			}
		}
		go p.handleConn(ctx, conn, instance.contract)
	}
}

func (p *PostgresCompatAdapter) handleConn(ctx context.Context, clientConn net.Conn, contract SidecarManagedDBAdapterContract) {
	defer clientConn.Close()

	params, err := readPostgresStartup(clientConn)
	if err != nil {
		writePostgresError(clientConn, "startup failed")
		return
	}

	upstreamAddr := stripPostgresUpstream(contract.Upstream)
	upstreamConn, err := net.DialTimeout("tcp", upstreamAddr, 10*time.Second)
	if err != nil {
		writePostgresError(clientConn, "upstream unavailable")
		if p.logger != nil {
			p.logger.Warn("postgres compatibility upstream dial failed",
				"alias", contract.Alias,
				"upstream", upstreamAddr,
				"error", err.Error(),
			)
		}
		return
	}
	defer upstreamConn.Close()

	if err := startupPostgresUpstream(upstreamConn, clientConn, params, contract); err != nil {
		writePostgresError(clientConn, "authentication bridge failed")
		if p.logger != nil {
			p.logger.Warn("postgres compatibility startup failed",
				"alias", contract.Alias,
				"error", err.Error(),
			)
		}
		return
	}

	done := make(chan struct{}, 2)
	go proxyBytes(upstreamConn, clientConn, done)
	go proxyBytes(clientConn, upstreamConn, done)

	select {
	case <-ctx.Done():
	case <-done:
	}
}

func (p *PostgresCompatAdapter) stopLocked(instance *postgresAdapterInstance) {
	if instance.cancel != nil {
		instance.cancel()
	}
	if instance.listener != nil {
		_ = instance.listener.Close()
	}
}

func postgresAdapterKey(contract SidecarManagedDBAdapterContract) string {
	return fmt.Sprintf("%s:%s:%d", contract.Alias, contract.ListenerHost, contract.ListenerPort)
}

func stripPostgresUpstream(value string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"tcp://", "http://", "https://"} {
		value = strings.TrimPrefix(value, prefix)
	}
	return value
}

func readPostgresStartup(conn net.Conn) (map[string]string, error) {
	for {
		header := make([]byte, 4)
		if _, err := io.ReadFull(conn, header); err != nil {
			return nil, err
		}
		length := int(binary.BigEndian.Uint32(header))
		if length < 8 {
			return nil, fmt.Errorf("invalid postgres startup length %d", length)
		}
		payload := make([]byte, length-4)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return nil, err
		}

		code := binary.BigEndian.Uint32(payload[:4])
		switch code {
		case postgresSSLRequestCode:
			if _, err := conn.Write([]byte("N")); err != nil {
				return nil, err
			}
			continue
		case postgresCancelCode:
			return nil, fmt.Errorf("postgres cancel requests are not supported by compatibility adapter")
		default:
			return parsePostgresStartupParams(payload[4:])
		}
	}
}

func parsePostgresStartupParams(payload []byte) (map[string]string, error) {
	params := make(map[string]string)
	fields := strings.Split(string(payload), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		key := strings.TrimSpace(fields[i])
		value := fields[i+1]
		if key == "" {
			break
		}
		params[key] = value
	}
	return params, nil
}

func startupPostgresUpstream(upstreamConn net.Conn, clientConn net.Conn, clientParams map[string]string, contract SidecarManagedDBAdapterContract) error {
	password := strings.TrimSpace(contract.UpstreamPassword)
	if password == "" {
		return fmt.Errorf("missing upstream password for postgres compatibility adapter")
	}
	params := make(map[string]string, len(clientParams)+2)
	for key, value := range clientParams {
		params[key] = value
	}
	if user := strings.TrimSpace(contract.UpstreamUsername); user != "" {
		params["user"] = user
	}
	if db := strings.TrimSpace(contract.UpstreamDatabase); db != "" {
		params["database"] = db
	}

	if err := writePostgresStartup(upstreamConn, params); err != nil {
		return err
	}

	for {
		typ, payload, err := readPostgresMessage(upstreamConn)
		if err != nil {
			return err
		}
		if typ != 'R' {
			if err := writePostgresTypedMessage(clientConn, typ, payload); err != nil {
				return err
			}
			if typ == 'Z' {
				return nil
			}
			continue
		}

		if len(payload) < 4 {
			return fmt.Errorf("invalid postgres auth payload")
		}
		authType := binary.BigEndian.Uint32(payload[:4])
		switch authType {
		case 0:
			if err := writePostgresTypedMessage(clientConn, typ, payload); err != nil {
				return err
			}
		case 3:
			if err := writePostgresPasswordMessage(upstreamConn, password); err != nil {
				return err
			}
		case 5:
			if len(payload) < 8 {
				return fmt.Errorf("invalid postgres md5 auth payload")
			}
			passwordDigest := postgresMD5Password(password, params["user"], payload[4:8])
			if err := writePostgresPasswordMessage(upstreamConn, passwordDigest); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported postgres auth type %d", authType)
		}
	}
}

func writePostgresStartup(conn net.Conn, params map[string]string) error {
	body := make([]byte, 0, 128)
	protocol := make([]byte, 4)
	binary.BigEndian.PutUint32(protocol, postgresProtocolV3)
	body = append(body, protocol...)
	for key, value := range params {
		if strings.TrimSpace(key) == "" || value == "" {
			continue
		}
		body = append(body, []byte(key)...)
		body = append(body, 0)
		body = append(body, []byte(value)...)
		body = append(body, 0)
	}
	body = append(body, 0)
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(body)+4))
	_, err := conn.Write(append(header, body...))
	return err
}

func readPostgresMessage(conn net.Conn) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, nil, err
	}
	typ := header[0]
	length := int(binary.BigEndian.Uint32(header[1:5]))
	if length < 4 {
		return 0, nil, fmt.Errorf("invalid postgres message length %d", length)
	}
	payload := make([]byte, length-4)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, nil, err
	}
	return typ, payload, nil
}

func writePostgresTypedMessage(conn net.Conn, typ byte, payload []byte) error {
	frame := make([]byte, 0, len(payload)+5)
	frame = append(frame, typ)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(payload)+4))
	frame = append(frame, length...)
	frame = append(frame, payload...)
	_, err := conn.Write(frame)
	return err
}

func writePostgresPasswordMessage(conn net.Conn, password string) error {
	payload := append([]byte(password), 0)
	return writePostgresTypedMessage(conn, 'p', payload)
}

func writePostgresError(conn net.Conn, message string) {
	payload := []byte("SERROR\x00M" + message + "\x00\x00")
	_ = writePostgresTypedMessage(conn, 'E', payload)
}

func postgresMD5Password(password, user string, salt []byte) string {
	first := md5.Sum([]byte(password + user))
	second := md5.Sum(append([]byte(fmt.Sprintf("%x", first)), salt...))
	return "md5" + fmt.Sprintf("%x", second)
}

func materializeManagedDBAdapterSecret(contract *SidecarManagedDBAdapterContract, stateKey string) error {
	if contract == nil {
		return fmt.Errorf("managed db adapter contract is required")
	}
	if strings.TrimSpace(contract.UpstreamPassword) != "" {
		return nil
	}
	if plain := strings.TrimSpace(contract.UpstreamPasswordPlaintext); plain != "" {
		contract.UpstreamPassword = plain
		return nil
	}
	if encrypted := strings.TrimSpace(contract.UpstreamPasswordEncrypted); encrypted != "" {
		if strings.TrimSpace(stateKey) == "" {
			return fmt.Errorf("state encryption key is required to decrypt managed db adapter secret for %s", contract.Alias)
		}
		plaintext, err := state.DecryptSecret(encrypted, stateKey)
		if err != nil {
			return fmt.Errorf("decrypt managed db adapter secret for %s: %w", contract.Alias, err)
		}
		contract.UpstreamPassword = plaintext
		return nil
	}
	return fmt.Errorf("managed db adapter secret is missing for %s", contract.Alias)
}

func proxyBytes(dst io.Writer, src io.Reader, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	done <- struct{}{}
}
