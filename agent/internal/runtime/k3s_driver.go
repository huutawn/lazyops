package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type K3sDriver struct {
	logger         *slog.Logger
	root           string
	kubectlBin     string
	kubeconfigPath string
	logCollector   *LogCollector
	now            func() time.Time

	mu           sync.Mutex
	logTailStops map[string]context.CancelFunc
}

const (
	k3sLogTailInitialLines   = 20
	k3sLogTailResumeLines    = 200
	k3sLogTailScannerMaxSize = 1024 * 1024
	k3sLogTailResumeOverlap  = 2 * time.Second
	k3sLogTailRetryBackoff   = 2 * time.Second
)

type kubectlApplySummary struct {
	Created    int
	Configured int
	Patched    int
	Deleted    int
	ServerSide int
	Unchanged  int
	Unknown    int
}

func NewK3sDriver(logger *slog.Logger, root, kubectlBin, kubeconfigPath string) *K3sDriver {
	bin := strings.TrimSpace(kubectlBin)
	if bin == "" {
		bin = "kubectl"
	}
	return &K3sDriver{
		logger:         logger,
		root:           root,
		kubectlBin:     bin,
		kubeconfigPath: strings.TrimSpace(kubeconfigPath),
		now: func() time.Time {
			return time.Now().UTC()
		},
		logTailStops: make(map[string]context.CancelFunc),
	}
}

func (d *K3sDriver) WithLogCollector(collector *LogCollector) *K3sDriver {
	if d == nil {
		return d
	}
	d.logCollector = collector
	return d
}

func (d *K3sDriver) PrepareReleaseWorkspace(_ context.Context, runtimeCtx RuntimeContext) (PreparedWorkspace, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	for _, path := range []string{
		layout.Root,
		layout.Artifacts,
		layout.Config,
		layout.Sidecars,
		layout.Gateway,
		layout.Mesh,
		layout.Services,
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return PreparedWorkspace{}, err
		}
	}

	if err := writeJSON(filepath.Join(layout.Config, "project.json"), runtimeCtx.Project); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Config, "binding.json"), runtimeCtx.Binding); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Config, "revision.json"), runtimeCtx.Revision); err != nil {
		return PreparedWorkspace{}, err
	}

	manifestPath := filepath.Join(layout.Config, "k8s-manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(strings.TrimSpace(runtimeCtx.Revision.ManifestBundle.CombinedYAML)+"\n"), 0o644); err != nil {
		return PreparedWorkspace{}, err
	}
	if strings.TrimSpace(runtimeCtx.Revision.ManifestBundle.RollbackYAML) != "" {
		if err := os.WriteFile(filepath.Join(layout.Config, "k8s-rollback.yaml"), []byte(strings.TrimSpace(runtimeCtx.Revision.ManifestBundle.RollbackYAML)+"\n"), 0o644); err != nil {
			return PreparedWorkspace{}, err
		}
	}

	manifest := WorkspaceManifest{
		PreparedAt: d.now(),
		Project:    runtimeCtx.Project,
		ProjectEnv: cloneStringMap(runtimeCtx.ProjectEnv),
		Binding:    runtimeCtx.Binding,
		Revision:   runtimeCtx.Revision,
		Services:   runtimeCtx.Services,
		Layout:     layout,
	}
	if err := writeJSON(filepath.Join(layout.Root, "workspace.json"), manifest); err != nil {
		return PreparedWorkspace{}, err
	}

	return PreparedWorkspace{
		Layout:       layout,
		ManifestPath: manifestPath,
	}, nil
}

func (d *K3sDriver) ReconcileRevision(ctx context.Context, runtimeCtx RuntimeContext) (ReconcileRevisionResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	manifestPath := filepath.Join(layout.Config, "k8s-manifest.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		prepared, prepErr := d.PrepareReleaseWorkspace(ctx, runtimeCtx)
		if prepErr != nil {
			return ReconcileRevisionResult{}, prepErr
		}
		manifestPath = prepared.ManifestPath
	}

	appliedSteps := []string{"write_manifest_bundle"}
	preflight, err := validateK3sManifestPreflight(runtimeCtx)
	if err != nil {
		return ReconcileRevisionResult{}, wrapK3sError("k8s_manifest_preflight_failed", err, false)
	}
	if len(preflight.Warnings) > 0 {
		appliedSteps = append(appliedSteps, fmt.Sprintf("manifest_preflight:warning:%d", len(preflight.Warnings)))
	} else {
		appliedSteps = append(appliedSteps, "manifest_preflight:passed")
	}

	applyPlan, err := d.materializeApplyManifests(layout.Config, runtimeCtx)
	if err != nil {
		return ReconcileRevisionResult{}, err
	}
	if applyPlan.NamespacePath != "" {
		if _, err := d.kubectlOutput(ctx, "", "apply", "-f", applyPlan.NamespacePath); err != nil {
			return ReconcileRevisionResult{}, wrapK3sError("k8s_namespace_apply_failed", err, true)
		}
		appliedSteps = append(appliedSteps, "kubectl_apply_namespace")
		if err := d.waitForNamespace(ctx, runtimeCtx.Project.Namespace, 20*time.Second); err != nil {
			return ReconcileRevisionResult{}, wrapK3sError("k8s_namespace_ready_failed", err, true)
		}
		appliedSteps = append(appliedSteps, "namespace_ready")
	}

	applyManifestPath := manifestPath
	if applyPlan.ResourcesPath != "" {
		applyManifestPath = applyPlan.ResourcesPath
	}
	if _, err := d.kubectlOutput(ctx, runtimeCtx.Project.Namespace, "apply", "--dry-run=server", "-f", applyManifestPath); err != nil {
		return ReconcileRevisionResult{}, wrapK3sError("k8s_dry_run_failed", err, true)
	}
	appliedSteps = append(appliedSteps, "kubectl_apply_dry_run")

	applyOutput, err := d.kubectlOutput(ctx, runtimeCtx.Project.Namespace, "apply", "-f", applyManifestPath)
	if err != nil {
		return ReconcileRevisionResult{}, wrapK3sError("k8s_apply_failed", err, true)
	}
	applySummary := summarizeKubectlApplyOutput(applyOutput)
	switch {
	case applySummary.totalMutations() == 0 && applySummary.Unchanged > 0:
		appliedSteps = append(appliedSteps, fmt.Sprintf("kubectl_apply:idempotent:%d", applySummary.Unchanged))
	case applySummary.totalChanges() > 0:
		appliedSteps = append(appliedSteps, fmt.Sprintf("kubectl_apply:changed:%d", applySummary.totalChanges()))
	default:
		appliedSteps = append(appliedSteps, "kubectl_apply")
	}

	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		if err := d.runKubectl(ctx, runtimeCtx.Project.Namespace, "rollout", "status", fmt.Sprintf("deployment/%s", spec.Name), "--timeout=120s"); err != nil {
			tolerated, toleratedProgress := d.shouldTolerateRolloutStatusFailure(ctx, runtimeCtx.Project.Namespace, spec.Name, err)
			if !tolerated {
				return ReconcileRevisionResult{}, wrapK3sError("k8s_rollout_failed", err, true)
			}
			appliedSteps = append(appliedSteps, "rollout_status:tolerated:"+spec.Name)
			if toleratedProgress.Status != "" {
				appliedSteps = append(appliedSteps, "rollout_status:"+spec.Name+":"+toleratedProgress.Status)
			}
			continue
		}
		appliedSteps = append(appliedSteps, "rollout_status:"+spec.Name)
	}

	portObservations := d.collectPortApplyObservations(ctx, runtimeCtx)
	portMismatchCount := countPortApplyMismatches(portObservations)
	if len(portObservations) > 0 {
		if portMismatchCount > 0 {
			appliedSteps = append(appliedSteps, fmt.Sprintf("port_telemetry:mismatch:%d", portMismatchCount))
		} else {
			appliedSteps = append(appliedSteps, "port_telemetry:verified")
		}
	}

	rolloutProgress := d.collectRolloutProgress(ctx, runtimeCtx)
	if len(rolloutProgress) > 0 {
		appliedSteps = append(appliedSteps, fmt.Sprintf("rollout_progress:services:%d", len(rolloutProgress)))
	}

	ingressObservations := d.collectIngressObservations(ctx, runtimeCtx)
	if len(ingressObservations) > 0 {
		readyIngresses := 0
		for _, item := range ingressObservations {
			if item.Ready {
				readyIngresses++
			}
		}
		appliedSteps = append(appliedSteps, fmt.Sprintf("ingress_observation:ready:%d/%d", readyIngresses, len(ingressObservations)))
	}

	d.ensureLiveLogTails(runtimeCtx)

	summary := fmt.Sprintf("applied %d k3s rollout steps in namespace %s", len(appliedSteps), runtimeCtx.Project.Namespace)
	if applySummary.totalMutations() == 0 && applySummary.Unchanged > 0 {
		summary = fmt.Sprintf("%s (idempotent re-apply detected)", summary)
	}
	if portMismatchCount > 0 {
		summary = fmt.Sprintf("%s with %d port mismatch warnings", summary, portMismatchCount)
	}

	return ReconcileRevisionResult{
		RevisionID:        runtimeCtx.Revision.RevisionID,
		AppliedSteps:      appliedSteps,
		Summary:           summary,
		PreflightWarnings: preflight.Warnings,
		PortObservations:  portObservations,
		PortMismatchCount: portMismatchCount,
		RolloutProgress:   rolloutProgress,
		Ingresses:         ingressObservations,
		CompletedAt:       d.now(),
	}, nil
}

