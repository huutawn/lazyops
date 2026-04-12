package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryRegisterAndGetStandaloneDriver(t *testing.T) {
	reg := NewRegistry()
	driver := NewStandaloneDriver()
	if err := reg.Register(driver); err != nil {
		t.Fatalf("register standalone driver: %v", err)
	}

	got, err := reg.Get(RuntimeModeStandalone)
	if err != nil {
		t.Fatalf("get standalone driver: %v", err)
	}
	if got.Mode() != RuntimeModeStandalone {
		t.Fatalf("expected mode %q, got %q", RuntimeModeStandalone, got.Mode())
	}
}

func TestRegistryRegisterAndGetDistributedMeshDriver(t *testing.T) {
	reg := NewRegistry()
	driver := NewDistributedMeshDriver()
	if err := reg.Register(driver); err != nil {
		t.Fatalf("register distributed-mesh driver: %v", err)
	}

	got, err := reg.Get(RuntimeModeDistributedMesh)
	if err != nil {
		t.Fatalf("get distributed-mesh driver: %v", err)
	}
	if got.Mode() != RuntimeModeDistributedMesh {
		t.Fatalf("expected mode %q, got %q", RuntimeModeDistributedMesh, got.Mode())
	}
}

func TestRegistryRegisterAndGetDistributedK3sDriver(t *testing.T) {
	reg := NewRegistry()
	driver := NewDistributedK3sDriver()
	if err := reg.Register(driver); err != nil {
		t.Fatalf("register distributed-k3s driver: %v", err)
	}

	got, err := reg.Get(RuntimeModeDistributedK3s)
	if err != nil {
		t.Fatalf("get distributed-k3s driver: %v", err)
	}
	if got.Mode() != RuntimeModeDistributedK3s {
		t.Fatalf("expected mode %q, got %q", RuntimeModeDistributedK3s, got.Mode())
	}
}

func TestRegistryRejectsDuplicateRegistration(t *testing.T) {
	reg := NewRegistry()
	driver := NewStandaloneDriver()
	if err := reg.Register(driver); err != nil {
		t.Fatalf("first register: %v", err)
	}

	if err := reg.Register(driver); err == nil {
		t.Fatal("expected error on duplicate registration, got nil")
	}
}

func TestRegistryReturnsErrorForUnknownMode(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("unknown-mode")
	if err == nil {
		t.Fatal("expected error for unknown mode, got nil")
	}
}

func TestRegistryListReturnsAllDrivers(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewStandaloneDriver())
	reg.Register(NewDistributedMeshDriver())
	reg.Register(NewDistributedK3sDriver())

	list := reg.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 drivers, got %d", len(list))
	}
}

func TestIsValidRuntimeMode(t *testing.T) {
	if !IsValidRuntimeMode(RuntimeModeStandalone) {
		t.Error("expected standalone to be valid")
	}
	if !IsValidRuntimeMode(RuntimeModeDistributedMesh) {
		t.Error("expected distributed-mesh to be valid")
	}
	if !IsValidRuntimeMode(RuntimeModeDistributedK3s) {
		t.Error("expected distributed-k3s to be valid")
	}
	if IsValidRuntimeMode("invalid") {
		t.Error("expected invalid mode to be rejected")
	}
}

func TestValidateRuntimeModeError(t *testing.T) {
	err := ValidateRuntimeMode("invalid")
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected error message to contain 'invalid', got %q", err.Error())
	}
}

func TestStandaloneDriverValidateTargetSuccess(t *testing.T) {
	driver := NewStandaloneDriver()
	target := TargetSpec{
		TargetKind:  "instance",
		TargetID:    "inst_123",
		RuntimeMode: RuntimeModeStandalone,
	}
	if err := driver.ValidateTarget(context.Background(), target); err != nil {
		t.Fatalf("expected valid target, got error: %v", err)
	}
}

func TestStandaloneDriverValidateTargetRejectsWrongKind(t *testing.T) {
	driver := NewStandaloneDriver()
	target := TargetSpec{
		TargetKind:  "cluster",
		TargetID:    "cls_123",
		RuntimeMode: RuntimeModeStandalone,
	}
	if err := driver.ValidateTarget(context.Background(), target); err == nil {
		t.Fatal("expected error for wrong target kind")
	}
}

func TestStandaloneDriverValidateTargetRejectsEmptyID(t *testing.T) {
	driver := NewStandaloneDriver()
	target := TargetSpec{
		TargetKind:  "instance",
		TargetID:    "",
		RuntimeMode: RuntimeModeStandalone,
	}
	if err := driver.ValidateTarget(context.Background(), target); err == nil {
		t.Fatal("expected error for empty target_id")
	}
}

