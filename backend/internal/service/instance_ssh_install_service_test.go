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
		result: SSHExecutionResult{HostKeyFingerprint: "SHA256:abc123"},
	}
	installSvc := NewInstanceSSHInstallService(instanceSvc, sshExec)

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
	if !strings.Contains(sshExec.lastInput.Command, "run -d --name") {
		t.Fatalf("expected container run command, got %q", sshExec.lastInput.Command)
	}
	if !strings.Contains(sshExec.lastInput.Command, "AGENT_BOOTSTRAP_TOKEN") {
		t.Fatalf("expected bootstrap token env in command, got %q", sshExec.lastInput.Command)
	}

	oldRecord := tokenStore.byID["boot_old"]
	if oldRecord == nil || oldRecord.UsedAt == nil {
		t.Fatalf("expected old bootstrap token revoked, got %#v", oldRecord)
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
	bindingSvc := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, newFakeMeshNetworkStore(), newFakeClusterStore())
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
		newFakeClusterStore(),
		nil,
	)

	sshExec := &fakeSSHExecutor{
		result: SSHExecutionResult{HostKeyFingerprint: "SHA256:auto"},
	}
	installSvc := NewInstanceSSHInstallService(instanceSvc, sshExec).WithBootstrapOrchestrator(orchestrator)

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
	if bindings[0].TargetKind != "instance" || bindings[0].TargetID != "inst_1" {
		t.Fatalf("expected auto binding to instance inst_1, got kind=%q id=%q", bindings[0].TargetKind, bindings[0].TargetID)
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
