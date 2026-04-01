package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type rolloutPaths struct {
	liveRoot      string
	trafficPath   string
	historyPath   string
	summaryPath   string
	eventsPath    string
	drainPath     string
	promotionPath string
}

func (d *FilesystemDriver) PromoteRelease(_ context.Context, runtimeCtx RuntimeContext) (PromoteReleaseResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	candidate, err := loadCandidateRecord(layout)
	if err != nil {
		return PromoteReleaseResult{}, &OperationError{
			Code:      "promotion_candidate_missing",
			Message:   fmt.Sprintf("candidate manifest is missing for revision %q", runtimeCtx.Revision.RevisionID),
			Retryable: true,
			Err:       err,
		}
	}
	if candidate.State != CandidateStatePromotable || candidate.HealthGate == nil || !candidate.HealthGate.Promotable {
		return PromoteReleaseResult{}, &OperationError{
			Code:      "promotion_candidate_not_ready",
			Message:   fmt.Sprintf("candidate revision %q is not promotable", runtimeCtx.Revision.RevisionID),
			Retryable: false,
			Details: map[string]any{
				"candidate_state": candidate.State,
			},
		}
	}

	gatewayVersion, publicURLs, err := loadPromotableGatewayState(d.root, runtimeCtx)
	if err != nil {
		return PromoteReleaseResult{}, err
	}
	sidecarVersion, err := loadPromotableSidecarState(d.root, runtimeCtx)
	if err != nil {
		return PromoteReleaseResult{}, err
	}

	previousStable := strings.TrimSpace(runtimeCtx.Rollout.CurrentRevisionID)
	if previousStable == "" {
		previousStable = strings.TrimSpace(runtimeCtx.Rollout.StableRevisionID)
	}
	if previousStable == runtimeCtx.Revision.RevisionID {
		previousStable = ""
	}

	now := d.now()
	paths := rolloutFilePaths(d.root, layout, runtimeCtx)
	for _, dir := range []string{paths.liveRoot, filepath.Dir(paths.summaryPath), filepath.Dir(paths.eventsPath), filepath.Dir(paths.drainPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return PromoteReleaseResult{}, err
		}
	}

	events := buildPromotionEvents(runtimeCtx, candidate, previousStable, gatewayVersion, sidecarVersion, now)
	latencySignals := collectLatencySignals(candidate.HealthGate)
	drainPlan := DrainPlan{
		PreviousRevisionID: previousStable,
		PromotedRevisionID: runtimeCtx.Revision.RevisionID,
		Status:             "not_required",
		ZeroDowntime:       true,
		CleanupPolicy:      "retain_current_stable_until_next_promotion_or_runtime_gc",
		StartedAt:          now,
	}
	rollbackReady := previousStable != ""
	if previousStable != "" {
		drainPlan.Status = "draining"
	}

	summary := PromotionSummary{
		ProjectID:                runtimeCtx.Project.ProjectID,
		BindingID:                runtimeCtx.Binding.BindingID,
		RevisionID:               runtimeCtx.Revision.RevisionID,
		PreviousStableRevisionID: previousStable,
		ZeroDowntime:             true,
		RollbackReady:            rollbackReady,
		DrainStatus:              drainPlan.Status,
		GatewayVersion:           gatewayVersion,
		SidecarVersion:           sidecarVersion,
		PublicURLs:               publicURLs,
		LatencySignals:           latencySignals,
		Events:                   events,
		Summary:                  promotionSummaryText(runtimeCtx.Revision.RevisionID, previousStable, drainPlan.Status),
		PromotedAt:               now,
	}

	traffic := TrafficShiftRecord{
		ActiveRevisionID:   runtimeCtx.Revision.RevisionID,
		PreviousRevisionID: previousStable,
		StableRevisionID:   runtimeCtx.Revision.RevisionID,
		GatewayVersion:     gatewayVersion,
		SidecarVersion:     sidecarVersion,
		ZeroDowntime:       true,
		RollbackReady:      rollbackReady,
		ShiftedAt:          now,
		DrainPlanPath:      paths.drainPath,
	}

	if err := writeJSON(paths.summaryPath, summary); err != nil {
		return PromoteReleaseResult{}, err
	}
	if err := writeJSON(paths.eventsPath, events); err != nil {
		return PromoteReleaseResult{}, err
	}
	if err := writeJSON(paths.drainPath, drainPlan); err != nil {
		return PromoteReleaseResult{}, err
	}
	if err := writeJSON(paths.trafficPath, traffic); err != nil {
		return PromoteReleaseResult{}, err
	}
	if err := appendPromotionHistory(paths.historyPath, summary); err != nil {
		return PromoteReleaseResult{}, err
	}

	candidate.RolloutSummary = &RolloutSummary{
		RevisionID:        runtimeCtx.Revision.RevisionID,
		CandidateState:    candidate.State,
		PolicyAction:      HealthGatePolicyPromoteCandidate,
		Summary:           summary.Summary,
		HealthyServices:   len(latencySignals),
		UnhealthyServices: 0,
		CheckedAt:         now,
	}
	if err := saveCandidateRecord(candidate); err != nil {
		return PromoteReleaseResult{}, err
	}

	result := PromoteReleaseResult{
		RevisionID:               runtimeCtx.Revision.RevisionID,
		PreviousStableRevisionID: previousStable,
		ZeroDowntime:             true,
		RollbackReady:            rollbackReady,
		GatewayVersion:           gatewayVersion,
		SidecarVersion:           sidecarVersion,
		TrafficPath:              paths.trafficPath,
		DrainPlanPath:            paths.drainPath,
		SummaryPath:              paths.summaryPath,
		EventsPath:               paths.eventsPath,
		Summary:                  summary,
		Traffic:                  traffic,
		DrainPlan:                drainPlan,
		Events:                   events,
	}
	if err := writeJSON(paths.promotionPath, result); err != nil {
		return PromoteReleaseResult{}, err
	}

	if d.logger != nil {
		d.logger.Info("promoted release candidate",
			"revision_id", runtimeCtx.Revision.RevisionID,
			"previous_stable_revision_id", previousStable,
			"gateway_version", gatewayVersion,
			"sidecar_version", sidecarVersion,
			"rollback_ready", rollbackReady,
		)
	}

	return result, nil
}

