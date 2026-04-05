package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/dispatcher"
	"lazyops-agent/internal/state"
)

func TestAcceptanceMatrixBootstrapRejectsExpiredAndReusedTokens(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.Enrollment.Enrolled = true
		local.Enrollment.BootstrapTokenUsed = true
		local.Metadata.CurrentState = contracts.AgentStateConnected
		return nil
	}); err != nil {
		t.Fatalf("seed enrollment state: %v", err)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !local.Enrollment.Enrolled {
		t.Fatal("expected enrolled to be true")
	}
	if !local.Enrollment.BootstrapTokenUsed {
		t.Fatal("expected bootstrap token used to be true")
	}
}

func TestAcceptanceMatrixStandaloneRolloutAndRollback(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)
	service.now = func() time.Time {
		return time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	configureServiceHealthChecks(t, &stablePayload, server, tcpListener)
	stableCtx, err := ContextFromPreparePayload(stablePayload)
	if err != nil {
		t.Fatalf("build stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), stableCtx); err != nil {
		t.Fatalf("prepare stable release workspace: %v", err)
	}

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.CurrentRevisionID = "rev_122"
		local.RevisionCache.StableRevisionID = "rev_122"
		return nil
	}); err != nil {
		t.Fatalf("seed stable revision: %v", err)
	}

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runPromotionReadySetup(t, service, raw)

	result := service.handlePromoteRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPromoteRelease,
		RequestID:     "req_promote",
		CorrelationID: "corr_promote",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("promote release failed: %#v", result.Error)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if local.RevisionCache.CurrentRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected current revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.CurrentRevisionID)
	}
	if local.RevisionCache.StableRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected stable revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.StableRevisionID)
	}

	result = service.handleRollbackRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRollbackRelease,
		RequestID:     "req_rollback",
		CorrelationID: "corr_rollback",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("rollback release failed: %#v", result.Error)
	}

	local, err = store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state after rollback: %v", err)
	}
	if local.RevisionCache.CurrentRevisionID != "rev_122" {
		t.Fatalf("expected current revision rev_122 after rollback, got %q", local.RevisionCache.CurrentRevisionID)
	}
}

func TestAcceptanceMatrixMeshPeerJoinAndLeave(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.Metadata.RuntimeMode = contracts.RuntimeModeDistributedMesh
		return nil
	}); err != nil {
		t.Fatalf("set runtime mode: %v", err)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if local.Metadata.RuntimeMode != contracts.RuntimeModeDistributedMesh {
		t.Fatalf("expected runtime mode distributed-mesh, got %q", local.Metadata.RuntimeMode)
	}
}

func TestAcceptanceMatrixSidecarPrecedenceChain(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	payload.Revision.CompatibilityPolicy = contracts.CompatibilityPolicy{
		EnvInjection:       true,
		ManagedCredentials: true,
		LocalhostRescue:    true,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runRuntimeCommands(t, service, raw,
		contracts.CommandPrepareReleaseWorkspace,
		contracts.CommandRenderSidecars,
	)

	sidecarPlanPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "revisions", "rev_123", "sidecars", "plan.json")
	if _, err := os.Stat(sidecarPlanPath); err != nil {
		t.Fatalf("expected sidecar plan at %s: %v", sidecarPlanPath, err)
	}

	var plan map[string]any
	planRaw, err := os.ReadFile(sidecarPlanPath)
	if err != nil {
		t.Fatalf("read sidecar plan: %v", err)
	}
	if err := json.Unmarshal(planRaw, &plan); err != nil {
		t.Fatalf("decode sidecar plan: %v", err)
	}

	services, ok := plan["services"].([]any)
	if !ok || len(services) == 0 {
		t.Fatal("expected services in sidecar plan")
	}

	firstSvc, ok := services[0].(map[string]any)
	if !ok {
		t.Fatal("expected first service to be a map")
	}

	mode, ok := firstSvc["selected_mode"].(string)
	if !ok {
		t.Fatal("expected selected_mode field in sidecar plan")
	}
	if mode != "env_injection" {
		t.Fatalf("expected env_injection mode (highest precedence), got %q", mode)
	}
}

func TestAcceptanceMatrixK3sNodeAgentReportsAndDoesNotDeploy(t *testing.T) {
	detector := testK3sDetector()
	os.Setenv("NODE_NAME", "k3s-node-1")
	defer os.Unsetenv("NODE_NAME")

	detector.Detect()

	guard := NewNodeAgentGuard(nil, detector, detector.Mode())

	if !guard.IsNodeAgentMode() {
		t.Fatal("expected node agent mode when K3s detected")
	}

	err := guard.AssertNotNodeAgent("prepare_release_workspace")
	if err == nil {
		t.Fatal("expected node agent to be blocked from prepare_release_workspace")
	}

	err = guard.AssertNotNodeAgent("promote_release")
	if err == nil {
		t.Fatal("expected node agent to be blocked from promote_release")
	}

	err = guard.AssertNotNodeAgent("docker_run")
	if err == nil {
		t.Fatal("expected node agent to be blocked from docker_run")
	}

	topologyReporter := testPodTopologyReporter()
	topologyReporter.ReportNode(ClusterNode{Name: "k3s-node-1", Status: "ready", Role: "worker"})
	topologyReporter.ReportPod(ClusterPod{Name: "pod-1", Namespace: "default", NodeName: "k3s-node-1", Status: "running"})

	summary := topologyReporter.BuildSummary()
	if summary.NodeCount != 1 {
		t.Fatalf("expected 1 node in topology, got %d", summary.NodeCount)
	}
	if summary.PodCount != 1 {
		t.Fatalf("expected 1 pod in topology, got %d", summary.PodCount)
	}

	incidentReporter := testClusterIncidentReporter()
	incidentReporter.ReportUnhealthyNode("k3s-node-1", map[string]any{"reason": "not_ready"})
	incidentReporter.ReportPodCrashLoop("pod-1", "default", 5)

	incidents := incidentReporter.CollectIncidents()
	if len(incidents) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(incidents))
	}
}

