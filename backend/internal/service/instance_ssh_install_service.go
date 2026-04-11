package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	ErrSSHAuthenticationRequired = errors.New("ssh authentication is required")
	ErrSSHConnectionFailed       = errors.New("ssh connection failed")
	ErrSSHExecutionFailed        = errors.New("ssh command execution failed")
)

const (
	defaultSSHPort              = 22
	defaultAgentImage           = "tawn/lazyops-agent:latest"
	defaultAgentContainerName   = "lazyops-agent"
	defaultAgentStateDir        = "/var/lib/lazyops-agent"
	defaultAgentRuntimeRootDir  = "/var/lib/lazyops-runtime"
	defaultAgentRuntimeMode     = "standalone"
	defaultAgentKind            = "instance_agent"
	maxSSHCommandErrorTailBytes = 512
)

type SSHExecutionInput struct {
	Address            string
	Username           string
	Password           string
	PrivateKey         string
	HostKeyFingerprint string
	Command            string
	ConnectionTimeout  time.Duration
}

type SSHExecutionResult struct {
	HostKeyFingerprint string
	Stdout             string
	Stderr             string
}

type SSHExecutor interface {
	Execute(ctx context.Context, input SSHExecutionInput) (SSHExecutionResult, error)
}

type NativeSSHExecutor struct {
}

func NewNativeSSHExecutor() *NativeSSHExecutor {
	return &NativeSSHExecutor{}
}

