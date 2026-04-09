package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToBootstrapAutoCommand(userID, role string, req requestdto.BootstrapAutoRequest) service.BootstrapAutoCommand {
	return service.BootstrapAutoCommand{
		RequesterUserID:      userID,
		RequesterRole:        role,
		ProjectID:            req.ProjectID,
		ProjectName:          req.ProjectName,
		DefaultBranch:        req.DefaultBranch,
		RepoFullName:         req.RepoFullName,
		GitHubInstallationID: req.GitHubInstallationID,
		GitHubRepoID:         req.GitHubRepoID,
		TrackedBranch:        req.TrackedBranch,
		InstanceID:           req.InstanceID,
		MeshNetworkID:        req.MeshNetworkID,
		ClusterID:            req.ClusterID,
		AutoModeEnabled:      req.AutoModeEnabled,
		LockedRuntimeMode:    req.LockedRuntimeMode,
	}
}

func ToBootstrapAutoAcceptedResponse(record service.BootstrapAutoAcceptedRecord) responsedto.BootstrapAutoAcceptedResponse {
	return responsedto.BootstrapAutoAcceptedResponse{
		JobID:     record.JobID,
		Status:    record.Status,
		ProjectID: record.ProjectID,
	}
}

func ToBootstrapOneClickDeployCommand(userID, role, projectID string, req requestdto.BootstrapOneClickDeployRequest) service.BootstrapOneClickDeployCommand {
	return service.BootstrapOneClickDeployCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		SourceRef:       req.SourceRef,
		CommitSHA:       req.CommitSHA,
		ArtifactRef:     req.ArtifactRef,
		ImageRef:        req.ImageRef,
		TriggerKind:     req.TriggerKind,
	}
}

func ToBootstrapOneClickDeployResponse(record service.BootstrapOneClickDeployRecord) responsedto.BootstrapOneClickDeployResponse {
	timeline := make([]responsedto.BootstrapPipelineEventResponse, 0, len(record.Timeline))
	for _, event := range record.Timeline {
		timeline = append(timeline, responsedto.BootstrapPipelineEventResponse{
			ID:        event.ID,
			State:     event.State,
			Label:     event.Label,
			Message:   event.Message,
			Timestamp: event.Timestamp,
		})
	}

	return responsedto.BootstrapOneClickDeployResponse{
		ProjectID:     record.ProjectID,
		BlueprintID:   record.BlueprintID,
		RevisionID:    record.RevisionID,
		DeploymentID:  record.DeploymentID,
		RolloutStatus: record.RolloutStatus,
		RolloutReason: record.RolloutReason,
		CorrelationID: record.CorrelationID,
		AgentID:       record.AgentID,
		Timeline:      timeline,
	}
}

func ToBootstrapStatusResponse(record service.ProjectBootstrapStatusRecord) responsedto.BootstrapStatusResponse {
	steps := make([]responsedto.BootstrapStepResponse, 0, len(record.Steps))
	for _, step := range record.Steps {
		actions := make([]responsedto.BootstrapStepActionResponse, 0, len(step.Actions))
		for _, action := range step.Actions {
			actions = append(actions, responsedto.BootstrapStepActionResponse{
				ID:       action.ID,
				Label:    action.Label,
				Kind:     action.Kind,
				Href:     action.Href,
				Method:   action.Method,
				Endpoint: action.Endpoint,
			})
		}

		steps = append(steps, responsedto.BootstrapStepResponse{
			ID:      step.ID,
			State:   step.State,
			Summary: step.Summary,
			Actions: actions,
		})
	}

	return responsedto.BootstrapStatusResponse{
		ProjectID:    record.ProjectID,
		OverallState: record.OverallState,
		Steps:        steps,
		AutoMode: responsedto.BootstrapAutoModeResponse{
			Enabled:              record.AutoMode.Enabled,
			SelectedMode:         record.AutoMode.SelectedMode,
			ModeSource:           record.AutoMode.ModeSource,
			ModeReasonCode:       record.AutoMode.ModeReasonCode,
			ModeReasonHuman:      record.AutoMode.ModeReasonHuman,
			UpshiftAllowed:       record.AutoMode.UpshiftAllowed,
			DownshiftAllowed:     record.AutoMode.DownshiftAllowed,
			DownshiftBlockReason: record.AutoMode.DownshiftBlockReason,
		},
		Inventory: responsedto.BootstrapInventoryResponse{
			HealthyInstances:    record.Inventory.HealthyInstances,
			HealthyMeshNetworks: record.Inventory.HealthyMeshNetworks,
			HealthyK3sClusters:  record.Inventory.HealthyK3sClusters,
		},
		UpdatedAt: record.UpdatedAt,
	}
}
