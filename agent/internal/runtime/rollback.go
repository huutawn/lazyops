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

func (d *FilesystemDriver) RollbackRelease(_ context.Context, runtimeCtx RuntimeContext) (RollbackReleaseResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
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

	failedRevisionID := currentFailedRevision(runtimeCtx, traffic)
	restoredRevisionID := rollbackTargetRevision(runtimeCtx, traffic, failedRevisionID)
	if restoredRevisionID == "" {
		return RollbackReleaseResult{}, &OperationError{
			Code:      "rollback_previous_stable_missing",
			Message:   fmt.Sprintf("cannot roll back revision %q because no previous stable revision is recorded", failedRevisionID),
			Retryable: false,
		}
	}

	if _, err := os.Stat(revisionRoot(d.root, runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, restoredRevisionID)); err != nil {
		return RollbackReleaseResult{}, &OperationError{
			Code:      "rollback_stable_workspace_missing",
			Message:   fmt.Sprintf("stable revision %q is missing from local runtime state", restoredRevisionID),
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
		RevisionID: failedRevisionID,
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_promoted_revision_unhealthy",
		Summary:    rollbackSummaryText(failedRevisionID, restoredRevisionID),
		OccurredAt: now,
		Details: map[string]any{
			"binding_id":             runtimeCtx.Binding.BindingID,
			"policy_action":          HealthGatePolicyRollbackRelease,
			"failed_revision_id":     failedRevisionID,
			"restored_revision_id":   restoredRevisionID,
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
			RevisionID: failedRevisionID,
			OccurredAt: now,
			Summary:    incident.Summary,
			Details: map[string]any{
				"policy_action":        HealthGatePolicyRollbackRelease,
				"restored_revision_id": restoredRevisionID,
				"severity":             incident.Severity,
				"kind":                 incident.Kind,
			},
		},
		{
			Type:       "deployment.rolled_back",
			RevisionID: restoredRevisionID,
			OccurredAt: now,
			Summary:    fmt.Sprintf("traffic returned to stable revision %s after rollback", restoredRevisionID),
			Details: map[string]any{
				"failed_revision_id": failedRevisionID,
				"zero_downtime":      true,
			},
		},
	}

	drainPlan := RollbackDrainPlan{
		FailedRevisionID:   failedRevisionID,
		RestoredRevisionID: restoredRevisionID,
		Status:             "draining",
		ZeroDowntime:       true,
		CleanupPolicy:      "retain_failed_revision_until_runtime_gc_confirms_it_is_unreferenced",
		StartedAt:          now,
	}

	updatedTraffic := TrafficShiftRecord{
		ActiveRevisionID:   restoredRevisionID,
		PreviousRevisionID: failedRevisionID,
		StableRevisionID:   restoredRevisionID,
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
		FailedRevisionID:   failedRevisionID,
		RestoredRevisionID: restoredRevisionID,
		ZeroDowntime:       true,
		GatewayVersion:     gatewayVersion,
		SidecarVersion:     sidecarVersion,
		PublicURLs:         append([]string(nil), publicURLs...),
		Incident:           incident,
		Events:             events,
		Summary:            rollbackSummaryText(failedRevisionID, restoredRevisionID),
		RolledBackAt:       now,
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
		FailedRevisionID:   failedRevisionID,
		RestoredRevisionID: restoredRevisionID,
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

	if d.processManager != nil {
		for _, svc := range runtimeCtx.Services {
			_ = d.processManager.StopProcess(svc.Name)
		}

		stableRoot := revisionRoot(d.root, runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, restoredRevisionID)
		for _, svc := range runtimeCtx.Services {
			configPath := filepath.Join(stableRoot, "services", svc.Name, "runtime.json")
			if _, err := os.Stat(configPath); err == nil {
				if _, err := d.processManager.RestartProcess(context.Background(), svc.Name, configPath); err != nil && d.logger != nil {
					d.logger.Warn("rollback process restart failed",
						"service", svc.Name,
						"error", err.Error(),
					)
				}
			}
		}
	}

	if err := annotateRolledBackCandidate(layout, incident, summary, len(runtimeCtx.Services), now); err != nil && d.logger != nil {
		d.logger.Warn("rollback candidate audit update failed",
			"revision_id", failedRevisionID,
			"error", err,
		)
	}

	if d.logger != nil {
		d.logger.Warn("rolled back unhealthy revision",
			"failed_revision_id", failedRevisionID,
			"restored_revision_id", restoredRevisionID,
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

func currentFailedRevision(runtimeCtx RuntimeContext, traffic TrafficShiftRecord) string {
	return firstNonEmpty(
		traffic.ActiveRevisionID,
		runtimeCtx.Rollout.CurrentRevisionID,
		runtimeCtx.Revision.RevisionID,
	)
}

func rollbackTargetRevision(runtimeCtx RuntimeContext, traffic TrafficShiftRecord, failedRevisionID string) string {
	target := firstNonEmpty(runtimeCtx.Rollout.PreviousStableRevisionID, traffic.PreviousRevisionID)
	if target == failedRevisionID {
		return ""
	}
	return target
}

func revisionRoot(root, projectID, bindingID, revisionID string) string {
	return filepath.Join(root, "projects", projectID, "bindings", bindingID, "revisions", revisionID)
}

func rollbackSummaryText(failedRevisionID, restoredRevisionID string) string {
	return fmt.Sprintf("promoted revision %s became unhealthy; traffic returned to stable revision %s", failedRevisionID, restoredRevisionID)
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
