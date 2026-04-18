package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeSSHExecutor struct {
	lastInput SSHExecutionInput
	result    SSHExecutionResult
	err       error
}

func (f *fakeSSHExecutor) Execute(_ context.Context, input SSHExecutionInput) (SSHExecutionResult, error) {
	f.lastInput = input
	if f.err != nil {
		return SSHExecutionResult{}, f.err
	}
	return f.result, nil
}

func successfulK3sBootstrapStdout(clusterName, publicIP string) string {
	return strings.Join([]string{
		"LAZYOPS_BOOTSTRAP_STAGE=k3s_installed",
		"LAZYOPS_CLUSTER_NAME=" + clusterName,
		"LAZYOPS_PUBLIC_IP=" + publicIP,
		"LAZYOPS_KUBECONFIG_B64=YXBpVmVyc2lvbjogdjE=",
		"LAZYOPS_BOOTSTRAP_STAGE=kubeconfig_captured",
		"LAZYOPS_BOOTSTRAP_STAGE=node_agent_ready",
	}, "\n")
}

func TestInstanceSSHInstallServiceRejectsMissingAuth(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-1",
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	tokenStore := newFakeBootstrapTokenStore()
	instanceSvc := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())
	sshExec := &fakeSSHExecutor{}
	installSvc := NewInstanceSSHInstallService(instanceSvc, sshExec)

	_, err := installSvc.Install(context.Background(), InstallInstanceAgentSSHCommand{
		UserID:          "usr_1",
		InstanceID:      "inst_1",
		Host:            "203.0.113.10",
		Port:            22,
		Username:        "root",
		ControlPlaneURL: "http://control.example:8080",
	})
	if !errors.Is(err, ErrSSHAuthenticationRequired) {
		t.Fatalf("expected ErrSSHAuthenticationRequired, got %v", err)
	}
}

func TestInstanceSSHInstallServiceIssuesTokenAndExecutesCommand(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-1",
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	tokenStore := newFakeBootstrapTokenStore(&models.BootstrapToken{
		ID:         "boot_old",
		UserID:     "usr_1",
		InstanceID: "inst_1",
		TokenHash:  "hash_old",
		ExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
	})
	instanceSvc := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())
	sshExec := &fakeSSHExecutor{
		result: SSHExecutionResult{
			HostKeyFingerprint: "SHA256:abc123",
			Stdout:             successfulK3sBootstrapStdout("k3s-inst-1", "203.0.113.10"),
		},
	}
	installSvc := NewInstanceSSHInstallService(instanceSvc, sshExec).WithClusterService(NewClusterService(newFakeClusterStore()))

	result, err := installSvc.Install(context.Background(), InstallInstanceAgentSSHCommand{
		UserID:             "usr_1",
		InstanceID:         "inst_1",
		Host:               "203.0.113.10",
		Port:               22,
		Username:           "root",
		Password:           "secret",
		ControlPlaneURL:    "http://control.example:8080",
		HostKeyFingerprint: "SHA256:abc123",
		AgentImage:         "tawn/lazyops-agent:test",
	})
	if err != nil {
		t.Fatalf("install via ssh: %v", err)
	}

	if result.InstanceID != "inst_1" {
		t.Fatalf("expected instance id inst_1, got %q", result.InstanceID)
	}
	if !strings.HasPrefix(result.Bootstrap.Token, "lop_boot_") {
		t.Fatalf("expected lop_boot_ token, got %q", result.Bootstrap.Token)
	}
	if strings.TrimSpace(result.HostKeyFingerprint) == "" {
		t.Fatal("expected host key fingerprint in result")
	}
	if result.TargetKind != "cluster" || result.RuntimeMode != "distributed-k3s" {
		t.Fatalf("expected cluster/distributed-k3s result, got kind=%q runtime=%q", result.TargetKind, result.RuntimeMode)
	}
	if result.ClusterID == "" || result.ClusterName == "" {
		t.Fatalf("expected cluster registration in result, got id=%q name=%q", result.ClusterID, result.ClusterName)
	}
	if !strings.Contains(sshExec.lastInput.Command, "https://get.k3s.io") {
		t.Fatalf("expected k3s bootstrap command, got %q", sshExec.lastInput.Command)
	}
	if !strings.Contains(sshExec.lastInput.Command, "AGENT_BOOTSTRAP_TOKEN") {
		t.Fatalf("expected bootstrap token env in command, got %q", sshExec.lastInput.Command)
	}
	if !strings.Contains(sshExec.lastInput.Command, "kind: DaemonSet") {
		t.Fatalf("expected node agent daemonset manifest in command, got %q", sshExec.lastInput.Command)
	}

	oldRecord := tokenStore.byID["boot_old"]
	if oldRecord == nil || oldRecord.UsedAt == nil {
		t.Fatalf("expected old bootstrap token revoked, got %#v", oldRecord)
	}
	if oldRecord.ExpectedRuntimeMode != "" || oldRecord.ExpectedAgentKind != "" {
		t.Fatalf("expected legacy token profile to remain empty, got %#v", oldRecord)
	}
	newRecord, err := tokenStore.GetByHash(hashOpaqueToken(result.Bootstrap.Token))
	if err != nil {
		t.Fatalf("load new bootstrap token: %v", err)
	}
	if newRecord == nil {
		t.Fatal("expected new bootstrap token to be stored")
	}
	if newRecord.ExpectedRuntimeMode != "distributed-k3s" || newRecord.ExpectedAgentKind != "node_agent" {
		t.Fatalf("expected k3s node bootstrap token profile, got %#v", newRecord)
	}
}

