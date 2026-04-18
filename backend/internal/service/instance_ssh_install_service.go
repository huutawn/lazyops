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
	ErrK3sBootstrapIncomplete    = errors.New("k3s bootstrap did not complete all required stages")
	ErrClusterRegistrationFailed = errors.New("cluster registration failed")
)

const (
	defaultSSHPort              = 22
	defaultAgentImage           = "tawn/lazyops-agent:latest"
	defaultAgentContainerName   = "lazyops-agent"
	defaultAgentStateDir        = "/var/lib/lazyops-agent"
	defaultAgentRuntimeRootDir  = "/var/lib/lazyops-agent/runtime"
	defaultAgentRuntimeMode     = "distributed-k3s"
	defaultAgentKind            = "node_agent"
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
	clusters  *ClusterService
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

func (s *InstanceSSHInstallService) WithClusterService(clusters *ClusterService) *InstanceSSHInstallService {
	s.clusters = clusters
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

	runtimeMode, agentKind, err := normalizeK3sInstallProfile(cmd.RuntimeMode, cmd.AgentKind)
	if err != nil {
		return nil, err
	}

	targetRef := instanceID
	bootstrapIssue, err := s.instances.IssueBootstrapTokenWithProfile(userID, instanceID, BootstrapTokenProfile{
		RuntimeMode: runtimeMode,
		AgentKind:   agentKind,
		TargetRef:   targetRef,
	})
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

	metadata := parseInstallBootstrapMetadata(execResult.Stdout)
	if !metadata.completed() {
		return nil, ErrK3sBootstrapIncomplete
	}

	clusterID := ""
	clusterName := metadata.ClusterName
	clusterStatus := ClusterStatusReady
	if clusterName == "" {
		clusterName = defaultManagedClusterName(instanceID)
	}
	if metadata.NodeAgentReady == false {
		clusterStatus = ClusterStatusDegraded
	}
	if s.clusters != nil {
		clusterSummary, clusterErr := s.clusters.UpsertManagedFromBootstrap(UpsertManagedClusterCommand{
			UserID:              userID,
			InstanceID:          instanceID,
			Name:                clusterName,
			Provider:            "k3s",
			KubeconfigSecretRef: buildManagedClusterSecretRef(clusterName),
			PublicIP:            firstNonEmpty(metadata.PublicIP, normalizeRemotePublicHost(host)),
			Status:              clusterStatus,
		})
		if clusterErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrClusterRegistrationFailed, clusterErr)
		}
		clusterID = clusterSummary.ID
		clusterName = clusterSummary.Name
		clusterStatus = clusterSummary.Status
	}

	attachedProjectID := ""
	projectID := strings.TrimSpace(cmd.ProjectID)
	if projectID != "" && s.bootstrap != nil && clusterID != "" {
		if _, autoErr := s.bootstrap.AutoBootstrap(BootstrapAutoCommand{
			RequesterUserID: userID,
			RequesterRole:   RoleViewer,
			ProjectID:       projectID,
			ClusterID:       clusterID,
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
		ClusterID:          clusterID,
		ClusterName:        clusterName,
		ClusterStatus:      clusterStatus,
		TargetKind:         "cluster",
		RuntimeMode:        runtimeMode,
		Stages:             buildInstallBootstrapStages(metadata, clusterID != ""),
	}, nil
}

type installBootstrapMetadata struct {
	ClusterName     string
	PublicIP        string
	KubeconfigB64   string
	K3sInstalled    bool
	KubeconfigReady bool
	NodeAgentReady  bool
}

func (m installBootstrapMetadata) completed() bool {
	return m.K3sInstalled && m.KubeconfigReady && m.NodeAgentReady && strings.TrimSpace(m.ClusterName) != "" && strings.TrimSpace(m.KubeconfigB64) != ""
}

func parseInstallBootstrapMetadata(stdout string) installBootstrapMetadata {
	meta := installBootstrapMetadata{}
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "LAZYOPS_BOOTSTRAP_STAGE=k3s_installed":
			meta.K3sInstalled = true
		case line == "LAZYOPS_BOOTSTRAP_STAGE=kubeconfig_captured":
			meta.KubeconfigReady = true
		case line == "LAZYOPS_BOOTSTRAP_STAGE=node_agent_ready":
			meta.NodeAgentReady = true
		case strings.HasPrefix(line, "LAZYOPS_CLUSTER_NAME="):
			meta.ClusterName = strings.TrimSpace(strings.TrimPrefix(line, "LAZYOPS_CLUSTER_NAME="))
		case strings.HasPrefix(line, "LAZYOPS_PUBLIC_IP="):
			meta.PublicIP = strings.TrimSpace(strings.TrimPrefix(line, "LAZYOPS_PUBLIC_IP="))
		case strings.HasPrefix(line, "LAZYOPS_KUBECONFIG_B64="):
			meta.KubeconfigB64 = strings.TrimSpace(strings.TrimPrefix(line, "LAZYOPS_KUBECONFIG_B64="))
		}
	}
	return meta
}