type k3sApplyManifestPlan struct {
	NamespacePath string
	ResourcesPath string
}

func (d *K3sDriver) materializeApplyManifests(configDir string, runtimeCtx RuntimeContext) (k3sApplyManifestPlan, error) {
	plan := k3sApplyManifestPlan{}
	if len(runtimeCtx.Revision.ManifestBundle.Documents) == 0 {
		plan.ResourcesPath = filepath.Join(configDir, "k8s-manifest.yaml")
		return plan, nil
	}

	namespaceDocs := make([]string, 0, 1)
	resourceDocs := make([]string, 0, len(runtimeCtx.Revision.ManifestBundle.Documents))
	for _, item := range runtimeCtx.Revision.ManifestBundle.Documents {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Kind), "Namespace") {
			namespaceDocs = append(namespaceDocs, content)
			continue
		}
		resourceDocs = append(resourceDocs, content)
	}

	if len(namespaceDocs) > 0 {
		plan.NamespacePath = filepath.Join(configDir, "k8s-manifest-namespaces.yaml")
		if err := os.WriteFile(plan.NamespacePath, []byte(joinManifestDocuments(namespaceDocs)), 0o644); err != nil {
			return k3sApplyManifestPlan{}, err
		}
	}
	if len(resourceDocs) > 0 {
		plan.ResourcesPath = filepath.Join(configDir, "k8s-manifest-resources.yaml")
		if err := os.WriteFile(plan.ResourcesPath, []byte(joinManifestDocuments(resourceDocs)), 0o644); err != nil {
			return k3sApplyManifestPlan{}, err
		}
	}
	return plan, nil
}

func joinManifestDocuments(documents []string) string {
	trimmed := make([]string, 0, len(documents))
	for _, item := range documents {
		if value := strings.TrimSpace(item); value != "" {
			trimmed = append(trimmed, value)
		}
	}
	if len(trimmed) == 0 {
		return ""
	}
	return strings.Join(trimmed, "\n---\n") + "\n"
}

