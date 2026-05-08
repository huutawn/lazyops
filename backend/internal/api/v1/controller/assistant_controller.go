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

type AssistantController struct {
	service *service.AssistantService
}

func NewAssistantController(service *service.AssistantService) *AssistantController {
	return &AssistantController{service: service}
}

func (ctl *AssistantController) CreateSession(c *gin.Context) {
	claims := middleware.MustClaims(c)
	var req requestdto.CreateAssistantSessionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
			return
		}
	}
	record, err := ctl.service.CreateSession(mapper.ToCreateAssistantSessionCommand(claims.UserID, req))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to create assistant session", "internal_error", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, "assistant session created", mapper.ToAssistantSessionResponse(*record))
}

func (ctl *AssistantController) ListSessions(c *gin.Context) {
	claims := middleware.MustClaims(c)
	items, err := ctl.service.ListSessions(claims.UserID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to load assistant sessions", "internal_error", err.Error())
		return
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, mapper.ToAssistantSessionResponse(item))
	}
	response.JSON(c, http.StatusOK, "assistant sessions loaded", gin.H{"items": out})
}

func (ctl *AssistantController) GetSession(c *gin.Context) {
	claims := middleware.MustClaims(c)
	record, err := ctl.service.GetSession(claims.UserID, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "assistant session not found", "session_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load assistant session", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "assistant session loaded", mapper.ToAssistantConversationResponse(*record))
}

func (ctl *AssistantController) PostMessage(c *gin.Context) {
	claims := middleware.MustClaims(c)
	var req requestdto.PostAssistantMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}
	record, err := ctl.service.PostMessage(mapper.ToAssistantMessageCommand(claims.UserID, claims.Role, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "assistant message rejected", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "assistant session not found", "session_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "assistant message failed", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "assistant response ready", mapper.ToAssistantConversationResponse(*record))
}

func (ctl *AssistantController) ConfirmPlan(c *gin.Context) {
	claims := middleware.MustClaims(c)
	record, err := ctl.service.ConfirmPlan(service.ConfirmAssistantPlanCommand{UserID: claims.UserID, Role: claims.Role, PlanID: c.Param("id")})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "plan confirmation denied", "plan_confirmation_denied", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "plan not found", "plan_not_found", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "plan confirmation rejected", "invalid_plan_state", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "plan confirmation failed", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "assistant plan confirmed", mapper.ToAssistantConversationResponse(*record))
}

func (ctl *AssistantController) CancelPlan(c *gin.Context) {
	claims := middleware.MustClaims(c)
	record, err := ctl.service.CancelPlan(service.CancelAssistantPlanCommand{UserID: claims.UserID, Role: claims.Role, PlanID: c.Param("id")})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "plan cancellation denied", "plan_cancellation_denied", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "plan not found", "plan_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "plan cancellation failed", "internal_error", err.Error())
		}
		return
	}
	response.JSON(c, http.StatusOK, "assistant plan cancelled", mapper.ToAssistantConversationResponse(*record))
}