func buildInstallBootstrapStages(metadata installBootstrapMetadata, clusterRegistered bool) []InstallBootstrapStageRecord {
	stages := []InstallBootstrapStageRecord{
		{
			ID:      "k3s_installed",
			State:   stageState(metadata.K3sInstalled),
			Message: stageMessage(metadata.K3sInstalled, "K3s server installed and node is ready", "K3s server bootstrap is incomplete"),
		},
		{
			ID:      "kubeconfig_captured",
			State:   stageState(metadata.KubeconfigReady && strings.TrimSpace(metadata.KubeconfigB64) != ""),
			Message: stageMessage(metadata.KubeconfigReady && strings.TrimSpace(metadata.KubeconfigB64) != "", "Kubeconfig captured from remote K3s host", "Remote kubeconfig was not captured"),
		},
		{
			ID:      "cluster_registered",
			State:   stageState(clusterRegistered),
			Message: stageMessage(clusterRegistered, "Cluster target registered in backend", "Cluster target registration failed"),
		},
		{
			ID:      "node_agent_ready",
			State:   stageState(metadata.NodeAgentReady),
			Message: stageMessage(metadata.NodeAgentReady, "LazyOps node agent DaemonSet is available", "LazyOps node agent DaemonSet is not ready"),
		},
	}
	return stages
}

func stageState(done bool) string {
	if done {
		return "completed"
	}
	return "failed"
}

func stageMessage(done bool, completed, failed string) string {
	if done {
		return completed
	}
	return failed
}

func normalizeK3sInstallProfile(runtimeMode, agentKind string) (string, string, error) {
	mode := strings.TrimSpace(runtimeMode)
	if mode == "" {
		mode = defaultAgentRuntimeMode
	}
	kind := strings.TrimSpace(agentKind)
	if kind == "" {
		kind = defaultAgentKind
	}
	if mode != "distributed-k3s" || kind != "node_agent" {
		return "", "", ErrInvalidInput
	}
	return mode, kind, nil
}

func buildManagedClusterSecretRef(clusterName string) string {
	clusterName = normalizeBindingTargetRef(clusterName)
	if clusterName == "" {
		clusterName = "lazyops-k3s"
	}
	return "secret://clusters/" + clusterName + "/kubeconfig"
}

func defaultManagedClusterName(instanceID string) string {
	base := normalizeBindingTargetRef(instanceID)
	if base == "" {
		base = "bootstrap"
	}
	return "k3s-" + base
}

func normalizeRemotePublicHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		parsedHost, _, splitErr := net.SplitHostPort(host)
		if splitErr == nil {
			return parsedHost
		}
	}
	return host
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func buildInstallAgentCommand(cmd InstallInstanceAgentSSHCommand, bootstrapToken, encryptionKey, controlPlaneURL string) string {
	agentImage := strings.TrimSpace(cmd.AgentImage)
	if agentImage == "" {
		agentImage = defaultAgentImageFromEnv()
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

	clusterName := defaultManagedClusterName(targetRef)
	publicHost := normalizeRemotePublicHost(cmd.Host)
	if publicHost == "" {
		publicHost = targetRef
	}

	script := fmt.Sprintf(`set -eu
if [ "$(id -u)" -eq 0 ]; then SUDO=""; elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then SUDO="sudo -n"; else SUDO=""; fi
STATE_DIR=%s
RUNTIME_ROOT=%s
PUBLIC_HOST=%s
BOOTSTRAP_TOKEN=%s
ENCRYPTION_KEY=%s
CONTROL_PLANE_URL=%s
AGENT_IMAGE=%s
RUNTIME_MODE=%s
AGENT_KIND=%s
TARGET_REF=%s
CLUSTER_NAME=%s
if [ "$(id -u)" -ne 0 ] && [ -z "$SUDO" ]; then STATE_DIR="${HOME:-/tmp}/.lazyops-agent"; RUNTIME_ROOT="${STATE_DIR}/runtime"; fi
if ! command -v curl >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then if [ -n "$SUDO" ]; then $SUDO apt-get update -y >/dev/null 2>&1 || $SUDO apt-get update >/dev/null 2>&1; DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y curl >/dev/null 2>&1; else apt-get update -y >/dev/null 2>&1 || apt-get update >/dev/null 2>&1; DEBIAN_FRONTEND=noninteractive apt-get install -y curl >/dev/null 2>&1; fi; elif command -v dnf >/dev/null 2>&1; then if [ -n "$SUDO" ]; then $SUDO dnf install -y curl >/dev/null 2>&1; else dnf install -y curl >/dev/null 2>&1; fi; elif command -v yum >/dev/null 2>&1; then if [ -n "$SUDO" ]; then $SUDO yum install -y curl >/dev/null 2>&1; else yum install -y curl >/dev/null 2>&1; fi; else echo 'curl_not_found_and_pkg_manager_unsupported' >&2; exit 1; fi
fi
if ! command -v k3s >/dev/null 2>&1; then
  if [ -n "$SUDO" ]; then curl -sfL https://get.k3s.io | $SUDO sh -s - server --write-kubeconfig-mode 644 >/dev/null 2>&1; else curl -sfL https://get.k3s.io | sh -s - server --write-kubeconfig-mode 644 >/dev/null 2>&1; fi
fi
if command -v systemctl >/dev/null 2>&1; then if [ -n "$SUDO" ]; then $SUDO systemctl enable --now k3s >/dev/null 2>&1 || true; else systemctl enable --now k3s >/dev/null 2>&1 || true; fi; fi
kctl() { if [ -n "$SUDO" ]; then $SUDO kubectl --kubeconfig /etc/rancher/k3s/k3s.yaml "$@"; else kubectl --kubeconfig /etc/rancher/k3s/k3s.yaml "$@"; fi; }
READY=0
for _i in $(seq 1 90); do
  if kctl get nodes >/dev/null 2>&1; then READY=1; break; fi
  sleep 2
done
if [ "$READY" -ne 1 ]; then echo 'k3s_node_not_ready' >&2; exit 1; fi
printf 'LAZYOPS_BOOTSTRAP_STAGE=k3s_installed\n'
if [ -n "$SUDO" ]; then $SUDO mkdir -p "$STATE_DIR" "$RUNTIME_ROOT"; $SUDO chmod 0777 "$STATE_DIR" "$RUNTIME_ROOT"; else mkdir -p "$STATE_DIR" "$RUNTIME_ROOT"; chmod 0777 "$STATE_DIR" "$RUNTIME_ROOT"; fi
KUBECONFIG_RAW=$(if [ -n "$SUDO" ]; then $SUDO cat /etc/rancher/k3s/k3s.yaml; else cat /etc/rancher/k3s/k3s.yaml; fi)
if [ -n "$PUBLIC_HOST" ]; then KUBECONFIG_RAW=$(printf '%%s' "$KUBECONFIG_RAW" | sed "s#https://127.0.0.1:6443#https://$PUBLIC_HOST:6443#g"); fi
KUBECONFIG_B64=$(printf '%%s' "$KUBECONFIG_RAW" | base64 | tr -d '\n')
printf 'LAZYOPS_CLUSTER_NAME=%%s\n' "$CLUSTER_NAME"
printf 'LAZYOPS_PUBLIC_IP=%%s\n' "$PUBLIC_HOST"
printf 'LAZYOPS_KUBECONFIG_B64=%%s\n' "$KUBECONFIG_B64"
printf 'LAZYOPS_BOOTSTRAP_STAGE=kubeconfig_captured\n'
cat <<EOF >/tmp/lazyops-bootstrap.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: lazyops-system
  labels:
    app.kubernetes.io/part-of: lazyops
    app.kubernetes.io/managed-by: lazyops
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: lazyops-node-agent
  namespace: lazyops-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lazyops-node-agent
rules:
  - apiGroups: [""]
    resources: ["namespaces", "services", "configmaps", "secrets", "persistentvolumeclaims", "pods", "pods/log", "events"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: lazyops-node-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: lazyops-node-agent
subjects:
  - kind: ServiceAccount
    name: lazyops-node-agent
    namespace: lazyops-system
---
apiVersion: v1
kind: Secret
metadata:
  name: lazyops-node-agent-env
  namespace: lazyops-system
type: Opaque
stringData:
  AGENT_APP_NAME: "lazyops-node-agent"
  AGENT_APP_ENV: "production"
  AGENT_RUNTIME_MODE: "%s"
  AGENT_KIND: "%s"
  AGENT_TARGET_REF: "%s"
  AGENT_BOOTSTRAP_TOKEN: "%s"
  AGENT_STATE_ENCRYPTION_KEY: "%s"
  AGENT_CONTROL_PLANE_URL: "%s"
  AGENT_STATE_DIR: "%s"
  AGENT_RUNTIME_ROOT_DIR: "%s"
  AGENT_KUBECTL_BIN: "kubectl"
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: lazyops-node-agent
  namespace: lazyops-system
  labels:
    app.kubernetes.io/name: lazyops-node-agent
    app.kubernetes.io/part-of: lazyops
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: lazyops-node-agent
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lazyops-node-agent
        app.kubernetes.io/part-of: lazyops
    spec:
      serviceAccountName: lazyops-node-agent
      tolerations:
        - operator: Exists
      containers:
        - name: agent
          image: "%s"
          imagePullPolicy: Always
          envFrom:
            - secretRef:
                name: lazyops-node-agent-env
          volumeMounts:
            - name: runtime-root
              mountPath: "%s"
      volumes:
        - name: runtime-root
          hostPath:
            path: "%s"
            type: DirectoryOrCreate
EOF
kctl apply -f /tmp/lazyops-bootstrap.yaml >/dev/null
if kctl -n lazyops-system rollout status daemonset/lazyops-node-agent --timeout=180s >/dev/null 2>&1; then printf 'LAZYOPS_BOOTSTRAP_STAGE=node_agent_ready\n'; else echo 'lazyops_node_agent_not_ready' >&2; exit 1; fi
`, shellQuote(stateDir), shellQuote(runtimeRoot), shellQuote(publicHost), shellQuote(bootstrapToken), shellQuote(encryptionKey), shellQuote(controlPlaneURL), shellQuote(agentImage), shellQuote(runtimeMode), shellQuote(agentKind), shellQuote(targetRef), shellQuote(clusterName), runtimeMode, agentKind, targetRef, bootstrapToken, encryptionKey, controlPlaneURL, stateDir, runtimeRoot, agentImage, stateDir, stateDir)

	return script
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