func TestDistributedMeshDriverValidateTargetSuccess(t *testing.T) {
	driver := NewDistributedMeshDriver()
	target := TargetSpec{
		TargetKind:  "mesh_network",
		TargetID:    "mesh_123",
		RuntimeMode: RuntimeModeDistributedMesh,
	}
	if err := driver.ValidateTarget(context.Background(), target); err != nil {
		t.Fatalf("expected valid target, got error: %v", err)
	}
}

func TestDistributedMeshDriverValidateTargetRejectsWrongKind(t *testing.T) {
	driver := NewDistributedMeshDriver()
	target := TargetSpec{
		TargetKind:  "instance",
		TargetID:    "inst_123",
		RuntimeMode: RuntimeModeDistributedMesh,
	}
	if err := driver.ValidateTarget(context.Background(), target); err == nil {
		t.Fatal("expected error for wrong target kind")
	}
}

func TestDistributedK3sDriverValidateTargetSuccess(t *testing.T) {
	driver := NewDistributedK3sDriver()
	target := TargetSpec{
		TargetKind:  "cluster",
		TargetID:    "cls_123",
		RuntimeMode: RuntimeModeDistributedK3s,
	}
	if err := driver.ValidateTarget(context.Background(), target); err != nil {
		t.Fatalf("expected valid target, got error: %v", err)
	}
}

func TestDistributedK3sDriverValidateTargetRejectsWrongKind(t *testing.T) {
	driver := NewDistributedK3sDriver()
	target := TargetSpec{
		TargetKind:  "instance",
		TargetID:    "inst_123",
		RuntimeMode: RuntimeModeDistributedK3s,
	}
	if err := driver.ValidateTarget(context.Background(), target); err == nil {
		t.Fatal("expected error for wrong target kind")
	}
}

func TestDistributedK3sDriverGuardK3sBoundaryRejectsDockerCommands(t *testing.T) {
	driver := NewDistributedK3sDriver()
	forbiddenCommands := []string{"docker_run", "docker_stop", "docker_rm", "direct_deploy"}
	for _, cmdType := range forbiddenCommands {
		cmd := AgentCommand{
			Type:      cmdType,
			RequestID: "req_123",
			ProjectID: "prj_123",
			Source:    "test",
		}
		_, err := driver.ExecuteCommand(context.Background(), cmd)
		if err == nil {
			t.Fatalf("expected error for forbidden command %q, got nil", cmdType)
		}
		if !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("expected error message for %q to contain 'forbidden', got %q", cmdType, err.Error())
		}
	}
}

func TestDistributedK3sDriverAllowsValidCommands(t *testing.T) {
	driver := NewDistributedK3sDriver()
	cmd := AgentCommand{
		Type:      "reconcile_revision",
		RequestID: "req_123",
		ProjectID: "prj_123",
		Source:    "test",
	}
	result, err := driver.ExecuteCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected valid command to succeed, got error: %v", err)
	}
	if result.RequestID != cmd.RequestID {
		t.Fatalf("expected request_id %q, got %q", cmd.RequestID, result.RequestID)
	}
}

func TestStandaloneDriverExecuteCommandRejectsEmptyType(t *testing.T) {
	driver := NewStandaloneDriver()
	cmd := AgentCommand{
		Type:      "",
		RequestID: "req_123",
	}
	_, err := driver.ExecuteCommand(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for empty command type")
	}
}

func TestStandaloneDriverExecuteCommandRejectsUnknownType(t *testing.T) {
	driver := NewStandaloneDriver()
	cmd := AgentCommand{
		Type:      "unknown_command",
		RequestID: "req_123",
	}
	_, err := driver.ExecuteCommand(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for unknown command type")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected error to contain 'unsupported', got %q", err.Error())
	}
}

func TestStandaloneDriverExecuteCommandReturnsDriverInfo(t *testing.T) {
	driver := NewStandaloneDriver()
	cmd := AgentCommand{
		Type:      "prepare_release_workspace",
		RequestID: "req_123",
	}
	result, err := driver.ExecuteCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if result.Output["driver"] != "standalone" {
		t.Fatalf("expected driver output to be 'standalone', got %v", result.Output["driver"])
	}
}

func TestDistributedMeshDriverExecuteCommandRejectsUnknownType(t *testing.T) {
	driver := NewDistributedMeshDriver()
	cmd := AgentCommand{
		Type:      "docker_run",
		RequestID: "req_123",
	}
	_, err := driver.ExecuteCommand(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for unknown command type")
	}
}