func (d *K3sDriver) waitForNamespace(ctx context.Context, namespace string, timeout time.Duration) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil
	}
	deadline := d.now().Add(timeout)
	for {
		output, err := d.kubectlOutput(ctx, "", "get", "namespace", namespace, "-o", "name")
		if err == nil && strings.TrimSpace(string(output)) != "" {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("namespace %q was not ready before timeout", namespace)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (d *K3sDriver) RenderGatewayConfig(_ context.Context, runtimeCtx RuntimeContext) (GatewayRenderResult, error) {
	manifestPath := filepath.Join(workspaceLayout(d.root, runtimeCtx).Config, "k8s-manifest.yaml")
	publicURLs := make([]string, 0, len(runtimeCtx.Revision.PublicDomains))
	for _, item := range runtimeCtx.Revision.PublicDomains {
		if strings.TrimSpace(item.FallbackURL) != "" {
			publicURLs = append(publicURLs, strings.TrimSpace(item.FallbackURL))
		}
	}
	return GatewayRenderResult{
		Version:        runtimeCtx.Revision.RevisionID,
		PlanPath:       manifestPath,
		ConfigPath:     manifestPath,
		LivePlanPath:   manifestPath,
		LiveConfigPath: manifestPath,
		ActivationPath: manifestPath,
		PublicURLs:     publicURLs,
		Plan:           GatewayPlan{Provider: "traefik", PublicServices: publicServiceNames(runtimeCtx.Services)},
		Activation: GatewayActivation{
			Version:    runtimeCtx.Revision.RevisionID,
			PlanPath:   manifestPath,
			ConfigPath: manifestPath,
			AppliedAt:  d.now(),
		},
	}, nil
}

func (d *K3sDriver) RenderSidecars(_ context.Context, runtimeCtx RuntimeContext) (SidecarRenderResult, error) {
	return SidecarRenderResult{
		Version:        runtimeCtx.Revision.RevisionID,
		PlanPath:       filepath.Join(workspaceLayout(d.root, runtimeCtx).Config, "k8s-manifest.yaml"),
		ConfigPath:     filepath.Join(workspaceLayout(d.root, runtimeCtx).Config, "k8s-manifest.yaml"),
		LivePlanPath:   filepath.Join(workspaceLayout(d.root, runtimeCtx).Config, "k8s-manifest.yaml"),
		LiveConfigRoot: filepath.Join(workspaceLayout(d.root, runtimeCtx).Config),
		ActivationPath: filepath.Join(workspaceLayout(d.root, runtimeCtx).Config, "k8s-manifest.yaml"),
		Services:       []string{},
		Plan:           SidecarPlan{EnabledServices: []string{}},
		Activation: SidecarActivation{
			Version:    runtimeCtx.Revision.RevisionID,
			PlanPath:   filepath.Join(workspaceLayout(d.root, runtimeCtx).Config, "k8s-manifest.yaml"),
			ConfigPath: filepath.Join(workspaceLayout(d.root, runtimeCtx).Config, "k8s-manifest.yaml"),
			AppliedAt:  d.now(),
		},
	}, nil
}

func (d *K3sDriver) ProvisionInternalServices(_ context.Context, request ProvisionInternalServicesRequest) (ProvisionInternalServicesResult, error) {
	return ProvisionInternalServicesResult{
		ProjectID: request.ProjectID,
		BindingID: request.BindingID,
		Summary:   "internal services are managed through k3s manifests",
		AppliedAt: d.now(),
	}, nil
}

func (d *K3sDriver) StartReleaseCandidate(_ context.Context, runtimeCtx RuntimeContext) (CandidateRecord, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	return CandidateRecord{
		RevisionID:       runtimeCtx.Revision.RevisionID,
		WorkspaceRoot:    layout.Root,
		State:            CandidateStatePrepared,
		StartedAt:        d.now(),
		ManifestPath:     filepath.Join(layout.Config, "k8s-manifest.yaml"),
		LastTransitionAt: d.now(),
	}, nil
}

func (d *K3sDriver) RunHealthGate(ctx context.Context, runtimeCtx RuntimeContext) (HealthGateResult, error) {
	results := make([]ServiceHealthResult, 0, len(runtimeCtx.Revision.ServiceSpecs))
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		results = append(results, d.runK3sServiceHealthGateCheck(ctx, runtimeCtx.Project.Namespace, spec))
	}

	passed := true
	failingServices := make([]string, 0)
	for _, result := range results {
		if result.Passed {
			continue
		}
		passed = false
		failingServices = append(failingServices, result.ServiceName)
	}

	summary := fmt.Sprintf("health gate passed for %d/%d services; candidate is promotable", countHealthyServices(results), len(results))
	policy := HealthGatePolicyPromoteCandidate
	candidateState := CandidateStatePromotable
	if !passed {
		summary = fmt.Sprintf("health gate failed for %d/%d services: %s", len(failingServices), len(results), strings.Join(failingServices, ", "))
		policy = HealthGatePolicyRollbackRelease
		candidateState = CandidateStateFailed
	}
	return HealthGateResult{
		RevisionID:     runtimeCtx.Revision.RevisionID,
		CandidateState: candidateState,
		Promotable:     passed,
		PolicyAction:   policy,
		Summary:        summary,
		CheckedAt:      d.now(),
		Services:       results,
	}, nil
}

func (d *K3sDriver) runK3sServiceHealthGateCheck(ctx context.Context, namespace string, spec contracts.K3sServiceSpecPayload) ServiceHealthResult {
	checkedAt := d.now()
	result := ServiceHealthResult{
		ServiceName: spec.Name,
		Protocol:    k3sHealthGateProtocol(spec),
		Address:     fmt.Sprintf("%s.%s.svc.cluster.local:%d", spec.Name, namespace, effectiveK3sHealthGatePort(spec)),
		Path:        spec.HealthCheck.Path,
		Attempts:    1,
		Successes:   1,
		Failures:    0,
		Passed:      true,
		CheckedAt:   checkedAt,
		Message:     "deployment rollout is healthy",
	}

	err := d.runKubectl(ctx, namespace, "rollout", "status", fmt.Sprintf("deployment/%s", spec.Name), "--timeout=60s")
	if err == nil {
		podsHealthy, podMessage := d.verifyK3sServicePodsHealthy(ctx, namespace, spec.Name)
		if !podsHealthy {
			result.Passed = false
			result.Successes = 0
			result.Failures = 1
			result.Message = podMessage
			return result
		}
		result.Message = firstNonEmptyK3s(strings.TrimSpace(podMessage), result.Message)
		return result
	}

	tolerated, progress := d.shouldTolerateRolloutStatusFailure(ctx, namespace, spec.Name, err)
	if tolerated {
		podsHealthy, podMessage := d.verifyK3sServicePodsHealthy(ctx, namespace, spec.Name)
		if !podsHealthy {
			result.Passed = false
			result.Successes = 0
			result.Failures = 1
			result.Message = firstNonEmptyK3s(strings.TrimSpace(podMessage), formatK3sHealthGateFailure(err, progress))
			return result
		}
		result.Message = joinNonEmptyK3s(
			"; ",
			strings.TrimSpace(progress.Message),
			strings.TrimSpace(podMessage),
			"deployment rollout timeout tolerated after readiness verification",
		)
		return result
	}

	result.Passed = false
	result.Successes = 0
	result.Failures = 1
	result.Message = formatK3sHealthGateFailure(err, progress)
	return result
}

func (d *K3sDriver) verifyK3sServicePodsHealthy(ctx context.Context, namespace, serviceName string) (bool, string) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return false, "service name is required to verify pod health"
	}

	podJSON, err := d.kubectlOutput(
		ctx,
		namespace,
		"get",
		"pods",
		"-l",
		fmt.Sprintf("app.kubernetes.io/name=%s", serviceName),
		"-o",
		"json",
	)
	if err != nil {
		return false, fmt.Sprintf("collect pod status failed: %s", err.Error())
	}
	return evaluateK3sServicePodsHealth(serviceName, podJSON)
}

func evaluateK3sServicePodsHealth(serviceName string, podJSON []byte) (bool, string) {
	var pods podListState
	if err := json.Unmarshal(podJSON, &pods); err != nil {
		return false, fmt.Sprintf("decode pod status failed: %s", err.Error())
	}

	activePods := make([]podState, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if strings.TrimSpace(pod.Metadata.DeletionTimestamp) != "" {
			continue
		}
		activePods = append(activePods, pod)
	}
	if len(activePods) == 0 {
		return false, fmt.Sprintf("service %s has no active pods", strings.TrimSpace(serviceName))
	}

	notReady := make([]string, 0)
	readyCount := 0
	for _, pod := range activePods {
		ready, fatal, detail := summarizeK3sPodHealth(pod)
		if fatal {
			return false, detail
		}
		if !ready {
			notReady = append(notReady, detail)
			continue
		}
		readyCount++
	}

	if len(notReady) > 0 {
		return false, fmt.Sprintf(
			"service %s has active pods that are not ready: %s",
			strings.TrimSpace(serviceName),
			strings.Join(notReady, ", "),
		)
	}

	return true, fmt.Sprintf(
		"deployment rollout is healthy; %d/%d active pods ready",
		readyCount,
		len(activePods),
	)
}

func summarizeK3sPodHealth(pod podState) (ready bool, fatal bool, detail string) {
	podName := firstNonEmptyK3s(strings.TrimSpace(pod.Metadata.Name), "unknown-pod")
	phase := firstNonEmptyK3s(strings.TrimSpace(pod.Status.Phase), "Unknown")

	if strings.EqualFold(phase, "Failed") {
		return false, true, fmt.Sprintf("pod %s is in Failed phase", podName)
	}

	for _, status := range pod.Status.InitContainerStatuses {
		if fatal, detail := summarizeK3sContainerFailure(podName, status); fatal {
			return false, true, detail
		}
		if !status.Ready && status.State.Terminated == nil {
			return false, false, fmt.Sprintf("pod %s init container %s is not ready", podName, firstNonEmptyK3s(strings.TrimSpace(status.Name), "unknown"))
		}
	}

	if len(pod.Status.ContainerStatuses) == 0 {
		return false, false, fmt.Sprintf("pod %s has no container status yet (phase=%s)", podName, phase)
	}

	if !strings.EqualFold(phase, "Running") {
		return false, false, fmt.Sprintf("pod %s is in %s phase", podName, phase)
	}

	for _, status := range pod.Status.ContainerStatuses {
		if fatal, detail := summarizeK3sContainerFailure(podName, status); fatal {
			return false, true, detail
		}
		if !status.Ready {
			return false, false, fmt.Sprintf(
				"pod %s container %s is not ready",
				podName,
				firstNonEmptyK3s(strings.TrimSpace(status.Name), "unknown"),
			)
		}
	}

	return true, false, fmt.Sprintf("pod %s is ready", podName)
}