func TestInstanceSSHInstallServicePropagatesExecutorError(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-1",
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	tokenStore := newFakeBootstrapTokenStore()
	instanceSvc := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())
	sshExec := &fakeSSHExecutor{err: ErrSSHConnectionFailed}
	installSvc := NewInstanceSSHInstallService(instanceSvc, sshExec)

	_, err := installSvc.Install(context.Background(), InstallInstanceAgentSSHCommand{
		UserID:          "usr_1",
		InstanceID:      "inst_1",
		Host:            "203.0.113.10",
		Port:            22,
		Username:        "root",
		Password:        "secret",
		ControlPlaneURL: "http://control.example:8080",
	})
	if !errors.Is(err, ErrSSHConnectionFailed) {
		t.Fatalf("expected ErrSSHConnectionFailed, got %v", err)
	}
}

func TestInstanceSSHInstallServiceAutoAttachesProjectBinding(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_1",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_1",
		UserID:                  "usr_1",
		Name:                    "edge-1",
		Status:                  "pending_enrollment",
		LabelsJSON:              "{}",
		RuntimeCapabilitiesJSON: "{}",
	})
	tokenStore := newFakeBootstrapTokenStore()
	instanceSvc := NewInstanceService(instanceStore, tokenStore, testEnrollmentConfig())
	bindingStore := newFakeDeploymentBindingStore()
	clusterStore := newFakeClusterStore()
	bindingSvc := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, newFakeMeshNetworkStore(), clusterStore)
	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		newFakeProjectRepoLinkStore(),
		bindingSvc,
		bindingStore,
		newFakeDeploymentStore(),
		instanceStore,
		newFakeMeshNetworkStore(),
		clusterStore,
		nil,
	)

	sshExec := &fakeSSHExecutor{
		result: SSHExecutionResult{
			HostKeyFingerprint: "SHA256:auto",
			Stdout:             successfulK3sBootstrapStdout("k3s-inst-1", "203.0.113.10"),
		},
	}
	installSvc := NewInstanceSSHInstallService(instanceSvc, sshExec).
		WithBootstrapOrchestrator(orchestrator).
		WithClusterService(NewClusterService(clusterStore))

	result, err := installSvc.Install(context.Background(), InstallInstanceAgentSSHCommand{
		UserID:          "usr_1",
		ProjectID:       "prj_1",
		InstanceID:      "inst_1",
		Host:            "203.0.113.10",
		Port:            22,
		Username:        "root",
		Password:        "secret",
		ControlPlaneURL: "http://control.example:8080",
	})
	if err != nil {
		t.Fatalf("install via ssh: %v", err)
	}
	if result.AttachedProjectID != "prj_1" {
		t.Fatalf("expected attached project id prj_1, got %q", result.AttachedProjectID)
	}

	bindings, err := bindingStore.ListByProject("prj_1")
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected one auto binding, got %d", len(bindings))
	}
	if bindings[0].TargetKind != "cluster" || bindings[0].TargetID == "" {
		t.Fatalf("expected auto binding to cluster target, got kind=%q id=%q", bindings[0].TargetKind, bindings[0].TargetID)
	}
}

func TestBuildInstallAgentCommandUsesDefaultAgentImage(t *testing.T) {
	command := buildInstallAgentCommand(InstallInstanceAgentSSHCommand{
		InstanceID: "inst_1",
	}, "lop_boot_123", "enc_key_123", "http://control.example:8080")

	if !strings.Contains(command, defaultAgentImage) {
		t.Fatalf("expected default image %q in command, got %q", defaultAgentImage, command)
	}
}

func TestBuildInstallAgentCommandRespectsConfiguredDefaultAgentImage(t *testing.T) {
	t.Setenv("AGENT_DEFAULT_IMAGE", "tawn/lazyops-agent:stable")

	command := buildInstallAgentCommand(InstallInstanceAgentSSHCommand{
		InstanceID: "inst_1",
	}, "lop_boot_123", "enc_key_123", "http://control.example:8080")

	if !strings.Contains(command, "tawn/lazyops-agent:stable") {
		t.Fatalf("expected configured default image in command, got %q", command)
	}
}

func TestBuildInstallAgentCommandResetsStateBeforeRun(t *testing.T) {
	command := buildInstallAgentCommand(InstallInstanceAgentSSHCommand{
		InstanceID: "inst_1",
	}, "lop_boot_123", "enc_key_123", "http://control.example:8080")

	if !strings.Contains(command, "lazyops-node-agent-env") {
		t.Fatalf("expected command to provision node agent secret before run, got %q", command)
	}
}
