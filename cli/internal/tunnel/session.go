package tunnel

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	DefaultTunnelTimeout = 30 * time.Minute
	MinTunnelTimeout     = 5 * time.Minute
	MaxTunnelTimeout     = 2 * time.Hour
	DefaultDBPort        = 15432
	DefaultDBRemote      = "localhost:5432"
	MinUserPort          = 1024
	MaxUserPort          = 65535
)

type TunnelType string

const (
	TypeDB  TunnelType = "db"
	TypeTCP TunnelType = "tcp"
)

func (t TunnelType) String() string {
	return string(t)
}

func ParseTunnelType(raw string) (TunnelType, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "db":
		return TypeDB, nil
	case "tcp":
		return TypeTCP, nil
	default:
		return "", fmt.Errorf("unknown tunnel type %q. next: use `db` or `tcp`", raw)
	}
}

type Config struct {
	Type      TunnelType
	LocalPort int
	Remote    string
	Timeout   time.Duration
	ProjectID string
	TargetRef string
}

func (c Config) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("tunnel type is required. next: use `db` or `tcp`")
	}
	if c.LocalPort < MinUserPort || c.LocalPort > MaxUserPort {
		return fmt.Errorf("local port %d is out of the allowed range (%d-%d). next: choose a different port with `--port`", c.LocalPort, MinUserPort, MaxUserPort)
	}
	if strings.TrimSpace(c.Remote) == "" {
		return fmt.Errorf("remote target is required for %s tunnel. next: specify the remote address with `--remote`", c.Type)
	}
	if c.Timeout < MinTunnelTimeout || c.Timeout > MaxTunnelTimeout {
		return fmt.Errorf("tunnel timeout %v is out of the allowed range (%v-%v). next: adjust `--timeout`", c.Timeout, MinTunnelTimeout, MaxTunnelTimeout)
	}
	if strings.TrimSpace(c.ProjectID) == "" {
		return fmt.Errorf("project id is required to create a tunnel session. next: run this command from a repo with a valid lazyops.yaml")
	}
	return nil
}

func DefaultConfig(tunnelType TunnelType) Config {
	cfg := Config{
		Type:    tunnelType,
		Timeout: DefaultTunnelTimeout,
	}
	switch tunnelType {
	case TypeDB:
		cfg.LocalPort = DefaultDBPort
		cfg.Remote = DefaultDBRemote
	case TypeTCP:
		cfg.LocalPort = 19090
		cfg.Remote = "localhost:8080"
	}
	return cfg
}

type Session struct {
	ID        string
	Type      TunnelType
	LocalPort int
	Remote    string
	Status    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewSessionFromContract(sessionID string, tunnelType TunnelType, localPort int, remote string, status string, timeout time.Duration) Session {
	now := time.Now()
	return Session{
		ID:        sessionID,
		Type:      tunnelType,
		LocalPort: localPort,
		Remote:    remote,
		Status:    status,
		CreatedAt: now,
		ExpiresAt: now.Add(timeout),
	}
}

type PortChecker interface {
	IsPortAvailable(port int) error
}

type DefaultPortChecker struct{}

func (DefaultPortChecker) IsPortAvailable(port int) error {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("local port %d is not available. next: choose a different port with `--port` or stop the process using port %d", port, port)
	}
	listener.Close()
	return nil
}

type Manager struct {
	mu          sync.Mutex
	sessions    map[string]Session
	portChecker PortChecker
}

func NewManager() *Manager {
	return &Manager{
		sessions:    make(map[string]Session),
		portChecker: DefaultPortChecker{},
	}
}

func NewManagerWithPortChecker(portChecker PortChecker) *Manager {
	return &Manager{
		sessions:    make(map[string]Session),
		portChecker: portChecker,
	}
}

func (m *Manager) Create(cfg Config) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := cfg.Validate(); err != nil {
		return Session{}, err
	}

	if err := m.portChecker.IsPortAvailable(cfg.LocalPort); err != nil {
		return Session{}, err
	}

	for _, existing := range m.sessions {
		if existing.LocalPort == cfg.LocalPort && existing.Status == "active" {
			return Session{}, fmt.Errorf("local port %d is already used by tunnel session %s. next: stop the existing tunnel or choose a different port with `--port`", cfg.LocalPort, existing.ID)
		}
	}

	return Session{
		Type:      cfg.Type,
		LocalPort: cfg.LocalPort,
		Remote:    cfg.Remote,
		Status:    "pending",
	}, nil
}

func (m *Manager) Register(session Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
}

func (m *Manager) Stop(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("tunnel session %q not found. next: list active tunnels or start a new one", sessionID)
	}
	if session.Status != "active" {
		return fmt.Errorf("tunnel session %q is not active (status=%s). next: start a new tunnel session", sessionID, session.Status)
	}

	session.Status = "stopped"
	m.sessions[sessionID] = session
	return nil
}

func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, session := range m.sessions {
		if session.Status == "active" && now.After(session.ExpiresAt) {
			session.Status = "expired"
			m.sessions[id] = session
		}
	}
}

func (m *Manager) ActiveSessions() []Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	var active []Session
	for _, session := range m.sessions {
		if session.Status == "active" {
			active = append(active, session)
		}
	}
	return active
}

func DebugWarningMessage(tunnelType TunnelType) string {
	return fmt.Sprintf("this is a debug tunnel (%s) and must not be used as a production access pattern. next: close this tunnel when debugging is complete", tunnelType)
}