func summarizeK3sContainerFailure(podName string, status podContainerStatus) (bool, string) {
	containerName := firstNonEmptyK3s(strings.TrimSpace(status.Name), "unknown")

	if status.State.Waiting != nil {
		reason := strings.TrimSpace(status.State.Waiting.Reason)
		if isFatalK3sContainerWaitingReason(reason) {
			return true, fmt.Sprintf(
				"pod %s container %s is waiting in %s (restarts=%d)",
				podName,
				containerName,
				firstNonEmptyK3s(reason, "Waiting"),
				status.RestartCount,
			)
		}
	}

	if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
		reason := strings.TrimSpace(status.State.Terminated.Reason)
		return true, fmt.Sprintf(
			"pod %s container %s terminated with exit code %d (%s)",
			podName,
			containerName,
			status.State.Terminated.ExitCode,
			firstNonEmptyK3s(reason, "terminated"),
		)
	}

	return false, ""
}

func isFatalK3sContainerWaitingReason(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "crashloopbackoff",
		"imagepullbackoff",
		"errimagepull",
		"createcontainerconfigerror",
		"createcontainererror",
		"runcontainererror",
		"starterror",
		"invalidimagename":
		return true
	default:
		return false
	}
}

func formatK3sHealthGateFailure(err error, progress RolloutProgress) string {
	parts := make([]string, 0, 3)
	if err != nil {
		parts = append(parts, strings.TrimSpace(err.Error()))
	}
	if progress.Status != "" && progress.Status != "unverified" {
		message := strings.TrimSpace(progress.Message)
		if message != "" {
			parts = append(parts, fmt.Sprintf("rollout %s: %s", progress.Status, message))
		} else {
			parts = append(parts, fmt.Sprintf("rollout %s", progress.Status))
		}
	}
	if progress.DesiredReplicas > 0 {
		parts = append(parts, fmt.Sprintf(
			"replicas desired=%d ready=%d updated=%d available=%d",
			progress.DesiredReplicas,
			progress.ReadyReplicas,
			progress.UpdatedReplicas,
			progress.AvailableReplicas,
		))
	}
	return firstNonEmptyK3s(strings.Join(parts, "; "), "deployment rollout health gate failed")
}

func effectiveK3sHealthGatePort(spec contracts.K3sServiceSpecPayload) int {
	return firstPositiveK3s(
		spec.ServicePort,
		spec.TargetPort,
		spec.HealthCheck.Port,
		defaultK3sServicePort(spec.Kind),
	)
}

func k3sHealthGateProtocol(spec contracts.K3sServiceSpecPayload) string {
	if protocol := strings.ToLower(strings.TrimSpace(spec.HealthCheck.Protocol)); protocol != "" {
		return protocol
	}
	switch strings.ToLower(strings.TrimSpace(spec.Kind)) {
	case "postgres", "mysql", "redis", "rabbitmq":
		return "tcp"
	default:
		return "http"
	}
}

func defaultK3sServicePort(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "redis":
		return 6379
	case "rabbitmq":
		return 5672
	default:
		return 8080
	}
}

func (d *K3sDriver) PromoteRelease(_ context.Context, runtimeCtx RuntimeContext) (PromoteReleaseResult, error) {
	publicURLs := make([]string, 0, len(runtimeCtx.Revision.PublicDomains))
	for _, item := range runtimeCtx.Revision.PublicDomains {
		if strings.TrimSpace(item.FallbackURL) != "" {
			publicURLs = append(publicURLs, strings.TrimSpace(item.FallbackURL))
		}
	}
	summary := PromotionSummary{
		ProjectID:  runtimeCtx.Project.ProjectID,
		BindingID:  runtimeCtx.Binding.BindingID,
		RevisionID: runtimeCtx.Revision.RevisionID,
		Summary:    "k3s deployment promoted by applied manifest reconciliation",
		PublicURLs: publicURLs,
		PromotedAt: d.now(),
	}
	return PromoteReleaseResult{
		RevisionID:    runtimeCtx.Revision.RevisionID,
		ZeroDowntime:  true,
		RollbackReady: strings.TrimSpace(runtimeCtx.Revision.ManifestBundle.RollbackYAML) != "",
		Summary:       summary,
		Traffic: TrafficShiftRecord{
			ActiveRevisionID: runtimeCtx.Revision.RevisionID,
			StableRevisionID: runtimeCtx.Revision.RevisionID,
			ZeroDowntime:     true,
			RollbackReady:    strings.TrimSpace(runtimeCtx.Revision.ManifestBundle.RollbackYAML) != "",
			ShiftedAt:        d.now(),
		},
		DrainPlan: DrainPlan{
			PromotedRevisionID: runtimeCtx.Revision.RevisionID,
			Status:             "not_required",
			ZeroDowntime:       true,
			CleanupPolicy:      "kubernetes_reconciliation",
			StartedAt:          d.now(),
		},
	}, nil
}

func (d *K3sDriver) RollbackRelease(ctx context.Context, runtimeCtx RuntimeContext) (RollbackReleaseResult, error) {
	rollbackYAML := strings.TrimSpace(runtimeCtx.Revision.ManifestBundle.RollbackYAML)
	if rollbackYAML == "" {
		return RollbackReleaseResult{}, wrapK3sError("k8s_rollback_bundle_missing", fmt.Errorf("rollback manifest bundle is empty"), false)
	}

	layout := workspaceLayout(d.root, runtimeCtx)
	rollbackPath := filepath.Join(layout.Config, "k8s-rollback.yaml")
	if err := os.MkdirAll(layout.Config, 0o755); err != nil {
		return RollbackReleaseResult{}, err
	}
	if err := os.WriteFile(rollbackPath, []byte(rollbackYAML+"\n"), 0o644); err != nil {
		return RollbackReleaseResult{}, err
	}
	rollbackOutput, err := d.kubectlOutput(ctx, runtimeCtx.Project.Namespace, "apply", "-f", rollbackPath)
	if err != nil {
		return RollbackReleaseResult{}, wrapK3sError("k8s_rollback_apply_failed", err, true)
	}
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		if err := d.runKubectl(ctx, runtimeCtx.Project.Namespace, "rollout", "status", fmt.Sprintf("deployment/%s", spec.Name), "--timeout=120s"); err != nil {
			return RollbackReleaseResult{}, wrapK3sError("k8s_rollback_rollout_failed", err, true)
		}
	}

	failedRevisionID := firstNonEmptyRollbackID(runtimeCtx.Rollout.RollbackFromRevisionID, runtimeCtx.Revision.RevisionID)
	restoredRevisionID := strings.TrimSpace(runtimeCtx.Rollout.RollbackToRevisionID)

	return RollbackReleaseResult{
		FailedRevisionID:   failedRevisionID,
		RestoredRevisionID: restoredRevisionID,
		RollbackPath:       rollbackPath,
		Summary: RollbackSummary{
			ProjectID:          runtimeCtx.Project.ProjectID,
			BindingID:          runtimeCtx.Binding.BindingID,
			FailedRevisionID:   failedRevisionID,
			RestoredRevisionID: restoredRevisionID,
			Summary:            rollbackApplySummary(rollbackOutput),
			RolledBackAt:       d.now(),
		},
	}, nil
}

func (d *K3sDriver) SleepService(_ context.Context, _ RuntimeContext, _ string) error {
	return &OperationError{
		Code:      "scale_to_zero_deferred",
		Message:   "sleep_service is deferred for distributed-k3s",
		Retryable: false,
	}
}