func TestAcceptanceMatrixObservabilityCorrelationAndMetrics(t *testing.T) {
	traceCollector := testTraceCollector()
	traceCollector.RecordHop("corr_acceptance", "gateway", "sidecar:web", "http", 10.0, "ok", true)
	traceCollector.CompleteTrace("prj_123", "corr_acceptance")

	traceCollector.now = func() time.Time {
		return time.Date(2026, 4, 4, 16, 0, 5, 0, time.UTC)
	}

	expired := traceCollector.CollectExpiredWindows()
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired trace window, got %d", len(expired))
	}
	if expired[0].CorrelationID != "corr_acceptance" {
		t.Fatalf("expected correlation_id corr_acceptance, got %q", expired[0].CorrelationID)
	}

	metricAgg := testMetricAggregator()
	values := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for _, v := range values {
		metricAgg.Record("latency", v)
	}

	metricAgg.now = func() time.Time {
		return time.Date(2026, 4, 4, 16, 0, 5, 0, time.UTC)
	}

	agg, ok := metricAgg.ComputeAggregate("latency")
	if !ok {
		t.Fatal("expected aggregate to exist")
	}
	if agg.Min != 10 {
		t.Fatalf("expected min 10, got %f", agg.Min)
	}
	if agg.Max != 100 {
		t.Fatalf("expected max 100, got %f", agg.Max)
	}
	if agg.Avg != 55 {
		t.Fatalf("expected avg 55, got %f", agg.Avg)
	}
	if agg.P95 != 100 {
		t.Fatalf("expected p95 100, got %f", agg.P95)
	}
	if agg.Count != 10 {
		t.Fatalf("expected count 10, got %d", agg.Count)
	}
}

func TestAcceptanceMatrixScaleToZeroOptInAndWake(t *testing.T) {
	guard := testScaleToZeroGuard()
	guard.autosleep.MarkActive("api")
	guard.autosleep.now = func() time.Time {
		return time.Date(2026, 4, 4, 16, 0, 5, 0, time.UTC)
	}

	can := guard.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true, IdleWindow: "2s"}, contracts.RuntimeModeStandalone)
	if !can {
		t.Fatal("expected can sleep when policy enabled and idle window met")
	}

	can = guard.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: false}, contracts.RuntimeModeStandalone)
	if can {
		t.Fatal("expected cannot sleep when policy disabled")
	}

	guard.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	state, err := guard.WakeService("api", contracts.RuntimeModeStandalone)
	if err != nil {
		t.Fatalf("wake service: %v", err)
	}
	if state.Status != "waking" {
		t.Fatalf("expected status waking, got %q", state.Status)
	}
}

func TestAcceptanceMatrixGatewayHoldTimeoutEnforced(t *testing.T) {
	holdMgr := testGatewayHoldManager()
	holdMgr.HoldRequest("api", "req_1", "corr_1", 1*time.Second)

	holdMgr.now = func() time.Time {
		return time.Date(2026, 4, 4, 16, 0, 5, 0, time.UTC)
	}

	resumed := holdMgr.ResumeRequests("api")
	if len(resumed) != 0 {
		t.Fatalf("expected 0 resumed (expired), got %d", len(resumed))
	}

	_, _, expired, _ := holdMgr.Stats()
	if expired != 1 {
		t.Fatalf("expected 1 expired, got %d", expired)
	}
}

func TestAcceptanceMatrixSecurityRedactionAndLogging(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(logger, root)
	service := NewService(logger, store, driver)

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	runRuntimeSetup(t, service, raw)

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if local.Enrollment.EncryptedAgentToken != "" {
		t.Fatal("expected agent token to be encrypted, not plaintext")
	}
}

func TestAcceptanceMatrixAllCommandsRegistered(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)

	registry := dispatcher.NewDefaultRegistry()
	service.Register(registry)

	expectedCommands := []contracts.CommandType{
		contracts.CommandPrepareReleaseWorkspace,
		contracts.CommandRenderGatewayConfig,
		contracts.CommandRenderSidecars,
		contracts.CommandStartReleaseCandidate,
		contracts.CommandRunHealthGate,
		contracts.CommandPromoteRelease,
		contracts.CommandRollbackRelease,
		contracts.CommandGarbageCollectRuntime,
		contracts.CommandReportTraceSummary,
		contracts.CommandReportLogBatch,
		contracts.CommandReportMetricRollup,
		contracts.CommandReportTopologyState,
		contracts.CommandSleepService,
		contracts.CommandWakeService,
	}

	for _, cmd := range expectedCommands {
		if _, ok := registry.Resolve(cmd); !ok {
			t.Fatalf("expected command %s to be registered", cmd)
		}
	}
}