func rolloutFilePaths(root string, layout WorkspaceLayout, runtimeCtx RuntimeContext) rolloutPaths {
	liveRoot := filepath.Join(
		root,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
		"rollout",
		"live",
	)
	return rolloutPaths{
		liveRoot:      liveRoot,
		trafficPath:   filepath.Join(liveRoot, "traffic.json"),
		historyPath:   filepath.Join(liveRoot, "history.json"),
		summaryPath:   filepath.Join(layout.Root, "promotion-summary.json"),
		eventsPath:    filepath.Join(layout.Root, "deployment-events.json"),
		drainPath:     filepath.Join(layout.Root, "drain-plan.json"),
		promotionPath: filepath.Join(layout.Root, "promotion.json"),
	}
}

func buildPromotionEvents(runtimeCtx RuntimeContext, candidate CandidateRecord, previousStable, gatewayVersion, sidecarVersion string, now time.Time) []DeploymentEvent {
	events := make([]DeploymentEvent, 0, 2)
	if candidate.HealthGate != nil {
		events = append(events, DeploymentEvent{
			Type:       "deployment.candidate_ready",
			RevisionID: runtimeCtx.Revision.RevisionID,
			OccurredAt: candidate.HealthGate.CheckedAt,
			Summary:    candidate.HealthGate.Summary,
			Details: map[string]any{
				"candidate_state": candidate.HealthGate.CandidateState,
				"policy_action":   candidate.HealthGate.PolicyAction,
			},
		})
	}

	promoted := DeploymentEvent{
		Type:       "deployment.promoted",
		RevisionID: runtimeCtx.Revision.RevisionID,
		OccurredAt: now,
		Summary:    promotionSummaryText(runtimeCtx.Revision.RevisionID, previousStable, "draining"),
		Details: map[string]any{
			"gateway_version": gatewayVersion,
			"sidecar_version": sidecarVersion,
			"zero_downtime":   true,
		},
	}
	if previousStable != "" {
		promoted.Details["previous_stable_revision_id"] = previousStable
		promoted.Details["drain_status"] = "draining"
	} else {
		promoted.Details["drain_status"] = "not_required"
	}
	events = append(events, promoted)
	return events
}

func collectLatencySignals(snapshot *CandidateHealthSnapshot) []LatencySignal {
	if snapshot == nil {
		return nil
	}
	signals := make([]LatencySignal, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		if !service.Passed {
			continue
		}
		signals = append(signals, LatencySignal{
			ServiceName: service.ServiceName,
			Protocol:    service.Protocol,
			LatencyMS:   service.LatencyMS,
			Status:      "healthy",
		})
	}
	return signals
}

func loadPromotableGatewayState(root string, runtimeCtx RuntimeContext) (string, []string, error) {
	if len(publicServiceNames(runtimeCtx.Services)) == 0 {
		return "", nil, nil
	}
	activationPath := filepath.Join(root, "projects", runtimeCtx.Project.ProjectID, "bindings", runtimeCtx.Binding.BindingID, "gateway", "live", "active.json")
	activation, err := loadGatewayActivation(activationPath)
	if err != nil {
		return "", nil, &OperationError{
			Code:      "promotion_gateway_not_ready",
			Message:   "gateway config must be rendered before promotion",
			Retryable: true,
			Err:       err,
		}
	}
	planPayload, err := os.ReadFile(filepath.Join(root, "projects", runtimeCtx.Project.ProjectID, "bindings", runtimeCtx.Binding.BindingID, "gateway", "live", "plan.json"))
	if err != nil {
		return "", nil, err
	}
	var plan GatewayPlan
	if err := json.Unmarshal(planPayload, &plan); err != nil {
		return "", nil, err
	}
	return activation.Version, collectPublicURLs(plan), nil
}

func loadPromotableSidecarState(root string, runtimeCtx RuntimeContext) (string, error) {
	needsSidecars := false
	for _, service := range runtimeCtx.Services {
		if len(service.Dependencies) > 0 {
			needsSidecars = true
			break
		}
	}
	if !needsSidecars {
		return "", nil
	}

	activationPath := filepath.Join(root, "projects", runtimeCtx.Project.ProjectID, "bindings", runtimeCtx.Binding.BindingID, "sidecars", "live", "activation.json")
	activation, err := loadSidecarActivation(activationPath)
	if err != nil {
		return "", &OperationError{
			Code:      "promotion_sidecars_not_ready",
			Message:   "sidecar config must be rendered before promotion",
			Retryable: true,
			Err:       err,
		}
	}
	return activation.Version, nil
}

func appendPromotionHistory(path string, summary PromotionSummary) error {
	history := make([]PromotionSummary, 0)
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
	return writeJSON(path, history)
}

func promotionSummaryText(revisionID, previousStable, drainStatus string) string {
	if previousStable == "" {
		return fmt.Sprintf("revision %s promoted with zero-downtime traffic shift; no previous stable revision to drain", revisionID)
	}
	return fmt.Sprintf("revision %s promoted with zero-downtime traffic shift; previous stable revision %s is %s", revisionID, previousStable, drainStatus)
}