func (d *K3sDriver) WakeService(_ context.Context, _ RuntimeContext, _ string) error {
	return &OperationError{
		Code:      "scale_to_zero_deferred",
		Message:   "wake_service is deferred for distributed-k3s",
		Retryable: false,
	}
}

func (d *K3sDriver) GarbageCollectRuntime(_ context.Context, runtimeCtx RuntimeContext) (GarbageCollectRuntimeResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	d.stopRevisionLogTails(runtimeCtx.Revision.RevisionID)
	_ = os.RemoveAll(layout.Root)
	return GarbageCollectRuntimeResult{
		ProjectID:            runtimeCtx.Project.ProjectID,
		BindingID:            runtimeCtx.Binding.BindingID,
		ProtectedRevisionIDs: []string{runtimeCtx.Revision.RevisionID},
		Summary:              "k3s workspace garbage collected",
		CollectedAt:          d.now(),
	}, nil
}

func (d *K3sDriver) runKubectl(ctx context.Context, namespace string, args ...string) error {
	output, err := d.kubectlOutput(ctx, namespace, args...)
	if err != nil {
		return err
	}
	if len(output) == 0 {
		return nil
	}
	return nil
}

func (d *K3sDriver) kubectlOutput(ctx context.Context, namespace string, args ...string) ([]byte, error) {
	if strings.TrimSpace(d.kubeconfigPath) != "" {
		if _, err := os.Stat(d.kubeconfigPath); err != nil {
			return nil, &OperationError{
				Code:      "k8s_kubeconfig_missing",
				Message:   fmt.Sprintf("kubeconfig %q is not readable", d.kubeconfigPath),
				Retryable: false,
				Details: map[string]any{
					"kubeconfig_path": d.kubeconfigPath,
				},
				Err: err,
			}
		}
	}
	cmdArgs := make([]string, 0, len(args)+4)
	if strings.TrimSpace(d.kubeconfigPath) != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", d.kubeconfigPath)
	} else if server, tokenPath, caPath, ok := detectInClusterKubectlAuth(); ok {
		cmdArgs = append(cmdArgs,
			"--server", server,
			"--token", readFileTrimmed(tokenPath),
			"--certificate-authority", caPath,
		)
	}
	if ns := strings.TrimSpace(namespace); ns != "" {
		cmdArgs = append(cmdArgs, "-n", ns)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, d.kubectlBin, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, classifyKubectlError(ctx, args, output, err)
	}
	return output, nil
}

func (d *K3sDriver) RestartDeployment(ctx context.Context, namespace, serviceName string) error {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(serviceName) == "" {
		return fmt.Errorf("namespace and service name are required")
	}
	if err := d.runKubectl(ctx, namespace, "rollout", "restart", fmt.Sprintf("deployment/%s", serviceName)); err != nil {
		return err
	}
	return d.runKubectl(ctx, namespace, "rollout", "status", fmt.Sprintf("deployment/%s", serviceName), "--timeout=120s")
}

func (d *K3sDriver) LabelNode(ctx context.Context, nodeName, labelKey, labelValue string) error {
	if strings.TrimSpace(nodeName) == "" || strings.TrimSpace(labelKey) == "" || strings.TrimSpace(labelValue) == "" {
		return fmt.Errorf("node name, label key, and label value are required")
	}
	ready := false
	for i := 0; i < 60; i++ {
		output, err := d.kubectlOutput(ctx, "", "get", "node", nodeName, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		if err == nil && strings.TrimSpace(string(output)) == "True" {
			ready = true
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if !ready {
		return fmt.Errorf("node %s did not become Ready before labeling", nodeName)
	}
	_, err := d.kubectlOutput(ctx, "", "label", "node", nodeName, fmt.Sprintf("%s=%s", labelKey, labelValue), "--overwrite")
	return err
}

func detectInClusterKubectlAuth() (server, tokenPath, caPath string, ok bool) {
	host := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_HOST"))
	port := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_PORT"))
	if host == "" || port == "" {
		return "", "", "", false
	}
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	caPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	if _, err := os.Stat(tokenPath); err != nil {
		return "", "", "", false
	}
	if _, err := os.Stat(caPath); err != nil {
		return "", "", "", false
	}
	return fmt.Sprintf("https://%s:%s", host, port), tokenPath, caPath, true
}

func readFileTrimmed(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func (d *K3sDriver) ensureLiveLogTails(runtimeCtx RuntimeContext) {
	if d.logCollector == nil {
		return
	}
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		key := runtimeCtx.Revision.RevisionID + ":" + spec.Name
		d.mu.Lock()
		if _, exists := d.logTailStops[key]; exists {
			d.mu.Unlock()
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		d.logTailStops[key] = cancel
		d.mu.Unlock()
		go d.tailServiceLogs(ctx, runtimeCtx, spec)
	}
}

func (d *K3sDriver) stopRevisionLogTails(revisionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, cancel := range d.logTailStops {
		if strings.HasPrefix(key, revisionID+":") {
			cancel()
			delete(d.logTailStops, key)
		}
	}
}

func (d *K3sDriver) tailServiceLogs(ctx context.Context, runtimeCtx RuntimeContext, spec contracts.K3sServiceSpecPayload) {
	var lastSeenAt time.Time
	firstAttempt := true
	for {
		endedAt, err := d.tailServiceLogsOnce(ctx, runtimeCtx, spec, firstAttempt, lastSeenAt)
		if !endedAt.IsZero() {
			lastSeenAt = endedAt
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			d.emitK3sSystemLog(runtimeCtx, spec.Name, contracts.SeverityWarning, fmt.Sprintf("live log stream retry for %s: %s", spec.Name, err.Error()))
		} else {
			d.emitK3sSystemLog(runtimeCtx, spec.Name, contracts.SeverityInfo, fmt.Sprintf("live log stream restarted for %s after pod update", spec.Name))
		}
		firstAttempt = false
		select {
		case <-ctx.Done():
			return
		case <-time.After(k3sLogTailRetryBackoff):
		}
	}
}

func (d *K3sDriver) tailServiceLogsOnce(ctx context.Context, runtimeCtx RuntimeContext, spec contracts.K3sServiceSpecPayload, firstAttempt bool, lastSeenAt time.Time) (time.Time, error) {
	cmdArgs := make([]string, 0, 16)
	if strings.TrimSpace(d.kubeconfigPath) != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", d.kubeconfigPath)
	}
	if ns := strings.TrimSpace(runtimeCtx.Project.Namespace); ns != "" {
		cmdArgs = append(cmdArgs, "-n", ns)
	}
	cmdArgs = append(cmdArgs, "logs", "-f", fmt.Sprintf("deployment/%s", spec.Name), "--all-containers=true", "--timestamps=true")
	if firstAttempt {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--tail=%d", k3sLogTailInitialLines))
	} else {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--tail=%d", k3sLogTailResumeLines))
		if !lastSeenAt.IsZero() {
			cmdArgs = append(cmdArgs, "--since-time", lastSeenAt.Add(-k3sLogTailResumeOverlap).UTC().Format(time.RFC3339))
		}
	}

	cmd := exec.CommandContext(ctx, d.kubectlBin, cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return time.Time{}, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		if stderr.Len() > 0 {
			return time.Time{}, fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return time.Time{}, err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), k3sLogTailScannerMaxSize)
	latestSeenAt := lastSeenAt
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		timestamp, message := parseK3sTimestampedLogLine(raw)
		if !timestamp.IsZero() && timestamp.After(latestSeenAt) {
			latestSeenAt = timestamp
		}
		d.logCollector.Ingest(contracts.LogEntry{
			Timestamp: nonZeroTime(timestamp, d.now()),
			Severity:  contracts.SeverityInfo,
			Source:    "k3s-pod",
			Message:   message,
			Excerpt:   message,
			Labels: map[string]string{
				"project_id":  runtimeCtx.Project.ProjectID,
				"binding_id":  runtimeCtx.Binding.BindingID,
				"revision_id": runtimeCtx.Revision.RevisionID,
				"service":     spec.Name,
				"source_kind": "k3s",
			},
		})
	}
	if scanErr := scanner.Err(); scanErr != nil && ctx.Err() == nil {
		_ = cmd.Wait()
		return latestSeenAt, scanErr
	}
	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		if stderr.Len() > 0 {
			return latestSeenAt, fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return latestSeenAt, err
	}
	return latestSeenAt, nil
}

func parseK3sTimestampedLogLine(raw string) (time.Time, string) {
	parts := strings.SplitN(strings.TrimSpace(raw), " ", 2)
	if len(parts) != 2 {
		return time.Time{}, strings.TrimSpace(raw)
	}
	timestamp, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, strings.TrimSpace(raw)
	}
	return timestamp.UTC(), strings.TrimSpace(parts[1])
}

func nonZeroTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func (d *K3sDriver) emitK3sSystemLog(runtimeCtx RuntimeContext, serviceName string, severity contracts.Severity, message string) {
	if d == nil || d.logCollector == nil || strings.TrimSpace(message) == "" {
		return
	}
	d.logCollector.Ingest(contracts.LogEntry{
		Timestamp: d.now(),
		Severity:  severity,
		Source:    "k3s-agent",
		Message:   strings.TrimSpace(message),
		Excerpt:   strings.TrimSpace(message),
		Labels: map[string]string{
			"project_id":  runtimeCtx.Project.ProjectID,
			"binding_id":  runtimeCtx.Binding.BindingID,
			"revision_id": runtimeCtx.Revision.RevisionID,
			"service":     serviceName,
			"source_kind": "k3s",
		},
	})
}

func wrapK3sError(code string, err error, retryable bool) error {
	var opErr *OperationError
	if errors.As(err, &opErr) {
		if strings.TrimSpace(opErr.Code) == "" {
			opErr.Code = code
		}
		if opErr.Details == nil {
			opErr.Details = map[string]any{}
		}
		if _, exists := opErr.Details["phase"]; !exists && strings.TrimSpace(code) != "" {
			opErr.Details["phase"] = code
		}
		return opErr
	}
	return &OperationError{
		Code:      code,
		Message:   err.Error(),
		Retryable: retryable,
		Err:       err,
	}
}

func summarizeKubectlApplyOutput(output []byte) kubectlApplySummary {
	summary := kubectlApplySummary{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasSuffix(trimmed, " created"):
			summary.Created++
		case strings.HasSuffix(trimmed, " configured"):
			summary.Configured++
		case strings.HasSuffix(trimmed, " patched"):
			summary.Patched++
		case strings.HasSuffix(trimmed, " deleted"):
			summary.Deleted++
		case strings.HasSuffix(trimmed, " serverside-applied"):
			summary.ServerSide++
		case strings.HasSuffix(trimmed, " unchanged"):
			summary.Unchanged++
		default:
			summary.Unknown++
		}
	}
	return summary
}

