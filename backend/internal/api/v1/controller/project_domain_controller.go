package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type ProjectDomainController struct {
	service *service.ProjectDomainService
}

func NewProjectDomainController(service *service.ProjectDomainService) *ProjectDomainController {
	return &ProjectDomainController{service: service}
}

func (ctl *ProjectDomainController) Get(c *gin.Context) {
	claims := middleware.MustClaims(c)
	record, err := ctl.service.Get(claims.UserID, claims.Role, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectDomainNotFound):
			response.Error(c, http.StatusNotFound, "failed to load project domain", "project_domain_not_found", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load project domain", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load project domain", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load project domain", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "project domain loaded", mapper.ToProjectDomainResponse(*record))
}

func (ctl *ProjectDomainController) Allocate(c *gin.Context) {
	var req requestdto.AllocateProjectDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	record, err := ctl.service.Allocate(mapper.ToAllocateProjectDomainCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to allocate project domain", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to allocate project domain", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to allocate project domain", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to allocate project domain", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusCreated, "project domain allocated", mapper.ToProjectDomainResponse(*record))
}

func (ctl *ProjectDomainController) Rename(c *gin.Context) {
	var req requestdto.RenameProjectDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	record, err := ctl.service.Rename(mapper.ToRenameProjectDomainCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectDomainNotFound):
			response.Error(c, http.StatusNotFound, "failed to rename project domain", "project_domain_not_found", err.Error())
		case errors.Is(err, service.ErrProjectDomainLabelInvalid), errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to rename project domain", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectDomainLabelTaken):
			response.Error(c, http.StatusConflict, "failed to rename project domain", "domain_label_taken", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to rename project domain", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to rename project domain", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to rename project domain", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "project domain renamed", mapper.ToProjectDomainResponse(*record))
}
