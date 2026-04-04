package controller

import (
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/internal/service"
)

func resolveProjectForClaims(projects service.ProjectStore, claims *service.Claims, projectID string) (*models.Project, error) {
	projectID = strings.TrimSpace(projectID)
	if claims == nil || strings.TrimSpace(claims.UserID) == "" || projectID == "" {
		return nil, service.ErrInvalidInput
	}

	if claims.Role == service.RoleAdmin {
		project, err := projects.GetByID(projectID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, service.ErrProjectNotFound
		}
		return project, nil
	}

	project, err := projects.GetByIDForUser(claims.UserID, projectID)
	if err != nil {
		return nil, err
	}
	if project != nil {
		return project, nil
	}

	otherProject, err := projects.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if otherProject == nil {
		return nil, service.ErrProjectNotFound
	}

	return nil, service.ErrProjectAccessDenied
}