func (s kubectlApplySummary) totalChanges() int {
	return s.Created + s.Configured + s.Patched + s.Deleted + s.ServerSide
}

func (s kubectlApplySummary) totalMutations() int {
	return s.totalChanges() + s.Unknown
}

func rollbackApplySummary(output []byte) string {
	summary := summarizeKubectlApplyOutput(output)
	switch {
	case summary.totalMutations() == 0 && summary.Unchanged > 0:
		return "rollback manifest re-applied and cluster was already at the stable revision"
	case summary.totalChanges() > 0:
		return fmt.Sprintf("rollback manifest applied to k3s cluster with %d changed resources", summary.totalChanges())
	default:
		return "rollback manifest applied to k3s cluster"
	}
}

func classifyKubectlError(ctx context.Context, args []string, output []byte, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return &OperationError{
			Code:      "k8s_command_timeout",
			Message:   fmt.Sprintf("kubectl %s timed out", strings.Join(args, " ")),
			Retryable: true,
			Err:       err,
		}
	}
	message := strings.TrimSpace(string(output))
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "certificate has expired"), strings.Contains(lower, "x509:"), strings.Contains(lower, "provide credentials"), strings.Contains(lower, "unauthorized"):
		return &OperationError{
			Code:      "k8s_kubeconfig_stale",
			Message:   firstNonEmptyK3s(message, err.Error()),
			Retryable: false,
			Err:       err,
		}
	case strings.Contains(lower, "forbidden"):
		return &OperationError{
			Code:      "k8s_rbac_denied",
			Message:   firstNonEmptyK3s(message, err.Error()),
			Retryable: false,
			Err:       err,
		}
	case strings.Contains(lower, "no such host"), strings.Contains(lower, "connection refused"), strings.Contains(lower, "i/o timeout"), strings.Contains(lower, "context deadline exceeded"):
		return &OperationError{
			Code:      "k8s_cluster_unreachable",
			Message:   firstNonEmptyK3s(message, err.Error()),
			Retryable: true,
			Err:       err,
		}
	default:
		return fmt.Errorf("%s: %w", firstNonEmptyK3s(message, err.Error()), err)
	}
}

func firstNonEmptyK3s(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func joinNonEmptyK3s(sep string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, sep)
}

func countHealthyServices(items []ServiceHealthResult) int {
	count := 0
	for _, item := range items {
		if item.Passed {
			count++
		}
	}
	return count
}

func firstPositiveK3s(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type deploymentPortState struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name  string `json:"name"`
					Ports []struct {
						Name          string `json:"name"`
						ContainerPort int    `json:"containerPort"`
					} `json:"ports"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type servicePortState struct {
	Spec struct {
		Ports []struct {
			Name       string `json:"name"`
			Port       int    `json:"port"`
			TargetPort any    `json:"targetPort"`
		} `json:"ports"`
	} `json:"spec"`
}

type deploymentRolloutState struct {
	Metadata struct {
		Generation int64 `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration int64 `json:"observedGeneration"`
		Replicas           int   `json:"replicas"`
		ReadyReplicas      int   `json:"readyReplicas"`
		UpdatedReplicas    int   `json:"updatedReplicas"`
		AvailableReplicas  int   `json:"availableReplicas"`
		Conditions         []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"conditions"`
	} `json:"status"`
}

type podListState struct {
	Items []podState `json:"items"`
}

type podState struct {
	Metadata struct {
		Name              string `json:"name"`
		DeletionTimestamp string `json:"deletionTimestamp"`
	} `json:"metadata"`
	Status struct {
		Phase                 string               `json:"phase"`
		InitContainerStatuses []podContainerStatus `json:"initContainerStatuses"`
		ContainerStatuses     []podContainerStatus `json:"containerStatuses"`
	} `json:"status"`
}

type podContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
	State        struct {
		Waiting *struct {
			Reason string `json:"reason"`
		} `json:"waiting"`
		Terminated *struct {
			Reason   string `json:"reason"`
			ExitCode int    `json:"exitCode"`
		} `json:"terminated"`
	} `json:"state"`
}

type ingressState struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Rules []struct {
			Host string `json:"host"`
		} `json:"rules"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

type gatewayServiceState struct {
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
	Spec struct {
		ClusterIP string `json:"clusterIP"`
	} `json:"spec"`
}

type observedContainerPort struct {
	Name string
	Port int
}

func (d *K3sDriver) collectPortApplyObservations(ctx context.Context, runtimeCtx RuntimeContext) []PortApplyObservation {
	observations := make([]PortApplyObservation, 0, len(runtimeCtx.Revision.ServiceSpecs))
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		observations = append(observations, d.collectServicePortObservation(ctx, runtimeCtx.Project.Namespace, spec))
	}
	return observations
}

