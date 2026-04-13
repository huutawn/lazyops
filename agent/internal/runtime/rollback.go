package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
)

type rollbackPaths struct {
	liveRoot     string
	trafficPath  string
	historyPath  string
	summaryPath  string
	incidentPath string
	eventsPath   string
	drainPath    string
	rollbackPath string
}

type rollbackScenario string

const (
	rollbackScenarioCandidateFailed   rollbackScenario = "candidate_failed_before_promotion"
	rollbackScenarioPromotedUnhealthy rollbackScenario = "promoted_revision_unhealthy"
)

type rollbackPlan struct {
	FailedRevisionID   string
	RestoredRevisionID string
	Scenario           rollbackScenario
}

func (d *FilesystemDriver) RollbackRelease(_ context.Context, runtimeCtx RuntimeContext) (RollbackReleaseResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	runtimeCtx = d.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)
	paths := rollbackFilePaths(d.root, layout, runtimeCtx)

	traffic, err := loadTrafficShiftRecord(paths.trafficPath)
	if err != nil {
		return RollbackReleaseResult{}, &OperationError{
			Code:      "rollback_traffic_state_missing",
			Message:   "live traffic state is required before rollback can proceed",
			Retryable: true,
			Err:       err,
		}
	}

	gatewayVersion, publicURLs, err := rollbackGatewayState(d.root, runtimeCtx)
	if err != nil {
		return RollbackReleaseResult{}, err
	}
	sidecarVersion, err := rollbackSidecarState(d.root, runtimeCtx)
	if err != nil {
		return RollbackReleaseResult{}, err
	}

	plan := selectRollbackPlan(runtimeCtx, traffic)
	if plan.RestoredRevisionID == "" {
		return RollbackReleaseResult{}, &OperationError{
			Code:      "rollback_previous_stable_missing",
			Message:   fmt.Sprintf("cannot roll back revision %q because no restorable stable revision is recorded", plan.FailedRevisionID),
			Retryable: false,
		}
	}

	stableRoot := revisionRoot(d.root, runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, plan.RestoredRevisionID)
	if _, err := os.Stat(stableRoot); err != nil {
		return RollbackReleaseResult{}, &OperationError{
			Code:      "rollback_stable_workspace_missing",
			Message:   fmt.Sprintf("stable revision %q is missing from local runtime state", plan.RestoredRevisionID),
			Retryable: false,
			Err:       err,
		}
	}
	stableRuntimeCtx, err := loadRuntimeContextFromWorkspaceManifestPath(filepath.Join(stableRoot, "workspace.json"))
	if err != nil {
		return RollbackReleaseResult{}, &OperationError{
			Code:      "rollback_stable_workspace_manifest_missing",
			Message:   fmt.Sprintf("stable revision %q is missing a workspace manifest", plan.RestoredRevisionID),
			Retryable: false,
			Err:       err,
		}
	}

	now := d.now()
	for _, dir := range []string{paths.liveRoot, filepath.Dir(paths.summaryPath), filepath.Dir(paths.eventsPath), filepath.Dir(paths.drainPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return RollbackReleaseResult{}, err
		}
	}

	incident := &contracts.IncidentPayload{
		ProjectID:  runtimeCtx.Project.ProjectID,
		RevisionID: plan.FailedRevisionID,
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_promoted_revision_unhealthy",
		Summary:    rollbackSummaryText(plan),
		OccurredAt: now,
		Details: map[string]any{
			"binding_id":             runtimeCtx.Binding.BindingID,
			"policy_action":          HealthGatePolicyRollbackRelease,
			"failed_revision_id":     plan.FailedRevisionID,
			"restored_revision_id":   plan.RestoredRevisionID,
			"rollback_scenario":      string(plan.Scenario),
			"previous_stable_source": firstNonEmpty(runtimeCtx.Rollout.PreviousStableRevisionID, traffic.PreviousRevisionID),
		},
	}
	if gatewayVersion != "" {
		incident.Details["gateway_version"] = gatewayVersion
	}
	if sidecarVersion != "" {
		incident.Details["sidecar_version"] = sidecarVersion
	}
	if len(publicURLs) > 0 {
		incident.Details["public_urls"] = append([]string(nil), publicURLs...)
	}

	events := []DeploymentEvent{
		{
			Type:       "deployment.unhealthy",
			RevisionID: plan.FailedRevisionID,
			OccurredAt: now,
			Summary:    incident.Summary,
			Details: map[string]any{
				"policy_action":        HealthGatePolicyRollbackRelease,
				"restored_revision_id": plan.RestoredRevisionID,
				"rollback_scenario":    string(plan.Scenario),
				"severity":             incident.Severity,
				"kind":                 incident.Kind,
			},
		},
		{
			Type:       "deployment.rolled_back",
			RevisionID: plan.RestoredRevisionID,
			OccurredAt: now,
			Summary:    fmt.Sprintf("traffic returned to stable revision %s after rollback", plan.RestoredRevisionID),
			Details: map[string]any{
				"failed_revision_id": plan.FailedRevisionID,
				"rollback_scenario":  string(plan.Scenario),
				"zero_downtime":      true,
			},
		},
	}

	drainPlan := RollbackDrainPlan{
		FailedRevisionID:   plan.FailedRevisionID,
		RestoredRevisionID: plan.RestoredRevisionID,
		Status:             "draining",
		ZeroDowntime:       true,
		CleanupPolicy:      "retain_failed_revision_until_runtime_gc_confirms_it_is_unreferenced",
		StartedAt:          now,
	}

	updatedTraffic := TrafficShiftRecord{
		ActiveRevisionID:   plan.RestoredRevisionID,
		PreviousRevisionID: plan.FailedRevisionID,
		StableRevisionID:   plan.RestoredRevisionID,
		GatewayVersion:     gatewayVersion,
		SidecarVersion:     sidecarVersion,
		ZeroDowntime:       true,
		RollbackReady:      false,
		ShiftedAt:          now,
		DrainPlanPath:      paths.drainPath,
	}

	summary := RollbackSummary{
		ProjectID:          runtimeCtx.Project.ProjectID,
		BindingID:          runtimeCtx.Binding.BindingID,
		FailedRevisionID:   plan.FailedRevisionID,
		RestoredRevisionID: plan.RestoredRevisionID,
		ZeroDowntime:       true,
		GatewayVersion:     gatewayVersion,
		SidecarVersion:     sidecarVersion,
		PublicURLs:         append([]string(nil), publicURLs...),
		Incident:           incident,
		Events:             events,
		Summary:            rollbackSummaryText(plan),
		RolledBackAt:       now,
	}

	if d.processManager != nil {
		stopServiceNames := unionRollbackServiceNames(runtimeCtx.Services, stableRuntimeCtx.Services)
		for _, serviceName := range stopServiceNames {
			_ = d.processManager.StopProcess(workloadProcessKey(runtimeCtx, serviceName))
			_ = d.processManager.StopProcess(sidecarProcessKey(runtimeCtx, serviceName))
		}

		restartFailures := make(map[string]string)
		for _, svc := range stableRuntimeCtx.Services {
			if isRollbackManagedInternalService(svc.Name) {
				continue
			}

			configPath := filepath.Join(stableRoot, "services", svc.Name, "runtime.json")
			if _, err := os.Stat(configPath); err == nil {
				processName := workloadProcessKey(stableRuntimeCtx, svc.Name)
				if _, err := d.processManager.RestartProcess(context.Background(), processName, configPath); err != nil {
					restartFailures[svc.Name] = err.Error()
				}
			}

			sidecarConfigPath := filepath.Join(
				d.root,
				"projects",
				runtimeCtx.Project.ProjectID,
				"bindings",
				runtimeCtx.Binding.BindingID,
				"sidecars",
				"live",
				"services",
				svc.Name,
				"config.json",
			)
			if _, err := os.Stat(sidecarConfigPath); err == nil {
				processName := sidecarProcessKey(stableRuntimeCtx, svc.Name)
				if _, err := d.processManager.RestartProcess(context.Background(), processName, sidecarConfigPath); err != nil {
					restartFailures[svc.Name+":sidecar"] = err.Error()
				}
			}
		}

		if len(restartFailures) > 0 {
			incident.Details["restart_failures"] = restartFailures
			if d.logger != nil {
				d.logger.Warn("rollback restore reported restart failures",
					"failed_revision_id", plan.FailedRevisionID,
					"restored_revision_id", plan.RestoredRevisionID,
					"restart_failures", restartFailures,
				)
			}
		}
	}

	if err := writeJSON(paths.summaryPath, summary); err != nil {
		return RollbackReleaseResult{}, err
	}
	if err := writeJSON(paths.incidentPath, incident); err != nil {
		return RollbackReleaseResult{}, err
	}
	if err := writeJSON(paths.drainPath, drainPlan); err != nil {
		return RollbackReleaseResult{}, err
	}
	if err := writeJSON(paths.trafficPath, updatedTraffic); err != nil {
		return RollbackReleaseResult{}, err
	}
	if err := appendDeploymentEvents(paths.eventsPath, events); err != nil {
		return RollbackReleaseResult{}, err
	}
	if err := appendRollbackHistory(paths.historyPath, summary); err != nil {
		return RollbackReleaseResult{}, err
	}

	result := RollbackReleaseResult{
		FailedRevisionID:   plan.FailedRevisionID,
		RestoredRevisionID: plan.RestoredRevisionID,
		TrafficPath:        paths.trafficPath,
		EventsPath:         paths.eventsPath,
		SummaryPath:        paths.summaryPath,
		IncidentPath:       paths.incidentPath,
		DrainPlanPath:      paths.drainPath,
		RollbackPath:       paths.rollbackPath,
		Summary:            summary,
		Traffic:            updatedTraffic,
		Incident:           incident,
		DrainPlan:          drainPlan,
		Events:             events,
	}
	if err := writeJSON(paths.rollbackPath, result); err != nil {
		return RollbackReleaseResult{}, err
	}

	if err := annotateRolledBackCandidate(layout, incident, summary, len(runtimeCtx.Services), now); err != nil && d.logger != nil {
		d.logger.Warn("rollback candidate audit update failed",
			"revision_id", plan.FailedRevisionID,
			"error", err,
		)
	}

	if d.logger != nil {
		d.logger.Warn("rolled back unhealthy revision",
			"failed_revision_id", plan.FailedRevisionID,
			"restored_revision_id", plan.RestoredRevisionID,
			"rollback_scenario", plan.Scenario,
			"gateway_version", gatewayVersion,
			"sidecar_version", sidecarVersion,
		)
	}

	return result, nil
}