func TestDistributedMeshDriverExecuteCommandReturnsDriverInfo(t *testing.T) {
	driver := NewDistributedMeshDriver()
	cmd := AgentCommand{
		Type:      "ensure_mesh_peer",
		RequestID: "req_123",
	}
	result, err := driver.ExecuteCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if result.Output["driver"] != "distributed_mesh" {
		t.Fatalf("expected driver output to be 'distributed_mesh', got %v", result.Output["driver"])
	}
}

func TestDistributedK3sDriverExecuteCommandRejectsUnknownType(t *testing.T) {
	driver := NewDistributedK3sDriver()
	cmd := AgentCommand{
		Type:      "docker_run",
		RequestID: "req_123",
	}
	_, err := driver.ExecuteCommand(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for forbidden command type")
	}
}

func TestDistributedK3sDriverExecuteCommandReturnsDriverInfo(t *testing.T) {
	driver := NewDistributedK3sDriver()
	cmd := AgentCommand{
		Type:      "render_gateway_config",
		RequestID: "req_123",
	}
	result, err := driver.ExecuteCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if result.Output["driver"] != "distributed_k3s" {
		t.Fatalf("expected driver output to be 'distributed_k3s', got %v", result.Output["driver"])
	}
}

func TestStandaloneDriverPlanRollout(t *testing.T) {
	driver := NewStandaloneDriver()
	req := RolloutRequest{
		ProjectID:       "prj_123",
		RevisionID:      "rev_123",
		BindingID:       "bind_123",
		RevisionPayload: map[string]any{"image_ref": "ghcr.io/app:v1"},
	}
	plan, err := driver.PlanRollout(context.Background(), req)
	if err != nil {
		t.Fatalf("plan rollout: %v", err)
	}
	if plan.RuntimeMode != RuntimeModeStandalone {
		t.Fatalf("expected runtime mode %q, got %q", RuntimeModeStandalone, plan.RuntimeMode)
	}
	if plan.TargetKind != "instance" {
		t.Fatalf("expected target kind instance, got %q", plan.TargetKind)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected at least one rollout step")
	}
	expected := []string{
		CommandTypePrepareReleaseWorkspace,
		CommandTypeRenderSidecars,
		CommandTypeRenderGatewayConfig,
		CommandTypeReconcileRevision,
		CommandTypeProvisionInternalSvc,
		CommandTypeStartReleaseCandidate,
		CommandTypeRunHealthGate,
		CommandTypePromoteRelease,
	}
	if len(plan.Steps) != len(expected) {
		t.Fatalf("expected %d rollout steps, got %d", len(expected), len(plan.Steps))
	}
	for index, commandType := range expected {
		if plan.Steps[index].Command.Type != commandType {
			t.Fatalf("expected step %d command %q, got %q", index, commandType, plan.Steps[index].Command.Type)
		}
	}
}

func TestDistributedMeshDriverPlanRollout(t *testing.T) {
	driver := NewDistributedMeshDriver()
	req := RolloutRequest{
		ProjectID:       "prj_123",
		RevisionID:      "rev_123",
		BindingID:       "bind_123",
		RevisionPayload: map[string]any{"image_ref": "ghcr.io/app:v1"},
	}
	plan, err := driver.PlanRollout(context.Background(), req)
	if err != nil {
		t.Fatalf("plan rollout: %v", err)
	}
	if plan.RuntimeMode != RuntimeModeDistributedMesh {
		t.Fatalf("expected runtime mode %q, got %q", RuntimeModeDistributedMesh, plan.RuntimeMode)
	}
	if plan.TargetKind != "mesh_network" {
		t.Fatalf("expected target kind mesh_network, got %q", plan.TargetKind)
	}
	expected := []string{
		CommandTypeEnsureMeshPeer,
		CommandTypeSyncOverlayRoutes,
		CommandTypeRenderSidecars,
		CommandTypeRenderGatewayConfig,
		CommandTypeReconcileRevision,
		CommandTypeRunHealthGate,
		CommandTypePromoteRelease,
	}
	if len(plan.Steps) != len(expected) {
		t.Fatalf("expected %d rollout steps, got %d", len(expected), len(plan.Steps))
	}
	for index, commandType := range expected {
		if plan.Steps[index].Command.Type != commandType {
			t.Fatalf("expected step %d command %q, got %q", index, commandType, plan.Steps[index].Command.Type)
		}
	}
}