func (d *K3sDriver) collectServicePortObservation(ctx context.Context, namespace string, spec contracts.K3sServiceSpecPayload) PortApplyObservation {
	observation := PortApplyObservation{
		ServiceName:         spec.Name,
		ExpectedTargetPort:  firstPositiveK3s(spec.TargetPort, spec.ServicePort, spec.HealthCheck.Port),
		ExpectedServicePort: firstPositiveK3s(spec.ServicePort, spec.TargetPort),
		Status:              "unverified",
	}

	deploymentJSON, err := d.kubectlOutput(ctx, namespace, "get", fmt.Sprintf("deployment/%s", spec.Name), "-o", "json")
	if err != nil {
		observation.Warning = fmt.Sprintf("collect deployment ports failed: %s", err.Error())
		return observation
	}
	serviceJSON, err := d.kubectlOutput(ctx, namespace, "get", fmt.Sprintf("service/%s", spec.Name), "-o", "json")
	if err != nil {
		observation.Warning = fmt.Sprintf("collect service ports failed: %s", err.Error())
		return observation
	}
	return buildPortApplyObservation(spec, deploymentJSON, serviceJSON)
}

func buildPortApplyObservation(spec contracts.K3sServiceSpecPayload, deploymentJSON, serviceJSON []byte) PortApplyObservation {
	observation := PortApplyObservation{
		ServiceName:         spec.Name,
		ExpectedTargetPort:  firstPositiveK3s(spec.TargetPort, spec.ServicePort, spec.HealthCheck.Port),
		ExpectedServicePort: firstPositiveK3s(spec.ServicePort, spec.TargetPort),
		Status:              "unverified",
	}

	var deployment deploymentPortState
	if err := json.Unmarshal(deploymentJSON, &deployment); err != nil {
		observation.Warning = fmt.Sprintf("decode deployment ports failed: %s", err.Error())
		return observation
	}
	var service servicePortState
	if err := json.Unmarshal(serviceJSON, &service); err != nil {
		observation.Warning = fmt.Sprintf("decode service ports failed: %s", err.Error())
		return observation
	}

	containerPorts := extractObservedContainerPorts(deployment)
	observation.ObservedContainerPort = firstObservedContainerPort(containerPorts)
	if len(service.Spec.Ports) > 0 {
		observation.ObservedServicePort = service.Spec.Ports[0].Port
		observation.ObservedServiceTargetPort, observation.ObservedTargetPortName = resolveObservedTargetPort(service.Spec.Ports[0].TargetPort, containerPorts)
	}

	mismatches := make([]string, 0, 3)
	if observation.ExpectedTargetPort > 0 && observation.ObservedContainerPort > 0 && observation.ExpectedTargetPort != observation.ObservedContainerPort {
		mismatches = append(mismatches, fmt.Sprintf("container_port expected=%d observed=%d", observation.ExpectedTargetPort, observation.ObservedContainerPort))
	}
	if observation.ExpectedTargetPort > 0 && observation.ObservedServiceTargetPort > 0 && observation.ExpectedTargetPort != observation.ObservedServiceTargetPort {
		mismatches = append(mismatches, fmt.Sprintf("service_target_port expected=%d observed=%d", observation.ExpectedTargetPort, observation.ObservedServiceTargetPort))
	}
	if observation.ExpectedServicePort > 0 && observation.ObservedServicePort > 0 && observation.ExpectedServicePort != observation.ObservedServicePort {
		mismatches = append(mismatches, fmt.Sprintf("service_port expected=%d observed=%d", observation.ExpectedServicePort, observation.ObservedServicePort))
	}

	switch {
	case len(mismatches) > 0:
		observation.Status = "mismatch"
		observation.Warning = strings.Join(mismatches, "; ")
	case observation.ObservedContainerPort == 0 && observation.ObservedServicePort == 0 && observation.ObservedServiceTargetPort == 0:
		observation.Status = "unverified"
		observation.Warning = "no applied ports discovered from deployment/service"
	default:
		observation.Status = "matched"
	}
	return observation
}

func extractObservedContainerPorts(state deploymentPortState) []observedContainerPort {
	out := make([]observedContainerPort, 0)
	for _, container := range state.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			if port.ContainerPort <= 0 {
				continue
			}
			out = append(out, observedContainerPort{
				Name: strings.TrimSpace(port.Name),
				Port: port.ContainerPort,
			})
		}
	}
	return out
}

func firstObservedContainerPort(items []observedContainerPort) int {
	for _, item := range items {
		if item.Port > 0 {
			return item.Port
		}
	}
	return 0
}

func resolveObservedTargetPort(raw any, containerPorts []observedContainerPort) (int, string) {
	switch typed := raw.(type) {
	case float64:
		return int(typed), ""
	case int:
		return typed, ""
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
			return 0, ""
		}
		if numeric, err := strconv.Atoi(value); err == nil {
			return numeric, ""
		}
		for _, item := range containerPorts {
			if item.Name == value {
				return item.Port, value
			}
		}
		return 0, value
	default:
		return 0, ""
	}
}

func countPortApplyMismatches(items []PortApplyObservation) int {
	count := 0
	for _, item := range items {
		if item.Status == "mismatch" {
			count++
		}
	}
	return count
}

func (d *K3sDriver) collectRolloutProgress(ctx context.Context, runtimeCtx RuntimeContext) []RolloutProgress {
	progress := make([]RolloutProgress, 0, len(runtimeCtx.Revision.ServiceSpecs))
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		deploymentJSON, err := d.kubectlOutput(ctx, runtimeCtx.Project.Namespace, "get", fmt.Sprintf("deployment/%s", spec.Name), "-o", "json")
		if err != nil {
			progress = append(progress, RolloutProgress{
				ServiceName: spec.Name,
				Status:      "unverified",
				Message:     fmt.Sprintf("collect deployment rollout status failed: %s", err.Error()),
				ObservedAt:  d.now(),
			})
			continue
		}
		progress = append(progress, buildRolloutProgress(spec.Name, deploymentJSON, d.now()))
	}
	return progress
}

func (d *K3sDriver) shouldTolerateRolloutStatusFailure(ctx context.Context, namespace, serviceName string, rolloutErr error) (bool, RolloutProgress) {
	progress := RolloutProgress{
		ServiceName: strings.TrimSpace(serviceName),
		Status:      "unverified",
		ObservedAt:  d.now(),
	}
	if rolloutErr == nil || strings.TrimSpace(serviceName) == "" {
		return false, progress
	}

	deploymentJSON, err := d.kubectlOutput(ctx, namespace, "get", fmt.Sprintf("deployment/%s", serviceName), "-o", "json")
	if err != nil {
		progress.Message = fmt.Sprintf("collect deployment rollout status failed: %s", err.Error())
		return false, progress
	}
	progress = buildRolloutProgress(serviceName, deploymentJSON, d.now())
	if progress.Status != "ready" {
		return false, progress
	}

	errText := strings.ToLower(rolloutErr.Error())
	if strings.Contains(errText, "old replicas are pending termination") {
		progress.Message = "deployment is ready; tolerating old replicas pending termination"
		return true, progress
	}
	if strings.Contains(errText, "timed out waiting for the condition") {
		progress.Message = "deployment is ready; tolerating rollout status timeout"
		return true, progress
	}
	return false, progress
}