func rollbackFilePaths(root string, layout WorkspaceLayout, runtimeCtx RuntimeContext) rollbackPaths {
	liveRoot := filepath.Join(
		root,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
		"rollout",
		"live",
	)
	return rollbackPaths{
		liveRoot:     liveRoot,
		trafficPath:  filepath.Join(liveRoot, "traffic.json"),
		historyPath:  filepath.Join(liveRoot, "rollback-history.json"),
		summaryPath:  filepath.Join(layout.Root, "rollback-summary.json"),
		incidentPath: filepath.Join(layout.Root, "rollback-incident.json"),
		eventsPath:   filepath.Join(layout.Root, "deployment-events.json"),
		drainPath:    filepath.Join(layout.Root, "rollback-drain-plan.json"),
		rollbackPath: filepath.Join(layout.Root, "rollback.json"),
	}
}

func rollbackGatewayState(root string, runtimeCtx RuntimeContext) (string, []string, error) {
	gatewayVersion, publicURLs, err := loadPromotableGatewayState(root, runtimeCtx)
	if err == nil {
		return gatewayVersion, publicURLs, nil
	}
	return "", nil, wrapRollbackPreconditionError(err, "rollback_gateway_unavailable", "gateway must remain available during rollback")
}

func rollbackSidecarState(root string, runtimeCtx RuntimeContext) (string, error) {
	sidecarVersion, err := loadPromotableSidecarState(root, runtimeCtx)
	if err == nil {
		return sidecarVersion, nil
	}
	return "", wrapRollbackPreconditionError(err, "rollback_sidecars_unavailable", "sidecar activation must remain available during rollback")
}