func TestDistributedK3sDriverPlanRollout(t *testing.T) {
	driver := NewDistributedK3sDriver()
	req := RolloutRequest{
		ProjectID:       "prj_123",
		RevisionID:      "rev_123",
		BindingID:       "bind_123",
		RevisionPayload: map[string]any{"image_ref": "ghcr.io/app:v1"},
	}
	plan, err := driver.PlanRollout(context.Background(), req)
	if err != nil {
		t.Fatalf("plan rollout: %v", err)
	}
	if plan.RuntimeMode != RuntimeModeDistributedK3s {
		t.Fatalf("expected runtime mode %q, got %q", RuntimeModeDistributedK3s, plan.RuntimeMode)
	}
	if plan.TargetKind != "cluster" {
		t.Fatalf("expected target kind cluster, got %q", plan.TargetKind)
	}
	expected := []string{
		CommandTypeRenderGatewayConfig,
		CommandTypeReconcileRevision,
		CommandTypeRunHealthGate,
		CommandTypePromoteRelease,
	}
	if len(plan.Steps) != len(expected) {
		t.Fatalf("expected %d rollout steps, got %d", len(expected), len(plan.Steps))
	}
	for index, commandType := range expected {
		if plan.Steps[index].Command.Type != commandType {
			t.Fatalf("expected step %d command %q, got %q", index, commandType, plan.Steps[index].Command.Type)
		}
	}
}

func TestDriverInfoCapabilities(t *testing.T) {
	tests := []struct {
		driver       RuntimeDriver
		expectedMode string
	}{
		{NewStandaloneDriver(), RuntimeModeStandalone},
		{NewDistributedMeshDriver(), RuntimeModeDistributedMesh},
		{NewDistributedK3sDriver(), RuntimeModeDistributedK3s},
	}

	for _, tt := range tests {
		info := tt.driver.Info()
		if info.Mode != tt.expectedMode {
			t.Errorf("driver %T: expected mode %q, got %q", tt.driver, tt.expectedMode, info.Mode)
		}
		if len(info.Capabilities) == 0 {
			t.Errorf("driver %T: expected non-empty capabilities", tt.driver)
		}
	}
}

func TestIsValidAgentCommand(t *testing.T) {
	if !IsValidAgentCommand("reconcile_revision") {
		t.Error("expected reconcile_revision to be valid")
	}
	if !IsValidAgentCommand("promote_release") {
		t.Error("expected promote_release to be valid")
	}
	if IsValidAgentCommand("unknown_command") {
		t.Error("expected unknown_command to be invalid")
	}
}

func TestIsValidOperatorEvent(t *testing.T) {
	if !IsValidOperatorEvent("deployment.started") {
		t.Error("expected deployment.started to be valid")
	}
	if !IsValidOperatorEvent("deployment.rolled_back") {
		t.Error("expected deployment.rolled_back to be valid")
	}
	if IsValidOperatorEvent("unknown.event") {
		t.Error("expected unknown.event to be invalid")
	}
}

func TestNewCommandEnvelope(t *testing.T) {
	env := NewCommandEnvelope(
		"reconcile_revision",
		"req_123",
		"corr_123",
		"agent_123",
		"prj_123",
		"test_driver",
		map[string]any{"key": "value"},
	)

	if env.Type != "reconcile_revision" {
		t.Fatalf("expected type reconcile_revision, got %q", env.Type)
	}
	if env.RequestID != "req_123" {
		t.Fatalf("expected request_id req_123, got %q", env.RequestID)
	}
	if env.CorrelationID != "corr_123" {
		t.Fatalf("expected correlation_id corr_123, got %q", env.CorrelationID)
	}
	if env.AgentID != "agent_123" {
		t.Fatalf("expected agent_id agent_123, got %q", env.AgentID)
	}
	if env.ProjectID != "prj_123" {
		t.Fatalf("expected project_id prj_123, got %q", env.ProjectID)
	}
	if env.Source != "test_driver" {
		t.Fatalf("expected source test_driver, got %q", env.Source)
	}
	if env.Payload["key"] != "value" {
		t.Fatalf("expected payload key=value, got %v", env.Payload)
	}
}

func TestNewOperatorEvent(t *testing.T) {
	evt := NewOperatorEvent("deployment.started", map[string]any{"deployment_id": "dep_123"}, map[string]any{"source": "test"})

	if evt.Type != "deployment.started" {
		t.Fatalf("expected type deployment.started, got %q", evt.Type)
	}
	if evt.Meta["source"] != "test" {
		t.Fatalf("expected meta source=test, got %v", evt.Meta)
	}
}