func buildRolloutProgress(serviceName string, deploymentJSON []byte, observedAt time.Time) RolloutProgress {
	progress := RolloutProgress{
		ServiceName: strings.TrimSpace(serviceName),
		Status:      "unverified",
		ObservedAt:  observedAt,
	}
	var deployment deploymentRolloutState
	if err := json.Unmarshal(deploymentJSON, &deployment); err != nil {
		progress.Message = fmt.Sprintf("decode deployment rollout status failed: %s", err.Error())
		return progress
	}
	desiredReplicas := deployment.Spec.Replicas
	if desiredReplicas <= 0 {
		desiredReplicas = deployment.Status.Replicas
	}
	progress.DesiredReplicas = desiredReplicas
	progress.ReadyReplicas = deployment.Status.ReadyReplicas
	progress.UpdatedReplicas = deployment.Status.UpdatedReplicas
	progress.AvailableReplicas = deployment.Status.AvailableReplicas
	progress.ObservedGeneration = deployment.Status.ObservedGeneration

	switch {
	case desiredReplicas == 0:
		progress.Status = "scaled_to_zero"
		progress.Message = "deployment has zero desired replicas"
	case deployment.Status.ReadyReplicas >= desiredReplicas &&
		deployment.Status.AvailableReplicas >= desiredReplicas &&
		deployment.Status.UpdatedReplicas >= desiredReplicas:
		progress.Status = "ready"
		progress.Message = "deployment replicas are fully available"
	default:
		progress.Status = "progressing"
		progress.Message = summarizeDeploymentCondition(deployment.Status.Conditions)
	}
	return progress
}

func summarizeDeploymentCondition(conditions []struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}) string {
	for _, condition := range conditions {
		if !strings.EqualFold(strings.TrimSpace(condition.Status), "True") {
			continue
		}
		if strings.TrimSpace(condition.Message) != "" {
			return strings.TrimSpace(condition.Message)
		}
		if strings.TrimSpace(condition.Reason) != "" {
			return strings.TrimSpace(condition.Reason)
		}
	}
	return "deployment rollout is still progressing"
}

func (d *K3sDriver) collectIngressObservations(ctx context.Context, runtimeCtx RuntimeContext) []IngressObservation {
	domainByService := make(map[string]contracts.PublicDomainPayload, len(runtimeCtx.Revision.PublicDomains))
	for _, item := range runtimeCtx.Revision.PublicDomains {
		domainByService[strings.TrimSpace(item.ServiceName)] = item
	}

	gatewayAddresses := d.collectGatewayExternalAddresses(ctx)
	observations := make([]IngressObservation, 0)
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if !spec.Public || strings.TrimSpace(spec.Name) == "" {
			continue
		}
		domain := domainByService[strings.TrimSpace(spec.Name)]
		ingressJSON, err := d.kubectlOutput(ctx, runtimeCtx.Project.Namespace, "get", fmt.Sprintf("ingress/%s", spec.Name), "-o", "json")
		if err != nil {
			observations = append(observations, buildMissingIngressObservation(spec.Name, domain, err, gatewayAddresses, d.now()))
			continue
		}
		observations = append(observations, buildIngressObservation(spec.Name, domain, ingressJSON, gatewayAddresses, d.now()))
	}
	return observations
}

func buildMissingIngressObservation(serviceName string, domain contracts.PublicDomainPayload, err error, gatewayAddresses []string, observedAt time.Time) IngressObservation {
	observation := newIngressObservation(serviceName, domain, observedAt)
	observation.Status = "missing"
	observation.Message = fmt.Sprintf("collect ingress failed: %s", err.Error())
	if len(gatewayAddresses) > 0 {
		observation.ExternalAddresses = gatewayAddresses
		observation.Status = "pending"
		observation.Message = "ingress is not readable yet, but gateway service has external address"
	}
	return observation
}

func buildIngressObservation(serviceName string, domain contracts.PublicDomainPayload, ingressJSON []byte, gatewayAddresses []string, observedAt time.Time) IngressObservation {
	observation := newIngressObservation(serviceName, domain, observedAt)
	var ingress ingressState
	if err := json.Unmarshal(ingressJSON, &ingress); err != nil {
		observation.Status = "unverified"
		observation.Message = fmt.Sprintf("decode ingress status failed: %s", err.Error())
		return observation
	}

	observation.IngressName = firstNonEmptyRollbackID(strings.TrimSpace(ingress.Metadata.Name), serviceName)
	for _, rule := range ingress.Spec.Rules {
		if host := strings.TrimSpace(rule.Host); host != "" {
			observation.Hosts = appendUniqueString(observation.Hosts, host)
			observation.URLs = appendUniqueString(observation.URLs, "http://"+host)
		}
	}
	for _, item := range ingress.Status.LoadBalancer.Ingress {
		if address := strings.TrimSpace(firstNonEmptyRollbackID(item.IP, item.Hostname)); address != "" {
			observation.ExternalAddresses = appendUniqueString(observation.ExternalAddresses, address)
		}
	}
	for _, address := range gatewayAddresses {
		observation.ExternalAddresses = appendUniqueString(observation.ExternalAddresses, address)
	}

	switch {
	case len(observation.ExternalAddresses) > 0:
		observation.Ready = true
		observation.Status = "ready"
		observation.Message = "ingress host rules resolved through Traefik gateway"
	case len(observation.Hosts) > 0:
		observation.Status = "pending_address"
		observation.Message = "ingress rules exist but external address is not published yet"
	default:
		observation.Status = "pending_rules"
		observation.Message = "ingress exists but host rules are empty"
	}
	return observation
}

func newIngressObservation(serviceName string, domain contracts.PublicDomainPayload, observedAt time.Time) IngressObservation {
	observation := IngressObservation{
		ServiceName: strings.TrimSpace(serviceName),
		IngressName: strings.TrimSpace(serviceName),
		ObservedAt:  observedAt,
	}
	for _, host := range []string{strings.TrimSpace(domain.FallbackHost), strings.TrimSpace(domain.PrimaryHost)} {
		if host == "" {
			continue
		}
		observation.Hosts = appendUniqueString(observation.Hosts, host)
	}
	for _, url := range []string{strings.TrimSpace(domain.FallbackURL), strings.TrimSpace(domain.PrimaryURL)} {
		if url == "" {
			continue
		}
		observation.URLs = appendUniqueString(observation.URLs, url)
	}
	return observation
}

func (d *K3sDriver) collectGatewayExternalAddresses(ctx context.Context) []string {
	output, err := d.kubectlOutput(ctx, "kube-system", "get", "service/traefik", "-o", "json")
	if err != nil {
		return nil
	}
	var service gatewayServiceState
	if err := json.Unmarshal(output, &service); err != nil {
		return nil
	}
	addresses := make([]string, 0, len(service.Status.LoadBalancer.Ingress)+1)
	for _, item := range service.Status.LoadBalancer.Ingress {
		if address := strings.TrimSpace(firstNonEmptyRollbackID(item.IP, item.Hostname)); address != "" {
			addresses = appendUniqueString(addresses, address)
		}
	}
	if clusterIP := strings.TrimSpace(service.Spec.ClusterIP); clusterIP != "" && clusterIP != "None" {
		addresses = appendUniqueString(addresses, clusterIP)
	}
	return addresses
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func firstNonEmptyRollbackID(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