func wrapRollbackPreconditionError(err error, fallbackCode, fallbackMessage string) error {
	var opErr *OperationError
	if errors.As(err, &opErr) {
		return &OperationError{
			Code:      fallbackCode,
			Message:   fallbackMessage,
			Retryable: opErr.Retryable,
			Details:   opErr.Details,
			Err:       err,
		}
	}
	return &OperationError{
		Code:      fallbackCode,
		Message:   fallbackMessage,
		Retryable: true,
		Err:       err,
	}
}

func loadTrafficShiftRecord(path string) (TrafficShiftRecord, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return TrafficShiftRecord{}, err
	}
	var record TrafficShiftRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return TrafficShiftRecord{}, err
	}
	return record, nil
}

func appendDeploymentEvents(path string, events []DeploymentEvent) error {
	current := make([]DeploymentEvent, 0)
	if payload, err := os.ReadFile(path); err == nil {
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &current); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	current = append(current, events...)
	return writeJSON(path, current)
}

func appendRollbackHistory(path string, summary RollbackSummary) error {
	history := make([]RollbackSummary, 0)
	if payload, err := os.ReadFile(path); err == nil {
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &history); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	history = append(history, summary)
	sort.Slice(history, func(i, j int) bool {
		return history[i].RolledBackAt.Before(history[j].RolledBackAt)
	})
	return writeJSON(path, history)
}

