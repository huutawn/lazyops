package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToProjectDomainResponse(record service.ProjectDomainRecord) responsedto.ProjectDomainResponse {
	return responsedto.ProjectDomainResponse{
		ID:                 record.ID,
		ProjectID:          record.ProjectID,
		Hostname:           record.Hostname,
		Label:              record.Label,
		Kind:               record.Kind,
		Status:             record.Status,
		StatusReason:       record.StatusReason,
		CloudflareRecordID: record.CloudflareRecordID,
		TargetKind:         record.TargetKind,
		TargetID:           record.TargetID,
		LastSyncedIP:       record.LastSyncedIP,
		PublicURL:          record.PublicURL,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func ToAllocateProjectDomainCommand(userID, role, projectID string, req requestdto.AllocateProjectDomainRequest) service.AllocateProjectDomainCommand {
	return service.AllocateProjectDomainCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		Regenerate:      req.Regenerate,
	}
}

func ToRenameProjectDomainCommand(userID, role, projectID string, req requestdto.RenameProjectDomainRequest) service.RenameProjectDomainCommand {
	return service.RenameProjectDomainCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		Label:           req.Label,
	}
}
