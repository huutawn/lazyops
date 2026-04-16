package runtime

import (
	"bufio"
	"context"
	"encoding/json"
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
	if err := d.runKubectl(ctx, runtimeCtx.Project.Namespace, "apply", "--dry-run=server", "-f", manifestPath); err != nil {
		return ReconcileRevisionResult{}, wrapK3sError("k8s_dry_run_failed", err, true)
	}
	appliedSteps = append(appliedSteps, "kubectl_apply_dry_run")

	if err := d.runKubectl(ctx, runtimeCtx.Project.Namespace, "apply", "-f", manifestPath); err != nil {
		return ReconcileRevisionResult{}, wrapK3sError("k8s_apply_failed", err, true)
	}
	appliedSteps = append(appliedSteps, "kubectl_apply")

	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		if err := d.runKubectl(ctx, runtimeCtx.Project.Namespace, "rollout", "status", fmt.Sprintf("deployment/%s", spec.Name), "--timeout=120s"); err != nil {
			return ReconcileRevisionResult{}, wrapK3sError("k8s_rollout_failed", err, true)
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
	passed := true
	for _, spec := range runtimeCtx.Revision.ServiceSpecs {
		checkedAt := d.now()
		err := d.runKubectl(ctx, runtimeCtx.Project.Namespace, "rollout", "status", fmt.Sprintf("deployment/%s", spec.Name), "--timeout=60s")
		serviceResult := ServiceHealthResult{
			ServiceName: spec.Name,
			Protocol:    "http",
			Address:     fmt.Sprintf("%s.%s.svc.cluster.local:%d", spec.Name, runtimeCtx.Project.Namespace, firstPositiveK3s(spec.ServicePort, spec.TargetPort)),
			Path:        spec.HealthCheck.Path,
			Attempts:    1,
			Successes:   1,
			Failures:    0,
			Passed:      true,
			CheckedAt:   checkedAt,
			Message:     "deployment rollout is healthy",
		}
		if err != nil {
			passed = false
			serviceResult.Passed = false
			serviceResult.Successes = 0
			serviceResult.Failures = 1
			serviceResult.Message = err.Error()
		}
		results = append(results, serviceResult)
	}

	summary := fmt.Sprintf("%d/%d services passed k3s rollout health gate", countHealthyServices(results), len(results))
	policy := HealthGatePolicyPromoteCandidate
	candidateState := CandidateStatePromotable
	if !passed {
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
	if err := d.runKubectl(ctx, runtimeCtx.Project.Namespace, "apply", "-f", rollbackPath); err != nil {
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
			Summary:            "rollback manifest applied to k3s cluster",
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
	cmdArgs := make([]string, 0, len(args)+4)
	if strings.TrimSpace(d.kubeconfigPath) != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", d.kubeconfigPath)
	}
	if ns := strings.TrimSpace(namespace); ns != "" {
		cmdArgs = append(cmdArgs, "-n", ns)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, d.kubectlBin, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return output, nil
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
	cmdArgs := make([]string, 0, 10)
	if strings.TrimSpace(d.kubeconfigPath) != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", d.kubeconfigPath)
	}
	if ns := strings.TrimSpace(runtimeCtx.Project.Namespace); ns != "" {
		cmdArgs = append(cmdArgs, "-n", ns)
	}
	cmdArgs = append(cmdArgs, "logs", "-f", fmt.Sprintf("deployment/%s", spec.Name), "--all-containers=true", "--tail=20")
	cmd := exec.CommandContext(ctx, d.kubectlBin, cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		d.logCollector.Ingest(contracts.LogEntry{
			Timestamp: d.now(),
			Severity:  contracts.SeverityInfo,
			Source:    "k3s-pod",
			Message:   line,
			Excerpt:   line,
			Labels: map[string]string{
				"project_id":  runtimeCtx.Project.ProjectID,
				"binding_id":  runtimeCtx.Binding.BindingID,
				"revision_id": runtimeCtx.Revision.RevisionID,
				"service":     spec.Name,
				"source_kind": "k3s",
			},
		})
	}
	_ = cmd.Wait()
}

func wrapK3sError(code string, err error, retryable bool) error {
	return &OperationError{
		Code:      code,
		Message:   err.Error(),
		Retryable: retryable,
		Err:       err,
	}
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
	progress.DesiredReplicas = deployment.Status.Replicas
	progress.ReadyReplicas = deployment.Status.ReadyReplicas
	progress.UpdatedReplicas = deployment.Status.UpdatedReplicas
	progress.AvailableReplicas = deployment.Status.AvailableReplicas
	progress.ObservedGeneration = deployment.Status.ObservedGeneration

	switch {
	case deployment.Status.Replicas == 0:
		progress.Status = "scaled_to_zero"
		progress.Message = "deployment has zero desired replicas"
	case deployment.Status.ReadyReplicas >= deployment.Status.Replicas &&
		deployment.Status.AvailableReplicas >= deployment.Status.Replicas &&
		deployment.Status.UpdatedReplicas >= deployment.Status.Replicas:
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