func selectRollbackPlan(runtimeCtx RuntimeContext, traffic TrafficShiftRecord) rollbackPlan {
	candidateRevisionID := strings.TrimSpace(runtimeCtx.Revision.RevisionID)
	currentStableRevisionID := firstNonEmpty(
		runtimeCtx.Rollout.CurrentRevisionID,
		runtimeCtx.Rollout.StableRevisionID,
		traffic.ActiveRevisionID,
	)
	previousStableRevisionID := firstNonEmpty(
		runtimeCtx.Rollout.PreviousStableRevisionID,
		traffic.PreviousRevisionID,
	)

	if candidateRevisionID != "" && candidateRevisionID != currentStableRevisionID && candidateRevisionID != traffic.ActiveRevisionID {
		restoredRevisionID := currentStableRevisionID
		if restoredRevisionID == candidateRevisionID {
			restoredRevisionID = ""
		}
		return rollbackPlan{
			FailedRevisionID:   candidateRevisionID,
			RestoredRevisionID: restoredRevisionID,
			Scenario:           rollbackScenarioCandidateFailed,
		}
	}

	failedRevisionID := firstNonEmpty(
		traffic.ActiveRevisionID,
		runtimeCtx.Rollout.CurrentRevisionID,
		candidateRevisionID,
	)
	restoredRevisionID := previousStableRevisionID
	if restoredRevisionID == failedRevisionID {
		restoredRevisionID = ""
	}
	return rollbackPlan{
		FailedRevisionID:   failedRevisionID,
		RestoredRevisionID: restoredRevisionID,
		Scenario:           rollbackScenarioPromotedUnhealthy,
	}
}

func unionRollbackServiceNames(candidateServices, restoredServices []ServiceRuntimeContext) []string {
	unique := make(map[string]struct{})
	for _, svc := range candidateServices {
		if isRollbackManagedInternalService(svc.Name) {
			continue
		}
		unique[svc.Name] = struct{}{}
	}
	for _, svc := range restoredServices {
		if isRollbackManagedInternalService(svc.Name) {
			continue
		}
		unique[svc.Name] = struct{}{}
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isRollbackManagedInternalService(serviceName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(serviceName)), "lazyops-internal-")
}

func revisionRoot(root, projectID, bindingID, revisionID string) string {
	return filepath.Join(root, "projects", projectID, "bindings", bindingID, "revisions", revisionID)
}

func rollbackSummaryText(plan rollbackPlan) string {
	switch plan.Scenario {
	case rollbackScenarioCandidateFailed:
		return fmt.Sprintf("candidate revision %s failed before promotion; runtime returned to stable revision %s", plan.FailedRevisionID, plan.RestoredRevisionID)
	default:
		return fmt.Sprintf("promoted revision %s became unhealthy; traffic returned to stable revision %s", plan.FailedRevisionID, plan.RestoredRevisionID)
	}
}

func annotateRolledBackCandidate(layout WorkspaceLayout, incident *contracts.IncidentPayload, summary RollbackSummary, serviceCount int, at time.Time) error {
	candidate, err := loadCandidateRecord(layout)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	switch candidate.State {
	case CandidateStatePromotable:
		if err := transitionCandidateState(&candidate, CandidateStateUnhealthy, "promoted revision became unhealthy after traffic shift", at); err != nil {
			return err
		}
		if err := transitionCandidateState(&candidate, CandidateStateFailed, "rollback returned traffic to previous stable revision", at); err != nil {
			return err
		}
	case CandidateStateHealthy:
		if err := transitionCandidateState(&candidate, CandidateStateUnhealthy, "promoted revision became unhealthy after traffic shift", at); err != nil {
			return err
		}
		if err := transitionCandidateState(&candidate, CandidateStateFailed, "rollback returned traffic to previous stable revision", at); err != nil {
			return err
		}
	case CandidateStateUnhealthy:
		if err := transitionCandidateState(&candidate, CandidateStateFailed, "rollback returned traffic to previous stable revision", at); err != nil {
			return err
		}
	case CandidateStateFailed:
	default:
		// Preserve rollback progress even if the audit candidate state is older than expected.
	}

	candidate.LatestIncident = incident
	candidate.LastIncidentKey = fmt.Sprintf("rollback|%s|%s", summary.FailedRevisionID, summary.RestoredRevisionID)
	candidate.LastIncidentAt = at
	candidate.RolloutSummary = &RolloutSummary{
		RevisionID:        summary.FailedRevisionID,
		CandidateState:    candidate.State,
		PolicyAction:      HealthGatePolicyRollbackRelease,
		Summary:           summary.Summary,
		HealthyServices:   0,
		UnhealthyServices: serviceCount,
		CheckedAt:         at,
	}
	return saveCandidateRecord(candidate)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