func (e *NativeSSHExecutor) Execute(ctx context.Context, input SSHExecutionInput) (SSHExecutionResult, error) {
	authMethods, err := sshAuthMethods(input.Password, input.PrivateKey)
	if err != nil {
		return SSHExecutionResult{}, err
	}
	if len(authMethods) == 0 {
		return SSHExecutionResult{}, ErrSSHAuthenticationRequired
	}

	seenFingerprint := ""
	hostKeyCallback := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		seenFingerprint = ssh.FingerprintSHA256(key)
		expected := strings.TrimSpace(input.HostKeyFingerprint)
		if expected == "" {
			return nil
		}
		if normalizeFingerprint(expected) != normalizeFingerprint(seenFingerprint) {
			return fmt.Errorf("host key fingerprint mismatch")
		}
		return nil
	}

	timeout := input.ConnectionTimeout
	if timeout <= 0 {
		timeout = 200 * time.Second
	}

	// NativeSSHExecutor has no logger, so we just set the timeout silently.
	// The caller (InstanceSSHInstallService) logs the attempt.

	clientConfig := &ssh.ClientConfig{
		User:            strings.TrimSpace(input.Username),
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}

	client, err := ssh.Dial("tcp", input.Address, clientConfig)
	if err != nil {
		return SSHExecutionResult{}, fmt.Errorf("%w: %v", ErrSSHConnectionFailed, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return SSHExecutionResult{}, fmt.Errorf("%w: create session: %v", ErrSSHConnectionFailed, err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	runDone := make(chan error, 1)
	go func() {
		runDone <- session.Run(input.Command)
	}()

	select {
	case err := <-runDone:
		if err != nil {
			return SSHExecutionResult{
				HostKeyFingerprint: seenFingerprint,
				Stdout:             stdout.String(),
				Stderr:             stderr.String(),
			}, fmt.Errorf("%w: %s", ErrSSHExecutionFailed, trimErrorTail(stderr.String()))
		}
	case <-ctx.Done():
		_ = client.Close()
		return SSHExecutionResult{}, ctx.Err()
	}

	return SSHExecutionResult{
		HostKeyFingerprint: seenFingerprint,
		Stdout:             stdout.String(),
		Stderr:             stderr.String(),
	}, nil
}

type InstanceSSHInstallService struct {
	instances *InstanceService
	executor  SSHExecutor
	bootstrap *BootstrapOrchestrator
}

func NewInstanceSSHInstallService(instances *InstanceService, executor SSHExecutor) *InstanceSSHInstallService {
	if executor == nil {
		executor = NewNativeSSHExecutor()
	}
	return &InstanceSSHInstallService{
		instances: instances,
		executor:  executor,
	}
}

func (s *InstanceSSHInstallService) WithBootstrapOrchestrator(bootstrap *BootstrapOrchestrator) *InstanceSSHInstallService {
	s.bootstrap = bootstrap
	return s
}

func (s *InstanceSSHInstallService) Install(ctx context.Context, cmd InstallInstanceAgentSSHCommand) (*InstallInstanceAgentSSHResult, error) {
	// Log with timeout marker to confirm new binary is deployed
	slog.Info("ssh_install_starting",
		"instance_id", cmd.InstanceID,
		"host", cmd.Host,
		"port", cmd.Port,
		"username", cmd.Username,
		"ssh_timeout", "200s",
	)
	userID := strings.TrimSpace(cmd.UserID)
	instanceID := strings.TrimSpace(cmd.InstanceID)
	if userID == "" || instanceID == "" {
		return nil, ErrInvalidInput
	}

	host := strings.TrimSpace(cmd.Host)
	username := strings.TrimSpace(cmd.Username)
	controlPlaneURL := strings.TrimSpace(cmd.ControlPlaneURL)
	if host == "" || username == "" || controlPlaneURL == "" {
		return nil, ErrInvalidInput
	}
	port := cmd.Port
	if port <= 0 || port > 65535 {
		port = defaultSSHPort
	}

	if strings.TrimSpace(cmd.Password) == "" && strings.TrimSpace(cmd.PrivateKey) == "" {
		return nil, ErrSSHAuthenticationRequired
	}

	bootstrapIssue, err := s.instances.IssueBootstrapToken(userID, instanceID)
	if err != nil {
		return nil, err
	}

	encryptionKey, err := newStateEncryptionKey()
	if err != nil {
		return nil, err
	}

	command := buildInstallAgentCommand(cmd, bootstrapIssue.Token, encryptionKey, controlPlaneURL)
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	execResult, err := s.executor.Execute(ctx, SSHExecutionInput{
		Address:            address,
		Username:           username,
		Password:           cmd.Password,
		PrivateKey:         cmd.PrivateKey,
		HostKeyFingerprint: cmd.HostKeyFingerprint,
		Command:            command,
	})
	if err != nil {
		return nil, err
	}

	attachedProjectID := ""
	projectID := strings.TrimSpace(cmd.ProjectID)
	if projectID != "" && s.bootstrap != nil {
		if _, autoErr := s.bootstrap.AutoBootstrap(BootstrapAutoCommand{
			RequesterUserID: userID,
			RequesterRole:   RoleViewer,
			ProjectID:       projectID,
			InstanceID:      instanceID,
		}); autoErr == nil {
			attachedProjectID = projectID
		}
	}

	return &InstallInstanceAgentSSHResult{
		InstanceID:         instanceID,
		Bootstrap:          *bootstrapIssue,
		StartedAt:          time.Now().UTC(),
		HostKeyFingerprint: strings.TrimSpace(execResult.HostKeyFingerprint),
		AttachedProjectID:  attachedProjectID,
	}, nil
}

func buildInstallAgentCommand(cmd InstallInstanceAgentSSHCommand, bootstrapToken, encryptionKey, controlPlaneURL string) string {
	agentImage := strings.TrimSpace(cmd.AgentImage)
	if agentImage == "" {
		agentImage = defaultAgentImageFromEnv()
	}
	containerName := strings.TrimSpace(cmd.ContainerName)
	if containerName == "" {
		containerName = defaultAgentContainerName
	}
	stateDir := strings.TrimSpace(cmd.StateDir)
	if stateDir == "" {
		stateDir = defaultAgentStateDir
	}
	runtimeRoot := strings.TrimSpace(cmd.ContainerRuntimeRootDir)
	if runtimeRoot == "" {
		runtimeRoot = defaultAgentRuntimeRootDir
	}
	runtimeMode := strings.TrimSpace(cmd.RuntimeMode)
	if runtimeMode == "" {
		runtimeMode = defaultAgentRuntimeMode
	}
	agentKind := strings.TrimSpace(cmd.AgentKind)
	if agentKind == "" {
		agentKind = defaultAgentKind
	}
	targetRef := strings.TrimSpace(cmd.InstanceID)
	if targetRef == "" {
		targetRef = "remote-instance"
	}

	dockerBootstrap := strings.Join([]string{
		`if [ "$(id -u)" -eq 0 ]; then SUDO=""; elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then SUDO="sudo -n"; else SUDO=""; fi;`,
		`STATE_DIR=` + shellQuote(stateDir) + `;`,
		`RUNTIME_ROOT=` + shellQuote(runtimeRoot) + `;`,
		`if [ "$(id -u)" -ne 0 ] && [ -z "$SUDO" ]; then STATE_DIR="${HOME:-/tmp}/.lazyops-agent"; RUNTIME_ROOT="${HOME:-/tmp}/.lazyops-runtime"; fi;`,
		`if ! command -v docker >/dev/null 2>&1; then`,
		`  if command -v apt-get >/dev/null 2>&1; then`,
		`    if [ -n "$SUDO" ]; then $SUDO apt-get update -y >/dev/null 2>&1 || $SUDO apt-get update >/dev/null 2>&1; DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y docker.io >/dev/null 2>&1 || DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y docker-ce docker-ce-cli containerd.io >/dev/null 2>&1; else apt-get update -y >/dev/null 2>&1 || apt-get update >/dev/null 2>&1; DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io >/dev/null 2>&1 || DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce docker-ce-cli containerd.io >/dev/null 2>&1; fi;`,
		`  elif command -v dnf >/dev/null 2>&1; then`,
		`    if [ -n "$SUDO" ]; then $SUDO dnf install -y docker >/dev/null 2>&1; else dnf install -y docker >/dev/null 2>&1; fi;`,
		`  elif command -v yum >/dev/null 2>&1; then`,
		`    if [ -n "$SUDO" ]; then $SUDO yum install -y docker >/dev/null 2>&1; else yum install -y docker >/dev/null 2>&1; fi;`,
		`  else`,
		`    echo 'docker_not_found_and_pkg_manager_unsupported' >&2; exit 1;`,
		`  fi;`,
		`fi;`,
		`if command -v systemctl >/dev/null 2>&1; then if [ -n "$SUDO" ]; then $SUDO systemctl enable --now docker >/dev/null 2>&1 || true; else systemctl enable --now docker >/dev/null 2>&1 || true; fi; fi;`,
		`if command -v service >/dev/null 2>&1; then if [ -n "$SUDO" ]; then $SUDO service docker start >/dev/null 2>&1 || true; else service docker start >/dev/null 2>&1 || true; fi; fi;`,
		`docker_exec() { if docker info >/dev/null 2>&1; then docker "$@"; elif [ -n "$SUDO" ] && $SUDO docker info >/dev/null 2>&1; then $SUDO docker "$@"; else echo 'docker_daemon_unavailable_or_permission_denied' >&2; exit 1; fi; };`,
	}, " ")

	return fmt.Sprintf(
		"set -eu; "+
			"%s "+
			"docker_exec rm -f %s >/dev/null 2>&1 || true; "+
			"docker_exec pull %s >/dev/null 2>&1 || true; "+
			"if [ -n \"$SUDO\" ]; then $SUDO mkdir -p \"$STATE_DIR\" \"$RUNTIME_ROOT\"; $SUDO chmod 0777 \"$STATE_DIR\" \"$RUNTIME_ROOT\"; else mkdir -p \"$STATE_DIR\" \"$RUNTIME_ROOT\"; chmod 0777 \"$STATE_DIR\" \"$RUNTIME_ROOT\"; fi; "+
			"docker_exec run -d --name %s --restart unless-stopped --network host --privileged "+
			"--user root "+
			"-v /var/run/docker.sock:/var/run/docker.sock "+
			"-v \"$STATE_DIR\":\"$STATE_DIR\" -v \"$RUNTIME_ROOT\":\"$RUNTIME_ROOT\" "+
			"-e AGENT_BOOTSTRAP_TOKEN=%s "+
			"-e AGENT_STATE_ENCRYPTION_KEY=%s "+
			"-e AGENT_CONTROL_PLANE_URL=%s "+
			"-e AGENT_RUNTIME_MODE=%s "+
			"-e AGENT_KIND=%s "+
			"-e AGENT_TARGET_REF=%s "+
			"-e AGENT_STATE_DIR=\"$STATE_DIR\" "+
			"-e AGENT_RUNTIME_ROOT_DIR=\"$RUNTIME_ROOT\" "+
			"%s >/dev/null",
		dockerBootstrap,
		shellQuote(containerName),
		shellQuote(agentImage),
		shellQuote(containerName),
		shellQuote(bootstrapToken),
		shellQuote(encryptionKey),
		shellQuote(controlPlaneURL),
		shellQuote(runtimeMode),
		shellQuote(agentKind),
		shellQuote(targetRef),
		shellQuote(agentImage),
	)
}

func defaultAgentImageFromEnv() string {
	if configured := strings.TrimSpace(os.Getenv("AGENT_DEFAULT_IMAGE")); configured != "" {
		return configured
	}
	return defaultAgentImage
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func newStateEncryptionKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func sshAuthMethods(password, privateKey string) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 2)
	if strings.TrimSpace(password) != "" {
		methods = append(methods, ssh.Password(password))
	}
	if strings.TrimSpace(privateKey) != "" {
		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			return nil, ErrInvalidInput
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	return methods, nil
}

func trimErrorTail(stderr string) string {
	text := strings.TrimSpace(stderr)
	if text == "" {
		return "remote command failed"
	}
	if len(text) <= maxSSHCommandErrorTailBytes {
		return text
	}
	return text[len(text)-maxSSHCommandErrorTailBytes:]
}

func normalizeFingerprint(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimPrefix(raw, "sha256:")
	return raw
}
